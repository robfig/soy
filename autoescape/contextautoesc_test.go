package autoescape

import (
	"strings"
	"testing"

	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/template"
)

// This file contains test cases from
// tests/com/google/template/soy/parsepasses/contextautoesc/ContextualAutoescaperTest.java

func TestTrivialTemplate(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo}\n",
			"Hello, World!\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo}\n",
			"Hello, World!\n",
			"{/template}"))
}

func TestPrintInText(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"Hello, {$world |escapeHtml}!\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"Hello, {$world}!\n",
			"{/template}"))
}

func TestPrintInTextAndLink(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"Hello,",
			"<a href='worlds?world={$world |escapeUri}'>",
			"{$world |escapeHtml}",
			"</a>!\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"Hello,\n",
			"<a href='worlds?world={$world}'>\n",
			"{$world}\n",
			"</a>!\n",
			"{/template}\n"))
}

func TestObscureUrlAttributes(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			//"<meta http-equiv=refresh content='{$x |filterNormalizeUri |escapeHtmlAttribute}'>",
			"<a xml:base='{$x |filterNormalizeUri |escapeHtmlAttribute}' href='/foo'>link</a>",
			"<button formaction='{$x |filterNormalizeUri |escapeHtmlAttribute}'>do</button>",
			"<command icon='{$x |filterNormalizeUri |escapeHtmlAttribute}'></command>",
			"<object data='{$x |filterNormalizeUri |escapeHtmlAttribute}'></object>",
			"<video poster='{$x |filterNormalizeUri |escapeHtmlAttribute}'></video>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			// TODO(user): Re-enable content since it is often (but often not) used to convey
			// URLs in place of <link rel> once we can figure out a good way to distinguish the
			// URL use-cases from others.
			//"<meta http-equiv=refresh content='{$x}'>\n",
			"<a xml:base='{$x}' href='/foo'>link</a>\n",
			"<button formaction='{$x}'>do</button>\n",
			"<command icon='{$x}'></command>\n",
			"<object data='{$x}'></object>\n",
			"<video poster='{$x}'></video>\n",
			"{/template}\n"))
}

func TestConditional(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello,",
			"{if $x == 1}",
			"{$y |escapeHtml}",
			"{elseif $x == 2}",
			"<script>foo({$z |escapeJsValue})</script>",
			"{else}",
			"World!",
			"{/if}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello,\n",
			"{if $x == 1}\n",
			"  {$y}\n",
			"{elseif $x == 2}\n",
			"  <script>foo({$z})</script>\n",
			"{else}\n",
			"  World!\n",
			"{/if}\n",
			"{/template}"))
}

func TestConditionalEndsInDifferentContext(t *testing.T) {
	// Make sure that branches that ends in consistently different contexts transition to
	// that different context.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<a",
			"{if $url}",
			" href='{$url |filterNormalizeUri |escapeHtmlAttribute}'>",
			"{elseif $name}",
			" name='{$name |escapeHtmlAttribute}'>",
			"{else}",
			">",
			"{/if}",
			" onclick='alert({$value |escapeHtml})'\n", // Not escapeJsValue.
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<a",
			// Each of these branches independently closes the tag.
			"{if $url}",
			" href='{$url}'>",
			"{elseif $name}",
			" name='{$name}'>",
			"{else}",
			">",
			"{/if}",
			// So now make something that looks like a script attribute but which actually
			// appears in a PCDATA.  If the context merge has properly happened is is escaped as
			// PCDATA.
			" onclick='alert({$value})'\n",
			"{/template}"))
}

func TestBrokenConditional(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:7, template .bar: "+
			"{if} command branch ends in a different context than preceding branches: "+
			"{elseif $x == 2}<script>foo({$z})//</scrpit>",
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello,\n",
			"{if $x == 1}\n",
			"  {$y}\n",
			"{elseif $x == 2}\n",
			"  <script>foo({$z})//</scrpit>\n", // Not closed so ends inside JS.
			"{else}\n",
			"  World!\n",
			"{/if}\n",
			"{/template}"))
}

func TestSwitch(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello,",
			"{switch $x}",
			"{case 1}",
			"{$y |escapeHtml}",
			"{case 2}",
			"<script>foo({$z |escapeJsValue})</script>",
			"{default}",
			"World!",
			"{/switch}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello,\n",
			"{switch $x}\n",
			"  {case 1}\n",
			"    {$y}\n",
			"  {case 2}\n",
			"    <script>foo({$z})</script>\n",
			"  {default}\n",
			"    World!\n",
			"{/switch}\n",
			"{/template}"))
}

func TestBrokenSwitch(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:8, template .bar: "+
			"{switch} command case ends in a different context than preceding cases: "+
			"{case 2}<script>foo({$z})//</scrpit>",
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello,\n",
			"{switch $x}\n",
			"  {case 1}\n",
			"    {$y}\n",
			"  {case 2}\n",
			// Not closed so ends inside JS
			"    <script>foo({$z})//</scrpit>\n",
			"  {default}\n",
			"    World!\n",
			"{/switch}\n",
			"{/template}"))
}

func TestPrintInsideScript(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<script>",
			"foo({$a |escapeJsValue}); ",
			"bar(\"{$b |escapeJsString}\"); ",
			"baz('{$c |escapeJsString}'); ",
			"boo(/{$d |escapeJsRegex}/.test(s) ? 1 / {$e |escapeJsValue}",
			" : /{$f |escapeJsRegex}/);",
			"</script>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<script>\n",
			"foo({$a});\n",
			"bar(\"{$b}\");\n",
			"baz('{$c}');\n",
			"boo(/{$d}/.test(s) ? 1 / {$e} : /{$f}/);\n",
			"</script>\n",
			"{/template}"))
}

func TestPrintInsideJsCommentRejected(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template .foo: "+
			"Don't put {print} or {call} inside comments : {$x}",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>// {$x}</script>\n",
			"{/template}"))
}

func TestJsStringInsideQuotesRejected(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template .foo: "+
			"Escaping modes [ESCAPE_JS_VALUE] not compatible with"+
			" (Context JS_SQ_STRING) : {$world |escapeJsValue}",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>alert('Hello {$world |escapeJsValue}');</script>\n",
			"{/template}"))
}

func TestLiteral(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<script>",
			"{lb}$a{rb}",
			"</script>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<script>\n",
			"{literal}{$a}{/literal}\n",
			"</script>\n",
			"{/template}"))
}

func TestForLoop(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<style>",
			"{for $i in range($n)}",
			".foo{$i |filterCssValue}:before {lb}",
			"content: '{$i |escapeCssString}'",
			"{rb}",
			"{/for}",
			"</style>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<style>\n",
			"{for $i in range($n)}\n",
			"  .foo{$i}:before {lb}\n",
			"    content: '{$i}'\n",
			"  {rb}\n",
			"{/for}",
			"</style>\n",
			"{/template}"))
}

