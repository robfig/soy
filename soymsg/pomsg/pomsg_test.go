package pomsg

import (
	"reflect"
	"testing"
)

func TestPOBundle(t *testing.T) {
	var pomsgs, err = Dir("testdata")
	if err != nil {
		t.Error(err)
		return
	}
	var bundle = pomsgs.Bundle("zz")
	var tests = []struct {
		id  uint64
		str []string
	}{
		{3329840836245051515, []string{"zA ztrip zwas ztaken."}},
		{6936162475751860807, []string{"zHello z{NAME}!"}},
		{7224011416745566687, []string{"zArchiveNoun"}},
		{4826315192146469447, []string{"zArchiveVerb"}},
		{1234567890123456789, []string{}},
		{176798647517908084, []string{
			"zYou zhave zone zegg",
			"zYou zhave z{$EGGS_2} zeggs",
			"zYou zhave ztwo zeggs",
		}},
	}

	for _, test := range tests {
		var actual = bundle.Message(test.id)
		if actual == nil {
			if len(test.str) == 0 {
				continue
			}
			t.Errorf("msg not found: %v", test.id)
		}

		var pluralVar = ""
		if len(test.str) > 1 {
			pluralVar = "EGGS_1"
		}
		var expected = newMessage(test.id, pluralVar, test.str)
		if !reflect.DeepEqual(&expected, actual) {
			t.Errorf("expected:\n%v\ngot:\n%v", expected, actual)
		}
	}
}

func TestPOBundleNotFound(t *testing.T) {
	var pomsgs, err = Dir("testdata")
	if err != nil {
		t.Error(err)
		return
	}

	var bundle = pomsgs.Bundle("xx")
	if bundle != nil {
		t.Errorf("expected null bundle, got %#v", bundle)
	}
}

func TestPlural(t *testing.T) {
	var pomsgs, err = Dir("testdata")
	if err != nil {
		t.Error(err)
		return
	}

	const locale = "zz"
	var bundle = pomsgs.Bundle(locale)
	if bundle.Locale() != locale {
		t.Errorf("actual %v != %v expected", bundle.Locale(), locale)
	}

	type test struct{ n, r int }
	var tests = []test{
		{1, 0},
		{2, 1},
		{3, 2},
		{0, 2},
	}
	for _, test := range tests {
		var actual = bundle.PluralCase(test.n)
		if actual != test.r {
			t.Errorf("actual %v != %v expected", actual, test.n)
		}
	}
}
