/*
Package soy is an implementation of Google's Closure Templates.

See the official documentation for features, syntax, data types, commands, functions, etc:
https://developers.google.com/closure/templates/

This implementation is meant to be byte-for-byte compatible with the equivalent
template rendered by the Java library.

Usage example: Web application

Typically in a web application you have a directory containing views for all of
your pages.  For example:

  app/views/
  app/views/account/
  app/views/feed/
  ...

This code snippet will parse a file of globals, all soy templates within
app/views, and provide back a Tofu intance that can be used to render any
declared template.  (Error checking is skipped.)

On startup...

  soy.AddGlobalsFile("views/globals")
  soy.AddTemplateDir("views")
  tofu, _ := soy.CompileToTofu()

To render a page..

  ...
  var obj = data.Map{
    "user":    user,
    "account": account,
  }
  tofu.Template("acme.account.overview").
      Render(resp, obj)

or if you don't have a data.Map handy:

  .Render(resp, data.New(obj))

*/
package soy
