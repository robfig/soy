/*
Package soyweb is a simple development server that serves the given template.

Invoke it like so:

  go get github.com/robfig/soy/soyweb
  soyweb test.soy

It will attempt to execute the "soyweb.soyweb" template found in the given file.

Parameters may be provided to the template in the URL query string.

*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/robfig/soy"
	"github.com/robfig/soy/data"
)

var port = flag.Int("port", 9812, "port on which to listen")

func main() {
	fmt.Print("Listening on :", *port, "...")
	log.Fatal(http.ListenAndServe(
		fmt.Sprintf(":%d", *port),
		http.HandlerFunc(handler)))
}

func handler(res http.ResponseWriter, req *http.Request) {
	var tofu, err = soy.NewBundle().
		AddTemplateFile(os.Args[1]).
		CompileToTofu()
	if err != nil {
		http.Error(res, err.Error(), 500)
		return
	}

	var tmpl = tofu.Template("soyweb.soyweb")
	if tmpl == nil {
		http.Error(res, "Template soyweb.soyweb not found", 500)
		return
	}

	var m = make(data.Map)
	for k, v := range req.URL.Query() {
		m[k] = data.String(v[0])
	}

	var buf bytes.Buffer
	err = tmpl.Render(&buf, m)
	if err != nil {
		http.Error(res, err.Error(), 500)
		return
	}

	io.Copy(res, &buf)
}