func TestBrokenForLoop(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:5, template .bar: "+
			"{for} command changes context so it cannot be reentered : "+
			"{for $i in range($n)}.foo{$i |filterCssValue}:before "+
			"{lb}content: '{$i |escapeCssString}{rb}{/for}",
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"  <style>\n",
			"    {for $i in range($n)}\n",
			"      .foo{$i |filterCssValue}:before {lb}\n",
			"        content: '{$i |escapeCssString}\n", // Missing close quote.
			"      {rb}\n",
			"    {/for}\n",
			"  </style>\n",
			"{/template}"))
}

func TestForeachLoop(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template baz autoescape=\"contextual\"}\n",
			"<ol>",
			"{foreach $x in $foo}",
			"<li>{$x |escapeHtml}</li>",
			"{/foreach}",
			"</ol>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template baz autoescape=\"contextual\"}\n",
			"  <ol>\n",
			"    {foreach $x in $foo}\n",
			"      <li>{$x}</li>\n",
			"    {/foreach}\n",
			"  </ol>\n",
			"{/template}"))
}

func TestBrokenForeachLoop(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:5, template baz: "+
			"{foreach} body changes context : "+
			"{foreach $x in $foo}<li class={$x}{/foreach}",
		join(
			"{namespace ns}\n\n",
			"{template baz autoescape=\"contextual\"}\n",
			"  <ol>\n",
			"    {foreach $x in $foo}\n",
			"      <li class={$x}\n",
			"    {/foreach}\n",
			"  </ol>\n",
			"{/template}"))
}

func TestForeachLoopWithIfempty(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template baz autoescape=\"contextual\"}\n",
			"<ol>",
			"{foreach $x in $foo}",
			"<li>{$x |escapeHtml}</li>",
			"{ifempty}",
			"<li><i>Nothing</i></li>",
			"{/foreach}",
			"</ol>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template baz autoescape=\"contextual\"}\n",
			"  <ol>\n",
			"    {foreach $x in $foo}\n",
			"      <li>{$x}</li>\n",
			"    {ifempty}\n",
			"      <li><i>Nothing</i></li>\n",
			"    {/foreach}\n",
			"  </ol>\n",
			"{/template}"))
}

func TestCall(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{call bar data=\"all\" /}\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello, {$world |escapeHtml}!\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  {call bar data=\"all\" /}\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"  Hello, {$world}!\n",
			"{/template}"))
}

func TestCallWithParams(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{call bar}{param x: $x + 1 /}{/call}\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello, {$world |escapeHtml}!\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  {call bar}{param x: $x + 1 /}{/call}\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"  Hello, {$world}!\n",
			"{/template}"))
}

func TestSameTemplateCalledInDifferentContexts(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{call bar data=\"all\" /}",
			"<script>",
			"alert('{call bar__C14 data=\"all\" /}');",
			"</script>\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"Hello, {$world |escapeHtml}!\n",
			"{/template}\n\n",
			"{template .bar__C14 autoescape=\"contextual\"}\n",
			"Hello, {$world |escapeJsString}!\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  {call bar data=\"all\" /}\n",
			"  <script>\n",
			"  alert('{call bar data=\"all\" /}');\n",
			"  </script>\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"  Hello, {$world}!\n",
			"{/template}"))
}

func TestRecursiveTemplateGuessWorks(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>",
			"x = [{call countDown__C2010 data=\"all\" /}]",
			"</script>\n",
			"{/template}\n\n",
			"{template .countDown autoescape=\"contextual\"}\n",
			"{if $x gt 0}",
			"{print --$x |escapeHtml},",
			"{call countDown /}",
			"{/if}\n",
			"{/template}\n\n",
			"{template .countDown__C2010 autoescape=\"contextual\"}\n",
			"{if $x gt 0}",
			"{print --$x |escapeJsValue},",
			"{call countDown__C2010 /}",
			"{/if}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  <script>\n",
			"    x = [{call countDown data=\"all\" /}]\n",
			"  </script>\n",
			"{/template}\n\n",
			"{template .countDown autoescape=\"contextual\"}\n",
			"  {if $x gt 0}{print --$x},{call countDown /}{/if}\n",
			"{/template}"))
}

func TestTemplateWithUnknownJsSlash(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>",
			"{if $declare}var {/if}",
			"x = {call bar__C2010 /}{\\n}",
			"y = 2",
			"  </script>\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"42",
			"{if $declare}",
			" , ",
			"{/if}\n",
			"{/template}\n\n",
			"{template .bar__C2010 autoescape=\"contextual\"}\n",
			"42",
			"{if $declare}",
			" , ",
			"{/if}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  <script>\n",
			"    {if $declare}var{sp}{/if}\n",
			"    x = {call bar /}{\\n}\n",
			// At this point we don't know whether or not a slash would start
			// a RegExp or not, but we don't see a slash so it doesn't matter.
			"    y = 2",
			"  </script>\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			// A slash following 42 would be a division operator.
			"  42\n",
			// But a slash following a comma would be a RegExp.
			"  {if $declare} , {/if}\n", //
			"{/template}"))

}

func TestTemplateUnknownJsSlashMatters(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:7, template .foo: "+
			"Slash (/) cannot follow the preceding branches since it is unclear whether the slash "+
			"is a RegExp literal or division operator."+
			"  Please add parentheses in the branches leading to `/ 2  </script>`",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  <script>\n",
			"    {if $declare}var{sp}{/if}\n",
			"    x = {call bar /}\n",
			// At this point we don't know whether or not a slash would start
			// a RegExp or not, so this constitutes an error.
			"    / 2",
			"  </script>\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			// A slash following 42 would be a division operator.
			"  42\n",
			// But a slash following a comma would be a RegExp.
			"  {if $declare} , {/if}\n", //
			"{/template}"))
}

func TestUrlContextJoining(t *testing.T) {
	// This is fine.  The ambiguity about
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<a href=\"",
			"{if $c}",
			"/foo?bar=baz",
			"{else}",
			"/boo",
			"{/if}",
			"\">\n",
			"{/template}"))
	assertRewriteFails(t,
		"In file no-path:4, template .foo: Cannot determine which part of the URL {$x} is in.",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<a href=\"",
			"{if $c}",
			"/foo?bar=baz&boo=",
			"{else}",
			"/boo/",
			"{/if}",
			"{$x}",
			"\">\n",
			"{/template}"))
}

func TestRecursiveTemplateGuessFails(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:10, template quot__C13: "+
			"{if} command without {else} changes context : "+
			"{if Math.random() lt 0.5}{call quot data=\"all\" /}{/if}",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  <script>\n",
			"    {call quot data=\"all\" /}\n",
			"  </script>\n",
			"{/template}\n\n",
			"{template quot autoescape=\"contextual\"}\n",
			"  \" {if Math.random() lt 0.5}{call quot data=\"all\" /}{/if}\n",
			"{/template}"))
}

