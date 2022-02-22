package bytecode

import (
	"errors"
	"fmt"
	"io"

	"github.com/robfig/soy/data"
)

func (prog *Program) Execute(wr io.Writer, templateName string, obj data.Map) error {
	tpl, ok := prog.TemplateByName[templateName]
	if !ok {
		return errors.New("template not found")
	}
	state := &state{
		prog: prog,
		wr:   wr,
	}
	return state.run(tpl.PC)
}

// state represents the state of an execution.
type state struct {
	prog *Program
	wr   io.Writer
}

func (s *state) run(pc int) error {
	code := s.prog.Instr
	for ip := pc; ip < len(code); {
		op := code[ip]
		ip++

		switch op {
		// Output nodes ----------
		case RawText:
			arg := code[ip]
			ip++
			if _, err := s.wr.Write(s.prog.RawTexts[arg]); err != nil {
				return s.errorf("%s", err)
			}

		case Return:
			return nil
		default:
			return s.errorf("unknown op: %s", op)
		}
	}
	return s.errorf("eof")
}

func (s *state) errorf(str string, args ...interface{}) error {
	return fmt.Errorf(str, args...)
}
