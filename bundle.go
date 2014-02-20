package soy

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.google.com/p/go.exp/fsnotify"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/parsepasses"
	"github.com/robfig/soy/template"
	"github.com/robfig/soy/tofu"
)

// Logger is used to print soy compile error messages when using the
// "WatchFiles" feature.
var Logger = log.New(os.Stderr, "[soy] ", 0)

type soyFile struct{ name, content string }

// Bundle is a collection of soy content (templates and globals).  It acts as
// input for the soy parser.
type Bundle struct {
	files     []soyFile
	globals   data.Map
	err       error
	watcher   *fsnotify.Watcher
	watchDirs map[string]bool
}

func NewBundle() *Bundle {
	return &Bundle{
		globals:   make(data.Map),
		watchDirs: make(map[string]bool),
	}
}

// WatchFiles tells soy to watch any template files added to this bundle,
// re-compile as necessary, and propagate the updates to your tofu.  It should
// be called once, before adding any files.
func (b *Bundle) WatchFiles(watch bool) *Bundle {
	if watch && b.err == nil && b.watcher == nil {
		b.watcher, b.err = fsnotify.NewWatcher()
	}
	return b
}

func (b *Bundle) WatchDir(path string) *Bundle {
	if b.err == nil && b.watcher != nil {
		if !b.watchDirs[path] {
			b.err = b.watcher.Watch(path)
			b.watchDirs[path] = true
		}
	}
	return b
}

// AddTemplateDir adds all *.soy files found within the given directory
// (including sub-directories) to the bundle.
func (b *Bundle) AddTemplateDir(root string) *Bundle {
	var err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			b.WatchDir(path)
			return nil
		}
		if !strings.HasSuffix(path, ".soy") {
			return nil
		}
		b.AddTemplateFile(path)
		return nil
	})
	if err != nil {
		b.err = err
	}

	return b
}

// AddTemplateFile adds the given soy template file text to this bundle.
func (b *Bundle) AddTemplateFile(filename string) *Bundle {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		b.err = err
	}

	path, _ := filepath.Split(filename)
	b.WatchDir(path)

	return b.AddTemplateString(filename, string(content))
}

func (b *Bundle) AddTemplateString(filename, soyfile string) *Bundle {
	b.files = append(b.files, soyFile{filename, soyfile})
	return b
}

func (b *Bundle) AddGlobalsFile(filename string) *Bundle {
	var f, err = os.Open(filename)
	if err != nil {
		b.err = err
		return b
	}
	globals, err := ParseGlobals(f)
	if err != nil {
		b.err = err
	}
	return b.AddGlobalsMap(globals)
}

func (b *Bundle) AddGlobalsMap(globals data.Map) *Bundle {
	for k, v := range globals {
		if existing, ok := b.globals[k]; ok {
			b.err = fmt.Errorf("global %q already defined as %q", k, existing)
			return b
		}
		b.globals[k] = v
	}
	return b
}

func (b *Bundle) CompileToTofu() (*tofu.Tofu, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Compile all the soy (globals are already parsed)
	var registry = template.Registry{}
	for _, soyfile := range b.files {
		var tree, err = parse.Soy(soyfile.name, soyfile.content, b.globals)
		if err != nil {
			return nil, err
		}
		if err = registry.Add(tree); err != nil {
			return nil, err
		}
	}

	// Apply the post-parse processing
	var err = parsepasses.CheckDataRefs(registry)
	if err != nil {
		return nil, err
	}

	var tofu = &tofu.Tofu{registry}
	if b.watcher != nil {
		go b.tofuUpdater(tofu)
	}
	return tofu, nil
}

func (b *Bundle) tofuUpdater(tofu *tofu.Tofu) {
	for {
		select {
		case <-b.watcher.Event:
			// Drain any queued events before rebuilding.
			for {
				select {
				case <-b.watcher.Event:
					continue
				default:
				}
				break
			}

			// Recompile all the soy.
			var bundle = NewBundle().
				AddGlobalsMap(b.globals)
			for _, soyfile := range b.files {
				bundle.AddTemplateFile(soyfile.name)
			}
			var newTofu, err = bundle.CompileToTofu()
			if err != nil {
				Logger.Println(err)
				continue
			}
			// update the existing tofu's template registry.
			tofu.Registry = newTofu.Registry

		case err := <-b.watcher.Error:
			// Nothing to do with errors
			Logger.Println(err)
		}
	}
}