func TestUris(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			// We use filterNormalizeUri at the beginning,
			"<a href='{$url |filterNormalizeUri |escapeHtmlAttribute}'",
			" style='background:url({$bgimage |filterNormalizeUri |escapeHtmlAttribute})'>",
			"Hi</a>",
			"<a href='#{$anchor |escapeHtmlAttribute}'",
			// escapeUri for substitutions into queries.
			" style='background:url(&apos;/pic?q={$file |escapeUri}&apos;)'>",
			"Hi",
			"</a>",
			"<style>",
			"body {lb} background-image: url(\"{$bg |filterNormalizeUri}\"); {rb}",
			// and normalizeUri without the filter in the path.
			"table {lb} border-image: url(\"borders/{$brdr |normalizeUri}\"); {rb}",
			"</style>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .bar autoescape=\"contextual\"}\n",
			"<a href='{$url}' style='background:url({$bgimage})'>Hi</a>\n",
			"<a href='#{$anchor}'\n",
			" style='background:url(&apos;/pic?q={$file}&apos;)'>Hi</a>\n",
			"<style>\n",
			"body {lb} background-image: url(\"{$bg}\"); {rb}\n",
			"table {lb} border-image: url(\"borders/{$brdr}\"); {rb}\n",
			"</style>\n",
			"{/template}"))
}

func TestCss(t *testing.T) {
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{css foo}\n",
			"{/template}"))
}

func TestXid(t *testing.T) {
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{xid foo}\n",
			"{/template}"))
}

func TestAlreadyEscaped(t *testing.T) {
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>a = \"{$FOO |escapeUri}\";</script>\n",
			"{/template}"))
}

func TestExplicitNoescapeNoop(t *testing.T) {
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>a = \"{$FOO |noAutoescape}\";</script>\n",
			"{/template}"))
}

func TestCustomDirectives(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{$x |customEscapeDirective} - {$y |customOtherDirective |escapeHtml}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  {$x |customEscapeDirective} - {$y |customOtherDirective}\n",
			"{/template}"))
}

func TestNoInterferenceWithNonContextualTemplates(t *testing.T) {
	// If a broken template calls a contextual template, object.
	assertRewriteFails(t,
		"",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"  Hello {$world}\n",
			"{/template}\n\n",
			"{template bad}\n",
			"  {if $x}\n",
			"    <!--\n",
			"  {/if}\n",
			"  {call foo/}\n",
			"{/template}"))

	// But if it doesn't, it's none of our business.
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"Hello {$world |escapeHtml}\n",
			"{/template}\n\n",
			"{template bad}\n",
			"{if $x}",
			"<!--",
			"{/if}\n",
			// No call to foo in this version.
			"{/template}"))
}

func TestExternTemplates(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>",
			"var x = {call bar /},", // Not defined in this compilation unit.
			"y = {$y |escapeJsValue};",
			"</script>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>",
			"var x = {call bar /},", // Not defined in this compilation unit.
			"y = {$y};",
			"</script>\n",
			"{/template}"))
}

func TestNonContextualCallers(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\" private=\"true\"}\n",
			"{$x |escapeHtml}\n",
			"{/template}\n\n",
			"{template .bar}\n",
			"<b>{call foo /}</b> {$y}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\" private=\"true\"}\n",
			"{$x}\n",
			"{/template}\n\n",
			"{template .bar}\n",
			"<b>{call foo /}</b> {$y}\n",
			"{/template}"))

	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\" private=\"true\"}\n",
			"{$x |escapeHtml}\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"false\"}\n",
			"<b>{call .foo /}</b> {$y}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\" private=\"true\"}\n",
			"{$x}\n",
			"{/template}\n\n",
			"{template .bar autoescape=\"false\"}\n",
			"<b>{call .foo /}</b> {$y}\n",
			"{/template}"))
}

func TestUnquotedAttributes(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<button onclick=alert({$msg |escapeJsValue |escapeHtmlAttributeNospace})>",
			"Launch</button>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<button onclick=alert({$msg})>Launch</button>\n",
			"{/template}"))
}

func TestMessagesWithEmbeddedTags(t *testing.T) {
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{msg desc=\"Say hello\"}Hello, <b>World</b>{/msg}\n",
			"{/template}"))
}

func TestNamespaces(t *testing.T) {
	// Test calls in namespaced files.
	assertContextualRewriting(t,
		join(
			"{namespace soy.examples.codelab}\n\n",
			"/** */\n",
			"{template .main autoescape=\"contextual\"}\n",
			"<title>{call soy.examples.codelab.pagenum__C81 data=\"all\" /}</title>",
			"",
			"<script>",
			"var pagenum = \"{call soy.examples.codelab.pagenum__C13 data=\"all\" /}\"; ",
			"...",
			"</script>\n",
			"{/template}\n\n",
			"/**\n",
			" * @param pageIndex 0-indexed index of the current page.\n",
			" * @param pageCount Total count of pages.  Strictly greater than pageIndex.\n",
			" */\n",
			"{template .pagenum autoescape=\"contextual\" private=\"true\"}\n",
			"{$pageIndex} of {$pageCount}\n",
			"{/template}\n\n",
			"/**\n",
			" * @param pageIndex 0-indexed index of the current page.\n",
			" * @param pageCount Total count of pages.  Strictly greater than pageIndex.\n",
			" */\n",
			"{template .pagenum__C81 autoescape=\"contextual\" private=\"true\"}\n",
			"{$pageIndex |escapeHtmlRcdata} of {$pageCount |escapeHtmlRcdata}\n",
			"{/template}\n\n",
			"/**\n",
			" * @param pageIndex 0-indexed index of the current page.\n",
			" * @param pageCount Total count of pages.  Strictly greater than pageIndex.\n",
			" */\n",
			"{template .pagenum__C13 autoescape=\"contextual\" private=\"true\"}\n",
			"{$pageIndex |escapeJsString} of {$pageCount |escapeJsString}\n",
			"{/template}"),
		join(
			"{namespace soy.examples.codelab}\n\n",
			"/** */\n",
			"{template .main autoescape=\"contextual\"}\n",
			"  <title>{call .pagenum data=\"all\" /}</title>\n",
			"  <script>\n",
			"    var pagenum = \"{call name=\".pagenum\" data=\"all\" /}\";\n",
			"    ...\n",
			"  </script>\n",
			"{/template}\n\n",
			"/**\n",
			" * @param pageIndex 0-indexed index of the current page.\n",
			" * @param pageCount Total count of pages.  Strictly greater than pageIndex.\n",
			" */\n",
			"{template .pagenum autoescape=\"contextual\" private=\"true\"}\n",
			"  {$pageIndex} of {$pageCount}\n",
			"{/template}"))
}

func TestConditionalAttributes(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<div{if $className} class=\"{$className |escapeHtmlAttribute}\"{/if}>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<div{if $className} class=\"{$className}\"{/if}>\n",
			"{/template}"))
}

