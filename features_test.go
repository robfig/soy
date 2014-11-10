package soy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"testing"

	"github.com/robertkrimen/otto"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/soyjs"
)

type d map[string]interface{}

type featureTest struct {
	name   string
	data   d
	output string
}

var featureTests = []featureTest{
	{"demoComments", nil, `blah blah<br>http://www.google.com<br>`},

	{"demoLineJoining", nil,
		`First second.<br>` +
			`<i>First</i>second.<br>` +
			`Firstsecond.<br>` +
			`<i>First</i> second.<br>` +
			`Firstsecond.<br>`},

	{"demoRawTextCommands", nil,
		`<pre>Space       : AA BB<br>` +
			`Empty string: AABB<br>` +
			`New line    : AA
BB<br>` +
			"Carriage ret: AA\rBB<br>" +
			`Tab         : AA	BB<br>` +
			`Left brace  : AA{BB<br>` +
			`Right brace : AA}BB<br>` +
			`Literal     : AA	BB { CC
  DD } EE {sp}{\n}{rb} FF</pre>`},

	{"demoPrint", d{"boo": "Boo!", "two": 2},
		`Boo!<br>` +
			`Boo!<br>` +
			`3<br>` +
			`Boo!<br>` +
			`3<br>` +
			`88, false.<br>`},

	{"demoPrintDirectives", d{
		"longVarName": "thisIsSomeRidiculouslyLongVariableName",
		"elementId":   "my_element_id",
		"cssClass":    "my_css_class",
	}, `insertWordBreaks:<br>` +
		`<div style="width:150px; border:1px solid #00CC00">thisIsSomeRidiculouslyLongVariableName<br>` +
		`thisI<wbr>sSome<wbr>Ridic<wbr>ulous<wbr>lyLon<wbr>gVari<wbr>ableN<wbr>ame<br>` +
		`</div>id:<br>` +
		`<span id="my_element_id" class="my_css_class" style="border:1px solid #000000">Hello</span>`},

	{"demoAutoescapeTrue", d{"italicHtml": "<i>italic</i>"},
		`&lt;i&gt;italic&lt;/i&gt;<br>` +
			`<i>italic</i><br>`},

	{"demoAutoescapeFalse", d{"italicHtml": "<i>italic</i>"},
		`<i>italic</i><br>` +
			`&lt;i&gt;italic&lt;/i&gt;<br>`},

	{"demoMsg", d{"name": "Ed", "labsUrl": "http://labs.google.com"},
		`Hello Ed!<br>` +
			`Click <a href="http://labs.google.com">here</a> to access Labs.<br>` +
			`Archive<br>` +
			`Archive<br>`},

	{"demoPlural", d{"eggs": 1}, "You have one egg<br>"},
	{"demoPlural", d{"eggs": 2}, "You have 2 eggs<br>"},
	{"demoPlural", d{"eggs": 0}, "You have 0 eggs<br>"},

	{"demoIf", d{"pi": 3.14159}, `3.14159 is a good approximation of pi.<br>`},
	{"demoIf", d{"pi": 2.71828}, `2.71828 is a bad approximation of pi.<br>`},
	{"demoIf", d{"pi": 1.61803}, `1.61803 is nowhere near the value of pi.<br>`},

	{"demoSwitch", d{"name": "Fay"}, `Dear Fay, &nbsp;You've been good this year.&nbsp; --Santa<br>`},
	{"demoSwitch", d{"name": "Go"}, `Dear Go, &nbsp;You've been bad this year.&nbsp; --Santa<br>`},
	{"demoSwitch", d{"name": "Hal"}, `Dear Hal, &nbsp;You don't really believe in me, do you?&nbsp; --Santa<br>`},
	{"demoSwitch", d{"name": "Ivy"}, `Dear Ivy, &nbsp;You've been good this year.&nbsp; --Santa<br>`},

	{"demoForeach", d{"persons": []d{
		{"name": "Jen", "numWaffles": 1},
		{"name": "Kai", "numWaffles": 3},
		{"name": "Lex", "numWaffles": 1},
		{"name": "Mel", "numWaffles": 2},
	}}, `First, Jen ate 1 waffle.<br>` +
		`Then Kai ate 3 waffles.<br>` +
		`Then Lex ate 1 waffle.<br>` +
		`Finally, Mel ate 2 waffles.<br>`},

	{"demoFor", d{"numLines": 3},
		`Line 1 of 3.<br>` +
			`Line 2 of 3.<br>` +
			`Line 3 of 3.<br>` +
			`2... 4... 6... 8... Who do we appreciate?<br>`},

	{"demoCallWithoutParam",
		d{"name": "Neo", "tripInfo": d{"name": "Neo", "destination": "The Matrix"}},
		`Hello world!<br>` +
			`A trip was taken.<br>` +
			`Neo took a trip.<br>` +
			`Neo took a trip to The Matrix.<br>`},

	{"demoCallWithParam", d{
		"name":          "Oz",
		"companionName": "Pip",
		"destinations": []string{
			"Gillikin Country",
			"Munchkin Country",
			"Quadling Country",
			"Winkie Country"}},
		`Oz took a trip to Gillikin Country.<br>` +
			`Pip took a trip to Gillikin Country.<br>` +
			`Oz took a trip to Munchkin Country.<br>` +
			`Oz took a trip to Quadling Country.<br>` +
			`Pip took a trip to Quadling Country.<br>` +
			`Oz took a trip to Winkie Country.<br>`},

	{"demoCallWithParamBlock", d{"name": "Quo"}, `Quo took a trip to Zurich.<br>`},

	{"demoExpressions", d{
		"currentYear": 2008,
		"students": []d{
			{"name": "Rob", "major": "Physics", "year": 1999},
			{"name": "Sha", "major": "Finance", "year": 1980},
			{"name": "Tim", "major": "Engineering", "year": 2005},
			{"name": "Uma", "major": "Biology", "year": 1972},
		}},
		`First student's major: Physics<br>` +
			`Last student's year: 1972<br>` +
			`Random student's major: Biology<br>` +
			`Rob: First. Physics. Scientist. Young. 90s. 90s.<br>` +
			`Sha: Middle. Even. Finance. 80s. 80s.<br>` +
			`Tim: Engineering. Young. 00s. 00s.<br>` +
			`Uma: Last. Even. Biology. Scientist. 70s. 70s.<br>`},

	{"demoDoubleBraces", d{
		"setName":    "prime numbers",
		"setMembers": []int{2, 3, 5, 7, 11, 13},
	},
		`The set of prime numbers is {2, 3, 5, 7, 11, 13, ...}.`},

	// 	{"demoBidiSupport", d{
	// 		"title":  "2008: A BiDi Odyssey",
	// 		"author": "John Doe, Esq.",
	// 		"year":   "1973",
	// 		"keywords": []string{
	// 			"Bi(Di)",
	// 			"2008 (\u05E9\u05E0\u05D4)",
	// 			"2008 (year)",
	// 		}},
	// 		`<div id="title1" style="font-variant:small-caps" >2008: A BiDi Odyssey</div>` +
	// 			`<div id="title2" style="font-variant:small-caps">2008: A BiDi Odyssey</div>by John Doe, Esq. (1973)` +
	// 			`<div id="choose_a_keyword">Your favorite keyword: ` +
	// 			`<select><option value="Bi(Di)">Bi(Di)</option>` +
	// 			`<option value="2008 (???)">?2008 (???)??</option>` +
	// 			`<option value="2008 (year)">2008 (year)</option></select></div>` +
	// 			`<a href="#" style="float:right">Help</a><br>`},

}

