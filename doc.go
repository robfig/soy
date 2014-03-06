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
declared template.  Additionally, if "mode == dev", it will watch the soy files
for changes and update your compiled templates in the background (or log compile
errors to soy.Logger).  Error checking is omitted.

On startup:

  tofu, _ := soy.NewBundle().
      WatchFiles(mode == "dev").            // watch soy files, reload on changes (in dev)
      AddGlobalsFile("views/globals.txt").  // parse a file of globals
      AddTemplateDir("views").              // load *.soy in all sub-directories
      CompileToTofu()

To render a page:

  var obj = map[string]interface{}{
    "user":    user,
    "account": account,
  }
  tofu.Render(resp, "acme.account.overview", obj)

Structs may be used as the data context too, but keep in mind that they are
converted to data maps -- unlike html/template, the context is pure data, and
you can not call methods on it.

  var obj = HomepageContext{
    User:    user,
    Account: account,
  }
  tofu.Render(resp, "acme.account.overview", obj)

See soyhtml.StructOptions for knobs to control how your structs get converted to
data maps.

Project Status

The goal is to be fully compatible and at feature parity with the official
Closure Templates project.

The server-side templating functionality is well tested and pretty complete,
except for two notable areas: contextual autoescaping and
internationalization/bidi support and workflow.  Contributions welcome.

The Javascript generation is primitive and lacks support for user functions, but
it successfully passes the server-side template test suite. Note that it is
possible to run the official Soy compiler to generate your javascript templates
at build time, even if you use this package for server-side templates.

Please see the TODO file for features that have yet to be implemented.

Please open a Github Issue for any bugs / problems / comments, or if you find a
template that renders differently than with the official compiler.

*/
package soy