func TestExtraSpacesInTag(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<div {if $className} class=\"{$className |escapeHtmlAttribute}\"{/if} id=x>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<div {if $className} class=\"{$className}\"{/if} id=x>\n",
			"{/template}"))
}

func TestOptionalAttributes(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template name=\"iconTemplate\" autoescape=\"contextual\"}\n",
			"<img class=\"{$iconClass |escapeHtmlAttribute}\"",
			"{if $iconId}",
			" id=\"{$iconId |escapeHtmlAttribute}\"",
			"{/if}",
			" src=",
			"{if $iconPath}",
			"\"{$iconPath |filterNormalizeUri |escapeHtmlAttribute}\"",
			"{else}",
			"\"images/cleardot.gif\"",
			"{/if}",
			"{if $title}",
			" title=\"{$title |escapeHtmlAttribute}\"",
			"{/if}",
			" alt=\"",
			"{if $alt || $alt == ''}",
			"{$alt |escapeHtmlAttribute}",
			"{elseif $title}",
			"{$title |escapeHtmlAttribute}",
			"{/if}\"",
			">\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template name=\"iconTemplate\" autoescape=\"contextual\"}\n",
			"<img class=\"{$iconClass}\"",
			"{if $iconId}",
			" id=\"{$iconId}\"",
			"{/if}",
			// Double quotes inside if/else.
			" src=",
			"{if $iconPath}",
			"\"{$iconPath}\"",
			"{else}",
			"\"images/cleardot.gif\"",
			"{/if}",
			"{if $title}",
			" title=\"{$title}\"",
			"{/if}",
			" alt=\"",
			"{if $alt || $alt == ''}",
			"{$alt}",
			"{elseif $title}",
			"{$title}",
			"{/if}\"",
			">\n",
			"{/template}"))
}

func TestDynamicAttrName(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<img src=\"bar\" {$baz |filterHtmlAttributes}=\"boo\">\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<img src=\"bar\" {$baz}=\"boo\">\n",
			"{/template}"))
}

func TestDynamicAttritubes(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<img src=\"bar\" {$baz |filterHtmlAttributes}>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<img src=\"bar\" {$baz}>\n",
			"{/template}"))
}

func TestDynamicElementName(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<h{$headerLevel |filterHtmlElementName}>Header"+
				"</h{$headerLevel |filterHtmlElementName}>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<h{$headerLevel}>Header</h{$headerLevel}>\n",
			"{/template}"))
}

func TestOptionalValuelessAttributes(t *testing.T) {
	assertContextualRewritingNoop(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<input {if c}checked{/if}>",
			"<input {if c}id={id |customEscapeDirective}{/if}>\n",
			"{/template}"))
}

func TestDirectivesOrderedProperly(t *testing.T) {
	// The |bidiSpanWrap directive takes HTML and produces HTML, so the |escapeHTML
	// should appear first.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{$x |escapeHtml |bidiSpanWrap}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{$x |bidiSpanWrap}\n",
			"{/template}"))

	// But if we have a |bidiSpanWrap directive in a non HTML context, then don't reorder.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>var html = {$x |bidiSpanWrap |escapeJsValue}</script>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<script>var html = {$x |bidiSpanWrap}</script>\n",
			"{/template}"))
}

// func TestDelegateTemplatesAreEscaped(t *testing.T) {
// 	assertContextualRewriting(t,
// 		join(
// 			"{delpackage dp}\n",
// 			"{namespace ns}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo autoescape=\"contextual\"}\n",
// 			"{$x |escapeHtml}\n",
// 			"{/deltemplate}"),
// 		join(
// 			"{delpackage dp}\n",
// 			"{namespace ns}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo autoescape=\"contextual\"}\n",
// 			"{$x}\n",
// 			"{/deltemplate}"))
// }

// func TestDelegateTemplateCalledInNonPcdataContexts(t *testing.T) {
// 	assertContextualRewriting(t,
// 		join(
// 			"{delpackage dp}\n",
// 			"{namespace ns}\n\n",
// 			"{template main autoescape=\"contextual\"}\n",
// 			"<script>{delcall foo__C2010 /}</script>\n",
// 			"{/template}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo autoescape=\"contextual\"}\n",
// 			"x = {$x |escapeHtml}\n",
// 			"{/deltemplate}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo__C2010 autoescape=\"contextual\"}\n",
// 			"x = {$x |escapeJsValue}\n",
// 			"{/deltemplate}"),
// 		join(
// 			"{delpackage dp}\n",
// 			"{namespace ns}\n\n",
// 			"{template main autoescape=\"contextual\"}\n",
// 			"<script>{delcall foo /}</script>\n",
// 			"{/template}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo autoescape=\"contextual\"}\n",
// 			"x = {$x}\n",
// 			"{/deltemplate}"))
// }

// func TestDelegateTemplatesReturnTypesUnioned(t *testing.T) {
// 	assertRewriteFails(t,
// 		"In file no-path-0:7, template main: "+
// 			"Slash (/) cannot follow the preceding branches since it is unclear whether the slash "+
// 			"is a RegExp literal or division operator.  "+
// 			"Please add parentheses in the branches leading to "+
// 			"`/foo/i.test(s) && alert(s);</script>`",
// 		join(
// 			"{namespace ns}\n\n",
// 			"{template main autoescape=\"contextual\"}\n",
// 			"{delcall foo}\n",
// 			"{param x: '' /}\n",
// 			"{/delcall}\n",
// 			// The / here is intended to start a regex, but if the version
// 			// from dp2 is used it won't be.
// 			"/foo/i.test(s) && alert(s);\n",
// 			"</script>\n",
// 			"{/template}"),
// 		join(
// 			"{delpackage dp1}\n",
// 			"{namespace ns}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo autoescape=\"contextual\"}\n",
// 			"<script>x = {$x};\n", // semicolon terminated
// 			"{/deltemplate}"),
// 		join(
// 			"{delpackage dp2}\n",
// 			"{namespace ns}\n\n",
// 			"/** @param x */\n",
// 			"{deltemplate .foo autoescape=\"contextual\"}\n",
// 			"<script>x = {$x}\n", // not semicolon terminated
// 			"{/deltemplate}"))
// }

func TestTypedLetBlockIsContextuallyEscaped(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{template t autoescape=\"contextual\"}\n",
			"<script> var y = '",
			// Note that the contents of the {let} block are escaped in HTML PCDATA context, even
			// though it appears in a JS string context in the template.
			"{let $l kind=\"html\"}",
			"<div>{$y |escapeHtml}</div>",
			"{/let}",
			"{$y |escapeJsString}'</script>\n",
			"{/template}"),
		join(
			"{template t autoescape=\"contextual\"}\n",
			"<script> var y = '\n",
			"{let $l kind=\"html\"}\n",
			"<div>{$y}</div>",
			"{/let}",
			"{$y}'</script>\n",
			"{/template}"))
}