// TestFeatures runs through the feature examples from:
// http://closure-templates.googlecode.com/svn/trunk/examples/features.soy
// The expected output is taken directly from that produced by the Java program.
func TestFeatures(t *testing.T) {
	rand.Seed(1) // two of the templates use a random number.
	runFeatureTests(t, featureTests)
}

func TestMsgs(t *testing.T) {

}

func BenchmarkLexParseFeatures(b *testing.B) {
	var (
		features = mustReadFile("testdata/features.soy")
		simple   = mustReadFile("testdata/simple.soy")
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var _, err = NewBundle().
			AddGlobalsFile("testdata/FeaturesUsage_globals.txt").
			AddTemplateString("", features).
			AddTemplateString("", simple).
			Compile()
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkExecuteFeatures(b *testing.B) {
	var (
		features = mustReadFile("testdata/features.soy")
		simple   = mustReadFile("testdata/simple.soy")
	)
	var tofu, err = NewBundle().
		AddGlobalsFile("testdata/FeaturesUsage_globals.txt").
		AddTemplateString("", features).
		AddTemplateString("", simple).
		CompileToTofu()
	if err != nil {
		panic(err)
	}
	b.ResetTimer()

	var buf = new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		for _, test := range featureTests {
			// if test.name != "demoAutoescapeTrue" {
			// 	continue
			// }
			buf.Reset()
			err = tofu.Render(buf, "soy.examples.features."+test.name, test.data)
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkExecuteSimple_Soy(b *testing.B) {
	var tofu, err = NewBundle().
		AddTemplateString("", mustReadFile("testdata/simple.soy")).
		CompileToTofu()
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	var buf = new(bytes.Buffer)
	var testdata = []data.Map{
		{"names": data.List{}},
		{"names": data.List{data.String("Rob")}},
		{"names": data.List{data.String("Rob"), data.String("Joe")}},
	}
	for i := 0; i < b.N; i++ {
		for _, data := range testdata {
			buf.Reset()
			err = tofu.Render(buf, "soy.examples.simple.helloNames", data)
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkExecuteSimple_Go(b *testing.B) {
	// from https://groups.google.com/forum/#!topic/golang-nuts/mqRbR7AFJj0
	var fns = template.FuncMap{
		"last": func(x int, a interface{}) bool {
			return x == reflect.ValueOf(a).Len()-1
		},
	}

	var tmpl = template.Must(template.New("").Funcs(fns).Parse(`
{{define "go.examples.simple.helloWorld"}}
Hello world!
{{end}}

{{define "go.examples.simple.helloName"}}
{{if .}}
  Hello {{.}}!
{{else}}
  {{template "go.examples.simple.helloWorld"}}
{{end}}
{{end}}

{{define "go.examples.simple.helloNames"}}
  {{range $i, $name := .names}}
    {{template "go.examples.simple.helloName" $name}}
    {{if last $i $.names | not }}
      <br>
    {{end}}
  {{else}}
    {{template "go.examples.simple.helloWorld"}}
  {{end}}
{{end}}`))

	var buf = new(bytes.Buffer)
	var testdata = []map[string]interface{}{
		{"names": nil},
		{"names": []string{"Rob"}},
		{"names": []string{"Rob", "Joe"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, data := range testdata {
			buf.Reset()
			var err = tmpl.ExecuteTemplate(buf, "go.examples.simple.helloNames", data)
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func BenchmarkSimpleTemplate_Soy(b *testing.B) {
	var tofu, err = NewBundle().
		AddTemplateString("", `
{namespace small}
/**
 * @param foo
 * @param bar
 * @param baz
 */
{template .test}
some {$foo}, some {$bar}, more {$baz}
{/template}`).
		CompileToTofu()
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	var buf = new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		buf.Reset()
		err = tofu.Render(buf, "small.test", data.Map{
			"foo": data.String("foostring"),
			"bar": data.Int(42),
			"baz": data.Bool(true),
		})
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkSimpleTemplate_Go(b *testing.B) {
	var tmpl = template.Must(template.New("").Parse(`
{{define "small.test"}}
some {{.foo}}, some {{.bar}}, more {{.baz}}
{{end}}`))
	b.ResetTimer()
	var buf = new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		buf.Reset()
		var err = tmpl.ExecuteTemplate(buf, "small.test", data.Map{
			"foo": data.String("foostring"),
			"bar": data.Int(42),
			"baz": data.Bool(true),
		})
		if err != nil {
			b.Error(err)
		}
	}
}

// TestFeaturesJavascript runs the javascript compiled by this implementation
// against that compiled by the reference implementation.
func TestFeaturesJavascript(t *testing.T) {
	rand.Seed(14)
	var registry, err = NewBundle().
		AddGlobalsFile("testdata/FeaturesUsage_globals.txt").
		AddTemplateFile("testdata/simple.soy").
		AddTemplateFile("testdata/features.soy").
		Compile()
	if err != nil {
		t.Error(err)
		return
	}
	var otto = initJs(t)
	for _, soyfile := range registry.SoyFiles {
		var buf bytes.Buffer
		var err = soyjs.Write(&buf, soyfile, soyjs.Options{})
		if err != nil {
			t.Error(err)
			return
		}
		_, err = otto.Run(buf.String())
		if err != nil {
			t.Error(err)
			return
		}
	}

	// Now run all the tests.
	for _, test := range featureTests {
		var jsonData, _ = json.Marshal(test.data)
		var renderStatement = fmt.Sprintf("%s(JSON.parse(%q));",
			"soy.examples.features."+test.name, string(jsonData))
		var actual, err = otto.Run(renderStatement)
		if err != nil {
			t.Errorf("render error: %v\n%v", err, string(jsonData))
			continue
		}

		if actual.String() != test.output {
			t.Errorf("%s\nexpected\n%q\n\ngot\n%q", test.name, test.output, actual.String())
		}
	}
}

func initJs(t *testing.T) *otto.Otto {
	var otto = otto.New()
	soyutilsFile, err := os.Open("soyjs/lib/soyutils.js")
	if err != nil {
		panic(err)
	}
	// remove any non-otto compatible regular expressions
	var soyutilsBuf bytes.Buffer
	var scanner = bufio.NewScanner(soyutilsFile)
	var i = 1
	for scanner.Scan() {
		switch i {
		case 2565, 2579, 2586:
			// skip these regexes
			// soy.esc.$$FILTER_FOR_FILTER_CSS_VALUE_
			// soy.esc.$$FILTER_FOR_FILTER_HTML_ATTRIBUTES_
			// soy.esc.$$FILTER_FOR_FILTER_HTML_ELEMENT_NAME_
		default:
			soyutilsBuf.Write(scanner.Bytes())
			soyutilsBuf.Write([]byte("\n"))
		}
		i++
	}
	// load the soyutils library
	_, err = otto.Run(soyutilsBuf.String())
	if err != nil {
		t.Errorf("soyutils error: %v", err)
		panic(err)
	}
	return otto
}

func runFeatureTests(t *testing.T, tests []featureTest) {
	var features = mustReadFile("testdata/features.soy")
	var tofu, err = NewBundle().
		AddGlobalsFile("testdata/FeaturesUsage_globals.txt").
		AddTemplateString("", features).
		AddTemplateFile("testdata/simple.soy").
		CompileToTofu()
	if err != nil {
		t.Error(err)
		return
	}

	b := new(bytes.Buffer)
	for _, test := range tests {
		b.Reset()
		err = tofu.Render(b, "soy.examples.features."+test.name, test.data)
		if err != nil {
			t.Error(err)
			continue
		}
		if b.String() != test.output {
			t.Errorf("%s\nexpected\n%q\n\ngot\n%q", test.name, test.output, b.String())
		}
	}
}

func mustReadFile(filename string) string {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	content, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return string(content)
}
