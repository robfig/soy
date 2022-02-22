package bytecode

//go:generate go run golang.org/x/tools/cmd/stringer@v0.1.8 -type=Opcode

type Opcode int32

const (
	Nop Opcode = iota

	RawText
	Return

	EndOpcode
)