func TestUntypedLetBlockIsContextuallyEscaped(t *testing.T) {
	// Test that the behavior for let blocks without kind attribute is unchanged (i.e., they are
	// contextually escaped in the context the {let} command appears in).
	assertContextualRewriting(t,
		join(
			"{template t autoescape=\"contextual\"}\n",
			"<script> var y = '",
			"{let $l}",
			"<div>{$y |escapeJsString}</div>",
			"{/let}",
			"{$y |escapeJsString}'</script>\n",
			"{/template}"),
		join(
			"{template t autoescape=\"contextual\"}\n",
			"<script> var y = '\n",
			"{let $l}\n",
			"<div>{$y}</div>",
			"{/let}",
			"{$y}'</script>\n",
			"{/template}"))
}

func TestTypedLetBlockMustEndInStartContext(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:2, template t: "+
			"A strict block of kind=\"html\" cannot end in context (Context JS REGEX). "+
			"Likely cause is an unclosed script block or attribute: "+
			"{let $l kind=\"html\"}",
		join(
			"{template t autoescape=\"contextual\"}\n",
			"{let $l kind=\"html\"}\n",
			"<script> var y ='{$y}';",
			"{/let}\n",
			"{/template}"))
}

func TestTypedLetBlockIsStrictModeAutoescaped(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:5, template t: "+
			"Autoescape-cancelling print directives like |customEscapeDirective are only allowed in "+
			"kind=\"text\" blocks. If you really want to over-escape, try using a let block: "+
			"{let $foo kind=\"text\"}{$y |customEscapeDirective}{/let}{$foo}.",
		join(
			"{namespace ns}\n\n",
			"{template .t autoescape=\"contextual\"}\n",
			"{let $l kind=\"html\"}\n",
			"<b>{$y |customEscapeDirective}</b>",
			"{/let}\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:5, template t: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"html\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			// Strict templates never allow noAutoescape.
			"{template .t autoescape=\"strict\"}\n",
			"{let $l kind=\"html\"}\n",
			"<b>{$y |noAutoescape}</b>",
			"{/let}\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:5, template t: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"js\" or SanitizedContent.",
		join(
			// Throw in a red-herring namespace, just to check things.
			"{namespace ns autoescape=\"contextual\"}\n\n",
			// Strict templates never allow noAutoescape.
			"{template .t autoescape=\"strict\"}\n",
			"{let $l kind=\"html\"}\n",
			"<script>{$y |noAutoescape}</script>",
			"{/let}\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:5, template t: "+
			"Soy strict autoescaping currently forbids calls to non-strict templates, unless the "+
			"context is kind=\"text\", since there's no guarantee the callee is safe: "+
			"{call .other data=\"all\" /}",
		join(
			"{namespace ns}\n\n",
			"{template .t autoescape=\"contextual\"}\n",
			"{let $l kind=\"html\"}\n",
			"<b>{call .other data=\"all\"/}</b>",
			"{/let}\n",
			"{/template}\n\n",
			"{template .other autoescape=\"contextual\"}\n",
			"Hello World\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:5, template t: "+
			"Soy strict autoescaping currently forbids calls to non-strict templates, unless the "+
			"context is kind=\"text\", since there's no guarantee the callee is safe: "+
			"{call .other data=\"all\" /}",
		join(
			"{namespace ns}\n\n",
			"{template .t autoescape=\"contextual\"}\n",
			"{let $l kind=\"html\"}\n",
			"<b>{call .other data=\"all\"/}</b>",
			"{/let}\n",
			"{/template}\n\n",
			"{template .other autoescape=\"contextual\"}\n",
			"Hello World\n",
			"{/template}"))

	// Non-autoescape-cancelling directives are allowed.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .t autoescape=\"contextual\"}\n",
			"{let $l kind=\"html\"}",
			"<b>{$y |customOtherDirective |escapeHtml}</b>",
			"{/let}\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .t autoescape=\"contextual\"}\n",
			"{let $l kind=\"html\"}\n",
			"<b>{$y |customOtherDirective}</b>",
			"{/let}\n",
			"{/template}"))
}

func TestTypedLetBlockNotAllowedInNonContextualTemplate(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:2, template t: "+
			"{let} node with 'kind' attribute is only permitted in contextually autoescaped "+
			"templates: {let $l kind=\"html\"}<b>{$y}</b>{/let}",
		join(
			"{template .t autoescape=\"true\"}\n",
			"{let $l kind=\"html\"}",
			"<b>{$y}</b>",
			"{/let}\n",
			"{/template}"))
}

func TestNonTypedParamMustEndInHtmlContextButWasAttribute(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:5, template caller: "+
			"Blocks should start and end in HTML context: {param foo}",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"  {call callee}\n",
			"    {param foo}<a href='{/param}\n",
			"  {/call}\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\"}\n",
			"  <b>{$foo}</b>\n",
			"{/template}\n"))
}

func TestNonTypedParamMustEndInHtmlContextButWasScript(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:5, template caller: "+
			"Blocks should start and end in HTML context: {param foo}",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"  {call callee}\n",
			"    {param foo}<script>var x={/param}\n",
			"  {/call}\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\"}\n",
			"  <b>{$foo}</b>\n",
			"{/template}\n"))
}

func TestNonTypedParamGetsContextuallyAutoescaped(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"{call callee}",
			"{param fooHtml}",
			"<a href=\"http://google.com/search?q={$query |escapeUri}\" ",
			"onclick=\"alert('{$query |escapeJsString |escapeHtmlAttribute}')\">",
			"Search for {$query |escapeHtml}",
			"</a>",
			"{/param}",
			"{/call}",
			"\n{/template}\n\n",
			"{template .callee autoescape=\"contextual\"}\n",
			"{$fooHTML |noAutoescape}",
			"\n{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"  {call callee}\n",
			"    {param fooHtml}\n",
			"      <a href=\"http://google.com/search?q={$query}\"\n",
			"         onclick=\"alert('{$query}')\">\n",
			"        Search for {$query}\n",
			"      </a>\n",
			"    {/param}\n",
			"  {/call}\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\"}\n",
			"  {$fooHTML |noAutoescape}\n",
			"{/template}"))
}

func TestTypedParamBlockIsContextuallyEscaped(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}",
			"<script> var y ='{$y |escapeJsString}';</script>",
			"{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x |escapeHtml}</b>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}{param x kind=\"html\"}",
			"<script> var y ='{$y}';</script>",
			"{/param}{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))
}

func TestTypedParamBlockMustEndInStartContext(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"A strict block of kind=\"html\" cannot end in context (Context JS REGEX). "+
			"Likely cause is an unclosed script block or attribute: "+
			"{param x kind=\"html\"}",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}{param x kind=\"html\"}<script> var y ='{$y}';{/param}{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))
}

