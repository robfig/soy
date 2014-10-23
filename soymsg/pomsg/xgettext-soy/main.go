// xgettext-soy is a tool to extract messages from Soy templates in the PO
// (gettext) file format.
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/robfig/gettext/po"
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/parsepasses"
	"github.com/robfig/soy/template"
)

func usage() {
	fmt.Println(`xgettext-soy is a tool to extract messages from Soy templates.

Usage:

	./xgettext-soy [INPUTPATH]...

INPUTPATH elements may be files or directories. Input directories will be
recursively searched for *.soy files.

The resulting PO template file is written to STDOUT
`)
}

var registry = template.Registry{}

func main() {
	if len(os.Args) < 2 || strings.HasSuffix(os.Args[1], "help") {
		usage()
		os.Exit(1)
	}

	// Add all the sources to the registry.
	for _, src := range os.Args[1:] {
		err := filepath.Walk(src, walkSource)
		if err != nil {
			exit(err)
		}
	}
	parsepasses.ProcessMessages(registry)

	var e = extractor{&po.File{}}
	for _, t := range registry.Templates {
		e.extract(t.Node)
	}
	e.file.WriteTo(os.Stdout)
}

func walkSource(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if !strings.HasSuffix(path, ".soy") {
		return nil
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	tree, err := parse.SoyFile(path, string(content))
	if err != nil {
		return err
	}
	if err = registry.Add(tree); err != nil {
		return err
	}
	return nil
}

type extractor struct {
	file *po.File
}

func (e extractor) extract(node ast.Node) {
	switch node := node.(type) {
	case *ast.MsgNode:
		e.file.Messages = append(e.file.Messages, po.Message{
			Comment: po.Comment{
				ExtractedComments: []string{node.Desc},
				References:        []string{fmt.Sprintf("id=%d", node.ID)},
			},
			Ctxt:     node.Meaning,
			Id:       node.PlaceholderString(),
			IdPlural: "", // TODO
		})
	default:
		if parent, ok := node.(ast.ParentNode); ok {
			for _, child := range parent.Children() {
				e.extract(child)
			}
		}
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
