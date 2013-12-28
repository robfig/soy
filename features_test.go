package soy

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
)

type featureTest struct {
	name   string
	data   data
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

	{"demoPrint", data{"boo": "Boo!", "two": 2},
		`Boo!<br>` +
			`Boo!<br>` +
			`3<br>` +
			`Boo!<br>` +
			`3<br>` +
			`88, false.<br>`},

	{"demoPrintDirectives", data{
		"longVarName": "thisIsSomeRidiculouslyLongVariableName",
		"elementId":   "my_element_id",
		"cssClass":    "my_css_class",
	}, `insertWordBreaks:<br>` +
		`<div style="width:150px; border:1px solid #00CC00">thisIsSomeRidiculouslyLongVariableName<br>` +
		`thisI<wbr>sSome<wbr>Ridic<wbr>ulous<wbr>lyLon<wbr>gVari<wbr>ableN<wbr>ame<br>` +
		`</div>id:<br>` +
		`<span id="my_element_id" class="my_css_class" style="border:1px solid #000000">Hello</span>`},

	{"demoAutoescapeTrue", data{"italicHtml": "<i>italic</i>"},
		`&lt;i&gt;italic&lt;/i&gt;<br>` +
			`<i>italic</i><br>`},

	{"demoAutoescapeFalse", data{"italicHtml": "<i>italic</i>"},
		`<i>italic</i><br>` +
			`&lt;i&gt;italic&lt;/i&gt;<br>`},

	{"demoMsg", data{"name": "Ed", "labsUrl": "http://labs.google.com"},
		`Hello Ed!<br>` +
			`Click <a href="http://labs.google.com">here</a> to access Labs.<br>` +
			`Archive<br>` +
			`Archive<br>`},

	{"demoIf", data{"pi": 3.14159}, `3.14159 is a good approximation of pi.<br>`},
	{"demoIf", data{"pi": 2.71828}, `2.71828 is a bad approximation of pi.<br>`},
	{"demoIf", data{"pi": 1.61803}, `1.61803 is nowhere near the value of pi.<br>`},

	{"demoSwitch", data{"name": "Fay"}, `Dear Fay, &nbsp;You've been good this year.&nbsp; --Santa<br>`},
	{"demoSwitch", data{"name": "Go"}, `Dear Go, &nbsp;You've been bad this year.&nbsp; --Santa<br>`},
	{"demoSwitch", data{"name": "Hal"}, `Dear Hal, &nbsp;You don't really believe in me, do you?&nbsp; --Santa<br>`},
	{"demoSwitch", data{"name": "Ivy"}, `Dear Ivy, &nbsp;You've been good this year.&nbsp; --Santa<br>`},

	{"demoForeach", data{"persons": []data{
		{"name": "Jen", "numWaffles": 1},
		{"name": "Kai", "numWaffles": 3},
		{"name": "Lex", "numWaffles": 1},
		{"name": "Mel", "numWaffles": 2},
	}}, `First, Jen ate 1 waffle.<br>` +
		`Then Kai ate 3 waffles.<br>` +
		`Then Lex ate 1 waffle.<br>` +
		`Finally, Mel ate 2 waffles.<br>`},

	{"demoFor", data{"numLines": 3},
		`Line 1 of 3.<br>` +
			`Line 2 of 3.<br>` +
			`Line 3 of 3.<br>` +
			`2... 4... 6... 8... Who do we appreciate?<br>`},

	{"demoCallWithoutParam",
		data{"name": "Neo", "tripInfo": data{"name": "Neo", "destination": "The Matrix"}},
		`Hello world!<br>` +
			`A trip was taken.<br>` +
			`Neo took a trip.<br>` +
			`Neo took a trip to The Matrix.<br>`},

	{"demoCallWithParam", data{
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

	{"demoCallWithParamBlock", data{"name": "Quo"}, `Quo took a trip to Zurich.<br>`},

	{"demoExpressions", data{
		"currentYear": 2008,
		"students": []data{
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

	{"demoDoubleBraces", data{
		"setName":    "prime numbers",
		"setMembers": []int{2, 3, 5, 7, 11, 13},
	},
		`The set of prime numbers is {2, 3, 5, 7, 11, 13, ...}.`},

	// 	{"demoBidiSupport", data{
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

func BenchmarkLexParseFeatures(b *testing.B) {
	features := mustReadFile("testdata/features.soy")
	simple := mustReadFile("testdata/simple.soy")
	globals := bytes.NewBufferString(mustReadFile("testdata/FeaturesUsage_globals.txt"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var tofu = New()
		var err = tofu.ParseGlobals(globals)
		if err != nil {
			b.Error(err)
		}
		err = tofu.Parse(features)
		if err != nil {
			b.Error(err)
		}
		err = tofu.Parse(simple)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkExecuteFeatures(b *testing.B) {
	var tofu = New()
	mustParseFile(tofu, "testdata/simple.soy")
	mustParseFile(tofu, "testdata/features.soy")
	mustParseGlobals(tofu, "testdata/FeaturesUsage_globals.txt")
	b.ResetTimer()

	var buf = new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		for _, test := range featureTests {
			// if test.name != "demoAutoescapeTrue" {
			// 	continue
			// }
			buf.Reset()
			tmpl, ok := tofu.Template("soy.examples.features." + test.name)
			if !ok {
				b.Errorf("couldn't find template for test: %s", test.name)
			}
			err := tmpl.Execute(buf, test.data)
			if err != nil {
				b.Error(err)
			}
		}
	}
}

func runFeatureTests(t *testing.T, tests []featureTest) {
	var tofu = New()
	mustParseFile(tofu, "testdata/simple.soy")
	mustParseFile(tofu, "testdata/features.soy")
	mustParseGlobals(tofu, "testdata/FeaturesUsage_globals.txt")

	b := new(bytes.Buffer)
	for _, test := range tests {
		b.Reset()
		tmpl, ok := tofu.Template("soy.examples.features." + test.name)
		if !ok {
			t.Errorf("couldn't find template for test: %s", test.name)
			continue
		}
		err := tmpl.Execute(b, test.data)
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

func mustParseFile(tofu Tofu, filename string) {
	err := tofu.Parse(mustReadFile(filename))
	if err != nil {
		panic(err)
	}
}

func mustParseGlobals(tofu Tofu, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	err = tofu.ParseGlobals(f)
	if err != nil {
		panic(err)
	}
}
