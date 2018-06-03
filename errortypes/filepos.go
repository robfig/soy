package errortypes

import "fmt"

// ErrFilePos extends the error interface to add details on the file position where the error occurred.
type ErrFilePos interface {
	error
	File() string
	Line() int
	Col() int
}

// NewErrFilePosf creates an error conforming to the ErrFilePos interface.
func NewErrFilePosf(file string, line, col int, format string, args ...interface{}) error {
	return &errFilePos{
		error: fmt.Errorf(format, args),
		file:  file,
		line:  line,
		col:   col,
	}
}

// IsErrFilePos identifies whethere or not the root cause of the provided error is of the ErrFilePos type.
// Wrapped errors are unwrapped via the Cause() function.
func IsErrFilePos(err error) bool {
	if err == nil {
		return false
	}
	err = rootCause(err)

	_, isErrFilePos := err.(ErrFilePos)
	return isErrFilePos
}

// ToErrFilePos converts the input error to an ErrFilePos if possible, or nil if not.
// If IsErrFilePos returns true, this will not return nil.
func ToErrFilePos(err error) ErrFilePos {
	if err == nil {
		return nil
	}
	err = rootCause(err)
	if out, isErrFilePos := err.(ErrFilePos); isErrFilePos {
		return out
	}
	return nil
}

func rootCause(err error) error {
	type causer interface {
		Cause() error
	}

	for {
		if e, ok := err.(causer); ok {
			err = e.Cause()
		} else {
			return err
		}
	}
}

var _ ErrFilePos = &errFilePos{}

type errFilePos struct {
	error
	file string
	line int
	col  int
}

func (e *errFilePos) File() string {
	return e.file
}

func (e *errFilePos) Line() int {
	return e.line
}

func (e *errFilePos) Col() int {
	return e.col
}
