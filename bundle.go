package soy

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/parsepasses"
	"github.com/robfig/soy/soyhtml"
	"github.com/robfig/soy/template"
	"gopkg.in/fsnotify.v0"
)

// Logger is used to print notifications and compile errors when using the
// "WatchFiles" feature.
var Logger = log.New(os.Stderr, "[soy] ", 0)

type soyFile struct{ name, content string }

// Bundle is a collection of soy content (templates and globals).  It acts as
// input for the soy compiler.
type Bundle struct {
	files     []soyFile
	globals   data.Map
	err       error
	watcher   *fsnotify.Watcher
	watchDirs map[string]bool
}

// NewBundle returns an empty bundle.
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
// If WatchFiles is on, it will be subsequently watched for updates.
func (b *Bundle) AddTemplateFile(filename string) *Bundle {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		b.err = err
	}

	path, _ := filepath.Split(filename)
	b.WatchDir(path)

	return b.AddTemplateString(filename, string(content))
}

// AddTemplateString adds the given template to the bundle. The name is only
// used for error messages - it does not need to be provided nor does it need to
// be a real filename.
func (b *Bundle) AddTemplateString(filename, soyfile string) *Bundle {
	b.files = append(b.files, soyFile{filename, soyfile})
	return b
}

// AddGlobalsFile opens and parses the given filename for Soy globals, and adds
// the resulting data map to the bundle.
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
	f.Close()
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

// Compile parses all of the soy files in this bundle, verifies a number of
// rules about data references, and returns the completed template registry.
func (b *Bundle) Compile() (*template.Registry, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Compile all the soy (globals are already parsed)
	var registry = template.Registry{}
	for _, soyfile := range b.files {
		var tree, err = parse.SoyFile(soyfile.name, soyfile.content, b.globals)
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

	if b.watcher != nil {
		go b.recompiler(&registry)
	}
	return &registry, nil
}

// CompileToTofu returns a soyhtml.Tofu object that allows you to render soy
// templates to HTML.
func (b *Bundle) CompileToTofu() (*soyhtml.Tofu, error) {
	var registry, err = b.Compile()
	// TODO: Verify all used funcs exist and have the right # args.
	return soyhtml.NewTofu(registry), err
}

func (b *Bundle) recompiler(reg *template.Registry) {
	for {
		select {
		case ev := <-b.watcher.Event:
			// If it's a rename, then fsnotify has removed the watch.
			// Add it back, after a delay.
			if ev.IsRename() || ev.IsDelete() {
				time.Sleep(10 * time.Millisecond)
				if err := b.watcher.Watch(ev.Name); err != nil {
					Logger.Println(err)
				}
			}

			// Recompile all the soy.
			var bundle = NewBundle().
				AddGlobalsMap(b.globals)
			for _, soyfile := range b.files {
				bundle.AddTemplateFile(soyfile.name)
			}
			var registry, err = bundle.Compile()
			if err != nil {
				Logger.Println(err)
				continue
			}

			// update the existing template registry.
			// (this is not goroutine-safe, but that seems ok for a development aid,
			// as long as it works in practice)
			*reg = *registry
			Logger.Printf("update successful (%v)", ev)

		case err := <-b.watcher.Error:
			// Nothing to do with errors
			Logger.Println(err)
		}
	}
}