func TestTypedParamBlockIsStrictModeAutoescaped(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"Autoescape-cancelling print directives like |customEscapeDirective are only allowed in "+
			"kind=\"text\" blocks. If you really want to over-escape, try using a let block: "+
			"{let $foo kind=\"text\"}{$y |customEscapeDirective}{/let}{$foo}.",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"strict\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |customEscapeDirective}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"strict\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// noAutoescape has a special error message.
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"html\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"strict\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |noAutoescape}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"strict\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// NOTE: This error only works for non-extern templates.
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"Soy strict autoescaping currently forbids calls to non-strict templates, unless the "+
			"context is kind=\"text\", since there's no guarantee the callee is safe: "+
			"{call subCallee data=\"all\" /}",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"strict\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}{call subCallee data=\"all\"/}{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"strict\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}\n\n",
			"{template subCallee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// Non-escape-cancelling directives are allowed.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"strict\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |customOtherDirective |escapeHtml}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"strict\" private=\"true\"}\n",
			"<b>{$x |escapeHtml}</b>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"strict\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |customOtherDirective}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"strict\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))
}

func TestTransitionalTypedParamBlock(t *testing.T) {
	// In non-strict contextual templates, param blocks employ "transitional" strict autoescaping,
	// which permits noAutoescape. This helps teams migrate the callees to strict even if not all
	// the callers can be fixed.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |noAutoescape}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x |escapeHtml}</b>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |noAutoescape}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// Other escape-cancelling directives are still not allowed.
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"Autoescape-cancelling print directives like |customEscapeDirective are only allowed in "+
			"kind=\"text\" blocks. If you really want to over-escape, try using a let block: "+
			"{let $foo kind=\"text\"}{$y |customEscapeDirective}{/let}{$foo}.",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |customEscapeDirective}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// NOTE: This error only works for non-extern templates.
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"Soy strict autoescaping currently forbids calls to non-strict templates, unless the "+
			"context is kind=\"text\", since there's no guarantee the callee is safe: "+
			"{call subCallee data=\"all\" /}",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}{call subCallee data=\"all\"/}{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}\n\n",
			"{template subCallee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// Non-escape-cancelling directives are allowed.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |customOtherDirective |escapeHtml}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x |escapeHtml}</b>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"html\"}<b>{$y |customOtherDirective}</b>{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))
}

func TestTypedParamBlockNotAllowedInNonContextualTemplate(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template caller: "+
			"{param} node with 'kind' attribute is only permitted in contextually autoescaped "+
			"templates: {param x kind=\"html\"}<b>{$y}</b>;{/param}",
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"true\"}\n",
			"<div>",
			"{call callee}{param x kind=\"html\"}<b>{$y}</b>;{/param}{/call}",
			"</div>\n",
			"{/template}\n\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))
}

func TestTypedTextParamBlock(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}",
			"{param x kind=\"text\"}",
			"Hello {$x |text} <{$y |text}, \"{$z |text}\">",
			"{/param}",
			"{/call}",
			"</div>\n",
			"{/template}\n",
			"\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x |escapeHtml}</b>\n",
			"{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .caller autoescape=\"contextual\"}\n",
			"<div>",
			"{call callee}{param x kind=\"text\"}",
			"Hello {$x} <{$y}, \"{$z}\">",
			"{/param}{/call}",
			"</div>\n",
			"{/template}\n",
			"\n",
			"{template .callee autoescape=\"contextual\" private=\"true\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))
}

func TestTypedTextLetBlock(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{let $a kind=\"text\"}",
			"Hello {$x |text} <{$y |text}, \"{$z |text}\">",
			"{/let}",
			"{$a |escapeHtml}",
			"\n{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"{let $a kind=\"text\"}",
			"Hello {$x} <{$y}, \"{$z}\">",
			"{/let}",
			"{$a}",
			"\n{/template}"))
}

// Tests for the initial, rudimentary strict contextual mode (which will be initially used only in
// param and let nodes of non-text kind).
// This basic strict mode
// - does not allow autoescape-cancelling print directives
// - does not allow any call/delcall commands
// - is enabled using autoescape="strict"

func TestStrictModeRejectsAutoescapeCancellingDirectives(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"Autoescape-cancelling print directives like |customEscapeDirective are only allowed in "+
			"kind=\"text\" blocks. If you really want to over-escape, try using a let block: "+
			"{let $foo kind=\"text\"}{$foo |customEscapeDirective}{/let}{$foo}.",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<b>{$foo|customEscapeDirective}</b>\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"html\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<b>{$foo|noAutoescape}</b>\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"uri\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<a href=\"{$foo|noAutoescape}\">Test</a>\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"attributes\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<div {$foo|noAutoescape}>Test</div>\n",
			"{/template}"))

	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with kind=\"js\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<script>{$foo|noAutoescape}</script>\n",
			"{/template}"))

	// NOTE: There's no recommended context for textarea, since it's really essentially text.
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"noAutoescape is not allowed in strict autoescaping mode. Instead, pass in a {param} "+
			"with appropriate kind=\"...\" or SanitizedContent.",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<textarea>{$foo|noAutoescape}</textarea>\n",
			"{/template}"))
}

func TestStrictModeRejectsNonStrictCalls(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"Soy strict autoescaping currently forbids calls to non-strict templates, unless the "+
			"context is kind=\"text\", since there's no guarantee the callee is safe: "+
			"{call bar data=\"all\" /}",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\" kind=\"html\"}\n",
			"<b>{call bar data=\"all\"/}\n",
			"{/template}\n\n"+
				"{template .bar autoescape=\"contextual\"}\n",
			"Hello World\n",
			"{/template}"))

	// assertRewriteFails(t,
	// 	"In file no-path-0:4, template main: "+
	// 		"Soy strict autoescaping currently forbids calls to non-strict templates, unless the "+
	// 		"context is kind=\"text\", since there's no guarantee the callee is safe: "+
	// 		"{delcall foo}",
	// 	join(
	// 		"{namespace ns}\n\n",
	// 		"{template main autoescape=\"strict\"}\n",
	// 		"{delcall foo}\n",
	// 		"{param x: '' /}\n",
	// 		"{/delcall}\n",
	// 		"{/template}"),
	// 	join(
	// 		"{delpackage dp1}\n",
	// 		"{namespace ns}\n\n",
	// 		"/** @param x */\n",
	// 		"{deltemplate .foo autoescape=\"contextual\"}\n",
	// 		"<b>{$x}</b>\n",
	// 		"{/deltemplate}"),
	// 	join(
	// 		"{delpackage dp2}\n",
	// 		"{namespace ns}\n\n",
	// 		"/** @param x */\n",
	// 		"{deltemplate .foo autoescape=\"contextual\"}\n",
	// 		"<i>{$x}</i>\n",
	// 		"{/deltemplate}"))
}

