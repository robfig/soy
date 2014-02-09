package soy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/parsepasses"
	"github.com/robfig/soy/template"
	"github.com/robfig/soy/tofu"
)

type soyFile struct{ content, name string }

// Bundle is a collection of soy content (templates and globals).  It acts as
// input for the soy parser.
type Bundle struct {
	files   []soyFile
	globals data.Map
	err     error
}

func NewBundle() *Bundle {
	return &Bundle{globals: make(data.Map)}
}

// AddTemplateDir adds all *.soy files found within the given directory
// (including sub-directories) to the bundle.
func (b *Bundle) AddTemplateDir(root string) *Bundle {
	b.err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		var filename = info.Name()
		if !strings.HasSuffix(filename, ".soy") {
			return nil
		}
		b.AddTemplateFile(filename)
		return nil
	})
	return b
}

// AddTemplateFile adds the given soy template file text to this bundle.
func (b *Bundle) AddTemplateFile(filename string) *Bundle {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		b.err = err
	}
	return b.AddTemplateString(string(content), filename)
}

func (b *Bundle) AddTemplateString(soyfile, filename string) *Bundle {
	b.files = append(b.files, soyFile{soyfile, filename})
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

	return &tofu.Tofu{registry}, nil
}
