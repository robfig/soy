package soy

import "testing"

func TestNullToString(t *testing.T) {
	if toString(nullValue) != "null" {
		t.Errorf("expected nullValue to print 'null', got %q", toString(nullValue))
	}
}

func TestUndefinedToString(t *testing.T) {
	defer func() { recover() }()
	str := toString(undefinedValue)
	t.Errorf("expected panic when trying to print undefined value, got %q", str)
}