func TestContextualCannotCallStrictOfWrongContext(t *testing.T) {
	// Can't call a text template from a strict context.
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"Cannot call strictly autoescaped template .foo of kind=\"text\" from incompatible "+
			"context (Context HTML_PCDATA). Strict templates generate extra code to safely call "+
			"templates of other content kinds, but non-strict templates do not: "+
			"{call foo}",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"contextual\"}\n",
			"{call foo}\n",
			"{param x: '' /}\n",
			"{/call}\n",
			"{/template}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"text\"}\n",
			"<b>{$x}</b>\n",
			"{/template}"))

	// assertRewriteFails(t,
	// 	"In file no-path-0:4, template main: "+
	// 		"Cannot call strictly autoescaped template .foo of kind=\"text\" from incompatible "+
	// 		"context (Context HTML_PCDATA). Strict templates generate extra code to safely call "+
	// 		"templates of other content kinds, but non-strict templates do not: "+
	// 		"{delcall foo}",
	// 	join(
	// 		"{namespace ns}\n\n",
	// 		"{template main autoescape=\"contextual\"}\n",
	// 		"{delcall foo}\n",
	// 		"{param x: '' /}\n",
	// 		"{/delcall}\n",
	// 		"{/template}"),
	// 	join(
	// 		"{delpackage dp1}\n",
	// 		"{namespace ns}\n\n",
	// 		"/** @param x */\n",
	// 		"{deltemplate .foo autoescape=\"strict\" kind=\"text\"}\n",
	// 		"<b>{$x}</b>\n",
	// 		"{/deltemplate}"),
	// 	join(
	// 		"{delpackage dp2}\n",
	// 		"{namespace ns}\n\n",
	// 		"/** @param x */\n",
	// 		"{deltemplate .foo autoescape=\"strict\" kind=\"text\"}\n",
	// 		"<i>{$x}</i>\n",
	// 		"{/deltemplate}"))
}

// func TestStrictModeAllowsNonAutoescapeCancellingDirectives(t *testing.T) {
//     SoyFileSetNode soyTree = SharedTestUtils.parseSoyFiles(join(
//         "{template main autoescape=\"strict\"}\n",
//           "<b>{$foo |customOtherDirective}</b>\n",
//         "{/template}"));
//     String rewrittenTemplate = rewrittenSource(soyTree);
//     assertEquals(
//         join(
//             "{template main autoescape=\"strict\"}\n",
//               "<b>{$foo |customOtherDirective |escapeHtml}</b>\n",
//             "{/template}"),
//         rewrittenTemplate.trim());
//   }

func TestTextDirectiveBanned(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:2, template main: "+
			"Print directive |text is only for internal use by the Soy compiler.",
		join(
			"{template main autoescape=\"contextual\"}\n",
			"{$foo |text}\n",
			"{/template}"))
}

func TestStrictModeDoesNotYetHaveDefaultParamKind(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"In strict templates, {let}...{/let} blocks require an explicit kind=\"<type>\". "+
			"This restriction will be lifted soon once a reasonable default is chosen. "+
			"(Note that {let $x: $y /} is NOT subject to this restriction). "+
			"Cause: {let $x}",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"{let $x}No Kind{/let}\n",
			"{/template}"))
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"In strict templates, {param}...{/param} blocks require an explicit kind=\"<type>\". "+
			"This restriction will be lifted soon once a reasonable default is chosen. "+
			"(Note that {param x: $y /} is NOT subject to this restriction). "+
			"Cause: {param x}",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"{call foo}{param x}No Kind{/param}{/call}\n",
			"{/template}"))
	// Test with a non-strict template but in a strict block.
	assertRewriteFails(t,
		"In file no-path:4, template main: "+
			"In strict templates, {let}...{/let} blocks require an explicit kind=\"<type>\". "+
			"This restriction will be lifted soon once a reasonable default is chosen. "+
			"(Note that {let $x: $y /} is NOT subject to this restriction). "+
			"Cause: {let $x}",
		join(
			"{namespace ns}\n\n",
			// Non-strict template.
			"{template main autoescape=\"contextual\"}\n",
			// Strict block in the non-strict template.
			"{let $y kind=\"html\"}",
			// Missing kind attribute in a let in a strict block.
			"{let $x}No Kind{/let}",
			"{$x}",
			"{/let}",
			"\n{/template}"))
}

func TestStrictModeRequiresStartAndEndToBeCompatible(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:3, template main: "+
			"A strict block of kind=\"html\" cannot end in context (Context JS_SQ_STRING). "+
			"Likely cause is an unterminated string literal: "+
			"{template main autoescape=\"strict\"}",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\"}\n",
			"<script>var x='\n",
			"{/template}"))
}

func TestStrictUriMustNotBeEmpty(t *testing.T) {
	assertRewriteFails(t,
		"In file no-path:3, template main: "+
			"A strict block of kind=\"uri\" cannot end in context (Context URI START). "+
			"Likely cause is an unterminated or empty URI: "+
			"{template main autoescape=\"strict\" kind=\"uri\"}",
		join(
			"{namespace ns}\n\n",
			"{template main autoescape=\"strict\" kind=\"uri\"}\n",
			"{/template}"))
}

func TestContextualCanCallStrictModeUri(t *testing.T) {
	// This ensures that a contextual template can use a strict URI -- specifically testing that
	// the contextual call site matching doesn't do an exact match on context (which would be
	// sensitive to whether single quotes or double quotes are used) but uses the logic in
	// Context.isValidStartContextForContentKindLoose().
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<a href=\"{call .bar data=\"all\" /}\">Test</a>",
			"\n{/template}\n\n",
			"{template .bar autoescape=\"strict\" kind=\"uri\"}\n",
			"http://www.google.com/search?q={$x |escapeUri}",
			"\n{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"contextual\"}\n",
			"<a href=\"{call .bar data=\"all\" /}\">Test</a>",
			"\n{/template}\n\n",
			"{template .bar autoescape=\"strict\" kind=\"uri\"}\n",
			"http://www.google.com/search?q={$x}",
			"\n{/template}"))
}

func TestStrictAttributes(t *testing.T) {
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"onclick={$x |escapeJsValue |escapeHtmlAttributeNospace} ",
			"style='{$y |filterCssValue |escapeHtmlAttribute}' ",
			"checked ",
			"foo=\"bar\" ",
			"title='{$z |escapeHtmlAttribute}'",
			"\n{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"onclick={$x} ",
			"style='{$y}' ",
			"checked ",
			"foo=\"bar\" ",
			"title='{$z}'",
			"\n{/template}"))
}

func TestStrictAttributesMustBeTerminated(t *testing.T) {
	// Basic "forgot to close attribute" issue.
	assertRewriteFails(t,
		"In file no-path:3, template ns.foo: "+
			"A strict block of kind=\"attributes\" cannot end in context "+
			"(Context HTML_NORMAL_ATTR_VALUE PLAIN_TEXT DOUBLE_QUOTE). "+
			"Likely cause is an unterminated attribute value, or ending with an unquoted attribute: "+
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"foo=\"{$x}",
			"\n{/template}"))
}

