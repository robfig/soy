package errortypes_test

import (
	"errors"
	"testing"

	"github.com/theothertomelliott/soy/errortypes"
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
		{
			name: "wrapped error",
			in: wrapErr(
				errortypes.NewErrFilePosf("file.soy", 1, 2, "message"),
			),
			out: true,
		},
		{
			name: "multi wrapped error",
			in: wrapErr(
				wrapErr(
					errortypes.NewErrFilePosf("file.soy", 1, 2, "message"),
				),
			),
			out: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := errortypes.IsErrFilePos(test.in)
			if got != test.out {
				t.Errorf("Expected %v, got %v", test.out, got)
			}
		})
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
		{
			name: "wrapped error",
			in: wrapErr(
				errortypes.NewErrFilePosf("file2.soy", 3, 4, "message"),
			),
			expectNil:        false,
			expectedFilename: "file2.soy",
			expectedLine:     3,
			expectedCol:      4,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := errortypes.ToErrFilePos(test.in)
			if test.expectNil && got != nil {
				t.Errorf("expected ErrFilePos to be nil")
			}
			if !test.expectNil {
				if got == nil {
					t.Errorf("expected ErrFilePos to be non-nil")
					return
				}
				if got.File() != test.expectedFilename {
					t.Errorf("expected file '%s', got '%s'", test.expectedFilename, got.File())
				}
				if got.Line() != test.expectedLine {
					t.Errorf("expected line %d, got %d", test.expectedLine, got.Line())
				}
				if got.Col() != test.expectedCol {
					t.Errorf("expected col %d, got %d", test.expectedCol, got.Col())
				}
			}
		})
	}
}

func wrapErr(err error) error {
	return &wrappedError{
		cause: err,
	}
}

type wrappedError struct {
	cause error
}

func (w *wrappedError) Error() string {
	return "wrapped"
}

func (w *wrappedError) Cause() error {
	return w.cause
}
