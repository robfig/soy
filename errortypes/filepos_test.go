package errortypes_test

import (
	"errors"
	"testing"

	"github.com/robfig/soy/errortypes"
)

func TestIsErrFilePos(t *testing.T) {
	var tests = []struct {
		name string
		in   error
		out  bool
	}{
		{
			name: "nil",
			out:  false,
		},
		{
			name: "errors.New",
			in:   errors.New("an error"),
			out:  false,
		},
		{
			name: "new ErrFilePos",
			in:   errortypes.NewErrFilePosf("file.soy", 1, 2, "message"),
			out:  true,
		},
	}
	for _, test := range tests {
		got := errortypes.IsErrFilePos(test.in)
		if got != test.out {
			t.Errorf("%s: Expected %v, got %v", test.name, test.out, got)
		}
	}
}

func TestToErrFilePos(t *testing.T) {
	var tests = []struct {
		name             string
		in               error
		expectNil        bool
		expectedFilename string
		expectedLine     int
		expectedCol      int
	}{
		{
			name:      "nil",
			expectNil: true,
		},
		{
			name:      "errors.New",
			in:        errors.New("an error"),
			expectNil: true,
		},
		{
			name:             "new ErrFilePos",
			in:               errortypes.NewErrFilePosf("file.soy", 1, 2, "message"),
			expectNil:        false,
			expectedFilename: "file.soy",
			expectedLine:     1,
			expectedCol:      2,
		},
	}
	for _, test := range tests {
		got := errortypes.ToErrFilePos(test.in)
		if test.expectNil && got != nil {
			t.Errorf("%s: expected ErrFilePos to be nil", test.name)
		}
		if !test.expectNil {
			if got == nil {
				t.Errorf("%s: expected ErrFilePos to be non-nil", test.name)
				return
			}
			if got.File() != test.expectedFilename {
				t.Errorf("%s: expected file '%s', got '%s'", test.name, test.expectedFilename, got.File())
			}
			if got.Line() != test.expectedLine {
				t.Errorf("%s: expected line %d, got %d", test.name, test.expectedLine, got.Line())
			}
			if got.Col() != test.expectedCol {
				t.Errorf("%s: expected col %d, got %d", test.name, test.expectedCol, got.Col())
			}
		}
	}
}