func TestStrictAttributesMustNotEndInUnquotedAttributeValue(t *testing.T) {
	// Ensure that any final attribute-value pair is quoted -- otherwise, if the use site of the
	// value forgets to add spaces, the next attribute will be swallowed.
	assertRewriteFails(t,
		"In file no-path:3, template ns.foo: "+
			"A strict block of kind=\"attributes\" cannot end in context "+
			"(Context JS SCRIPT SPACE_OR_TAG_END DIV_OP). "+
			"Likely cause is an unterminated attribute value, or ending with an unquoted attribute: "+
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"onclick={$x}",
			"\n{/template}"))

	assertRewriteFails(t,
		"In file no-path:3, template ns.foo: "+
			"A strict block of kind=\"attributes\" cannot end in context "+
			"(Context HTML_NORMAL_ATTR_VALUE PLAIN_TEXT SPACE_OR_TAG_END). "+
			"Likely cause is an unterminated attribute value, or ending with an unquoted attribute: "+
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}",
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"title={$x}",
			"\n{/template}"))
}

func TestStrictAttributesCanEndInValuelessAttribute(t *testing.T) {
	// Allow ending in a valueless attribute like "checked". Unfortunately a sloppy user might end
	// up having this collide with another attribute name.
	// TODO: In the future, we might automatically add a space to the end of strict attributes.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"foo=bar checked",
			"\n{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\" kind=\"attributes\"}\n",
			"foo=bar checked",
			"\n{/template}"))
}

func TestStrictModeJavascriptRegexHandling(t *testing.T) {
	// NOTE: This ensures that the call site is treated as a dynamic value, such that it switches
	// from "before regexp" context to "before division" context. Note this isn't foolproof (such
	// as when the expression leads to a full statement) but is generally going to be correct more
	// often.
	assertContextualRewriting(t,
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\"}\n",
			"<script>",
			"{call .bar /}/{$x |escapeJsValue}+/{$x |escapeJsRegex}/g",
			"</script>",
			"\n{/template}"),
		join(
			"{namespace ns}\n\n",
			"{template .foo autoescape=\"strict\"}\n",
			"<script>",
			"{call .bar /}/{$x}+/{$x}/g",
			"</script>",
			"\n{/template}"))
}

// public void testStrictModeEscapesCallSites() {
//   String source =
//       "{namespace ns}\n\n" +
//       "{template .main autoescape=\"strict\"}\n" +
//         "{call .htmlTemplate /}" +
//         "<script>var x={call .htmlTemplate /};</script>\n" +
//         "<script>var x={call .jsTemplate /};</script>\n" +
//         "{call .externTemplate /}" +
//       "\n{/template}\n\n" +
//       "{template .htmlTemplate autoescape=\"strict\"}\n" +
//         "Hello World" +
//       "\n{/template}\n\n" +
//       "{template .jsTemplate autoescape=\"strict\" kind=\"js\"}\n" +
//         "foo()" +
//       "\n{/template}";

//   SoyFileSetNode soyTree = SharedTestUtils.parseSoyFiles(source);
//   new CheckEscapingSanityVisitor().exec(soyTree);
//   new ContextualAutoescaper(SOY_PRINT_DIRECTIVES).rewrite(soyTree);
//   TemplateNode mainTemplate = soyTree.getChild(0).getChild(0);
//   assertEquals("Sanity check", "ns.main", mainTemplate.getTemplateName());
//   final List<CallNode> callNodes = SoytreeUtils.getAllNodesOfType(
//       mainTemplate, CallNode.class);
//   assertEquals(4, callNodes.size());
//   assertEquals("HTML->HTML escaping should be pruned",
//       ImmutableList.of(), callNodes.get(0).getEscapingDirectiveNames());
//   assertEquals("JS -> HTML call should be escaped",
//       ImmutableList.of("|escapeJsValue"), callNodes.get(1).getEscapingDirectiveNames());
//   assertEquals("JS -> JS pruned",
//       ImmutableList.of(), callNodes.get(2).getEscapingDirectiveNames());
//   assertEquals("HTML -> extern call should be escaped",
//       ImmutableList.of("|escapeHtml"), callNodes.get(3).getEscapingDirectiveNames());
// }

// public void testStrictModeOptimizesDelegates() {
//   String source =
//       "{namespace ns}\n\n" +
//       "{template .main autoescape=\"strict\"}\n" +
//         "{delcall ns.delegateHtml /}" +
//         "{delcall ns.delegateText /}" +
//       "\n{/template}\n\n" +
//       "/** A delegate returning HTML. */\n" +
//       "{deltemplate ns.delegateHtml autoescape=\"strict\"}\n" +
//         "Hello World" +
//       "\n{/deltemplate}\n\n" +
//       "/** A delegate returning JS. */\n" +
//       "{deltemplate ns.delegateText autoescape=\"strict\" kind=\"text\"}\n" +
//         "Hello World" +
//       "\n{/deltemplate}";

//   SoyFileSetNode soyTree = SharedTestUtils.parseSoyFiles(source);
//   new CheckEscapingSanityVisitor().exec(soyTree);
//   new ContextualAutoescaper(SOY_PRINT_DIRECTIVES).rewrite(soyTree);
//   TemplateNode mainTemplate = soyTree.getChild(0).getChild(0);
//   assertEquals("Sanity check", "ns.main", mainTemplate.getTemplateName());
//   final List<CallNode> callNodes = SoytreeUtils.getAllNodesOfType(
//       mainTemplate, CallNode.class);
//   assertEquals(2, callNodes.size());
//   assertEquals("We're compiling a complete set; we can optimize based on usages.",
//       ImmutableList.of(), callNodes.get(0).getEscapingDirectiveNames());
//   assertEquals("HTML -> TEXT requires escaping",
//       ImmutableList.of("|escapeHtml"), callNodes.get(1).getEscapingDirectiveNames());
// }

// // TODO: Tests for dynamic attributes: <a on{$name}="...">,
// // <div data-{$name}={$value}>

func join(parts ...string) string {
	return strings.Join(parts, "")
}

func assertContextualRewriting(t *testing.T, expected, input string) {
	var registry = template.Registry{}
	var tree, err = parse.SoyFile("", input, nil)
	if err != nil {
		t.Errorf("parse error: %s", err)
		return
	}
	registry.Add(tree)
	err = Strict(&registry)
	if err != nil {
		t.Error(err)
		return
	}

	var actual = ""
	for _, node := range registry.SoyFiles {
		actual += node.String()
	}
	actual = strings.TrimSpace(actual)

	if actual != expected {
		t.Errorf("input:\n%s\n\nexpected:\n%s\n\ngot:\n%s", input, expected, actual)
	}
}

func assertContextualRewritingNoop(t *testing.T, expected string) {
	assertContextualRewriting(t, expected, expected)
}

func assertRewriteFails(t *testing.T, msg string, input ...string) {
	// TODO
}
