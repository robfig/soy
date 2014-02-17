/*
Package soy is an implementation of Google's Closure Templates.

See the official documentation for features, syntax, data types, commands, functions, etc:

https://developers.google.com/closure/templates/

Usage example

Typically in a web application you have a directory containing views for all of
your pages.  For example:

  app/views/
  app/views/account/
  app/views/feed/
  ...

This code snippet will parse a file of globals, all soy templates within
app/views, and provide back a Tofu intance that can be used to render any
declared template.  (Error checking is skipped.)

On startup:

  tofu, _ := soy.NewBundle().
      AddGlobalsFile("views/globals.txt").  // parse a file of globals
      AddTemplateDir("views").              // load *.soy in all sub-directories
      CompileToTofu()

To render a page:

  var obj = data.Map{
    "user":    user,
    "account": account,
  }
  tofu.Template("acme.account.overview").
      Render(resp, obj)

If you prefer to prepare your data in non-soy-specific data structures ahead of
time, you can easily convert it using soy/data.New():

  tofu.Template("acme.account.overview").
      Render(resp, data.New(obj))

Advanced Usage

The soy package provides a friendly interface to its sub-packages.  Advanced
usages like automated template rewriting will be better served by using
e.g. soy/parse directly.

Project Status

This project is in alpha status.  The server-side templating functionality is
well tested and pretty complete (as compared to the reference implementation).
However, the API may still change in backwards-incompatible ways without notice.

Please see the TODO file for features that have yet to be implemented.

Please open a Github Issue for any bugs / problems / comments.

*/
package soy
