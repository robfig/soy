# soy

[![GoDoc](http://godoc.org/github.com/robfig/soy?status.png)](http://godoc.org/github.com/robfig/soy)
[![Build Status](https://github.com/robfig/soy/actions/workflows/go.yaml/badge.svg?query=branch%3Amaster)](https://github.com/robfig/soy/actions/workflows/go.yaml?query=branch%3Amaster)
[![Go Report Card](https://goreportcard.com/badge/robfig/soy)](https://goreportcard.com/report/robfig/soy)

Go implementation for Soy templates aka [Google Closure
Templates](https://github.com/google/closure-templates).  See
[godoc](http://godoc.org/github.com/robfig/soy) for more details and usage
examples.

This project requires Go 1.12 or higher due to one of the transitive
dependencies requires it as a minimum version; otherwise, Go 1.11 would
suffice for `go mod` support.

Be sure to set the env var `GO111MODULE=on` to use the `go mod` dependency
versioning when building and testing this project.
