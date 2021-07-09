/*
Package soy is an implementation of Google's Closure Templates, which are
data-driven templates for generating HTML.

Compared to html/template, Closure Templates have a few advantages

 * Intuitive templating language that supports simple control flow, expressions and arithmetic.
 * The same templates may be used from Go, Java, and Javascript.
 * Internationalization is built in

and specific to this implementation:

 * High performance (> 3x faster than html/template in BenchmarkSimpleTemplate)
 * Hot reload for templates
 * Parse a directory tree of templates

Refer to the official language spec for details:

https://developers.google.com/closure/templates/

Template example

Here is Hello World

	{namespace examples.simple}

	/**
	 * Says hello to the world.*/
//	 */
/*	{template .helloWorld}
	  Hello world!
	{/template}


Here is a more customized version that addresses us by name and can use
greetings other than "Hello".

	/**
	 * Greets a person using "Hello" by default.
	 * @param name The name of the person.
	 * @param? greetingWord Optional greeting word to use instead of "Hello".*/
//	 */
/*	{template .helloName}
	  {if not $greetingWord}
	    Hello {$name}!
	  {else}
	    {$greetingWord} {$name}!
	  {/if}
	{/template}

This last example renders a greeting for each person in a list of names.

It demonstrates a [foreach] loop with an [ifempty] command. It also shows how to
call other templates and insert their output using the [call] command. Note that
the [data="all"] attribute in the call command passes all of the caller's
template data to the callee template.

	/**
	 * Greets a person and optionally a list of other people.
	 * @param name The name of the person.
	 * @param additionalNames The additional names to greet. May be an empty list.*/
//	 */
/*	{template .helloNames}
	  // Greet the person.
	  {call .helloName data="all" /}<br>
	  // Greet the additional people.
	  {foreach $additionalName in $additionalNames}
	    {call .helloName}
	      {param name: $additionalName /}
	    {/call}
	    {if not isLast($additionalName)}
	      <br>  // break after every line except the last
	    {/if}
	  {ifempty}
	    No additional people to greet.
	  {/foreach}
	{/template}

This example is from
https://developers.google.com/closure/templates/docs/helloworld_java.

Many more examples of Soy language features/commands may be seen here:
https://github.com/robfig/soy/blob/master/testdata/features.soy

Usage example

These are the high level steps:

 * Create a soy.Bundle and add templates to it (the literal template strings,
   files, or directories).
 * Compile the bundle of templates, resulting in a "Tofu" instance. It provides
   access to all your soy.
 * Render a HTML template from Tofu by providing the template name and a data
   object.

Typically in a web application you have a directory containing views for all of
your pages.  For example:

  app/views/
  app/views/account/
  app/views/feed/
  ...

This code snippet will parse a file of globals, all Soy templates within
app/views, and provide back a Tofu intance that can be used to render any
declared template.  Additionally, if "mode == dev", it will watch the Soy files
for changes and update your compiled templates in the background (or log compile
errors to soy.Logger).  Error checking is omitted.

On startup:

  tofu, _ := soy.NewBundle().
      WatchFiles(true).                     // watch Soy files, reload on changes
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

The goal is full compatibility and feature parity with the official Closure
Templates project.

The server-side templating functionality is well tested and nearly complete,
except for a few notable areas:

 * contextual autoescaping
 * strict autoescaping enforcement
 * internationalization/bidi support
 * strongly-typed parameter declarations (via the `{@param}` command)

Contributions to address these shortcomings are welcome.

The Javascript generation is early and lacks many generation options, but
it successfully passes the server-side template test suite. Note that it is
possible to run the official Soy compiler to generate your javascript templates
at build time, even if you use this package for server-side templates.

Please see the TODO file for features that have yet to be implemented.

Please open a Github Issue for any bugs / problems / comments, or if you find a
template that renders differently than with the official compiler.
*/
package soy
