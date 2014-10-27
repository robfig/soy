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
		str string
	}{
		{3329840836245051515, "zA ztrip zwas ztaken."},
		{6936162475751860807, "zHello z{NAME}!"},
		{7224011416745566687, "zArchiveNoun"},
		{4826315192146469447, "zArchiveVerb"},
		{1234567890123456789, ""},
	}

	for _, test := range tests {
		var actual = bundle.Message(test.id)
		if actual == nil {
			if test.str == "" {
				continue
			}
			t.Errorf("msg not found: %v", test.id)
		}

		var expected = newMessageSingular(test.id, test.str)
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
