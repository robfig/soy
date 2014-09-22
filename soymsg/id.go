package soymsg

import (
	"bytes"
	"fmt"

	"github.com/robfig/soy/ast"
)

// calcID calculates the message ID for the given message node.
// The ID changes if the text content or meaning attribute changes.
// It is invariant to changes in description.
func calcID(n *ast.MsgNode) uint64 {
	var buf bytes.Buffer
	for _, part := range n.Body {
		switch part := part.(type) {
		case *ast.RawTextNode:
			buf.Write(part.Text)
		case *ast.MsgPlaceholderNode:
			buf.WriteString(part.Name)
		default:
			panic(fmt.Sprintf("unrecognized type %T", part))
		}
	}

	var fp = fingerprint(buf.Bytes())
	if n.Meaning != "" {
		var topbit uint64
		if fp&(1<<63) > 0 {
			topbit = 1
		}
		fp = (fp << 1) + topbit + fingerprint([]byte(n.Meaning))
	}

	return fp & 0x7fffffffffffffff
}

// fingerprinting functions ported from official Soy, so that we end up with the
// same message ids.

func fingerprint(str []byte) uint64 {
	var hi = hash32(str, 0, len(str), 0)
	var lo = hash32(str, 0, len(str), 102072)
	if (hi == 0) && (lo == 0 || lo == 1) {
		// Turn 0/1 into another fingerprint
		hi ^= 0x130f9bef
		lo ^= 0x94a0a928
	}
	return (uint64(hi) << 32) | uint64(lo&0xffffffff)
}

func hash32(str []byte, start, limit int, c uint32) uint32 {
	var a uint32 = 0x9e3779b9
	var b uint32 = 0x9e3779b9

	var i int
	for i = start; i+12 <= limit; i += 12 {
		a += (uint32(str[i+0]&0xff) << 0) |
			(uint32(str[i+1]&0xff) << 8) |
			(uint32(str[i+2]&0xff) << 16) |
			(uint32(str[i+3]&0xff) << 24)
		b += (uint32(str[i+4]&0xff) << 0) |
			(uint32(str[i+5]&0xff) << 8) |
			(uint32(str[i+6]&0xff) << 16) |
			(uint32(str[i+7]&0xff) << 24)
		c += (uint32(str[i+8]&0xff) << 0) |
			(uint32(str[i+9]&0xff) << 8) |
			(uint32(str[i+10]&0xff) << 16) |
			(uint32(str[i+11]&0xff) << 24)

		// Mix.
		a -= b
		a -= c
		a ^= (c >> 13)
		b -= c
		b -= a
		b ^= (a << 8)
		c -= a
		c -= b
		c ^= (b >> 13)
		a -= b
		a -= c
		a ^= (c >> 12)
		b -= c
		b -= a
		b ^= (a << 16)
		c -= a
		c -= b
		c ^= (b >> 5)
		a -= b
		a -= c
		a ^= (c >> 3)
		b -= c
		b -= a
		b ^= (a << 10)
		c -= a
		c -= b
		c ^= (b >> 15)
	}

	c += uint32(limit - start)
	switch limit - i { // Deal with rest. Cases fall through.
	case 11:
		c += uint32(str[i+10]&0xff) << 24
		fallthrough
	case 10:
		c += uint32(str[i+9]&0xff) << 16
		fallthrough
	case 9:
		c += uint32(str[i+8]&0xff) << 8
		// the first byte of c is reserved for the length
		fallthrough
	case 8:
		b += uint32(str[i+7]&0xff) << 24
		fallthrough
	case 7:
		b += uint32(str[i+6]&0xff) << 16
		fallthrough
	case 6:
		b += uint32(str[i+5]&0xff) << 8
		fallthrough
	case 5:
		b += uint32(str[i+4] & 0xff)
		fallthrough
	case 4:
		a += uint32(str[i+3]&0xff) << 24
		fallthrough
	case 3:
		a += uint32(str[i+2]&0xff) << 16
		fallthrough
	case 2:
		a += uint32(str[i+1]&0xff) << 8
		fallthrough
	case 1:
		a += uint32(str[i+0] & 0xff)
		// case 0 : nothing left to add
	}

	// Mix.
	a -= b
	a -= c
	a ^= (c >> 13)
	b -= c
	b -= a
	b ^= (a << 8)
	c -= a
	c -= b
	c ^= (b >> 13)
	a -= b
	a -= c
	a ^= (c >> 12)
	b -= c
	b -= a
	b ^= (a << 16)
	c -= a
	c -= b
	c ^= (b >> 5)
	a -= b
	a -= c
	a ^= (c >> 3)
	b -= c
	b -= a
	b ^= (a << 10)
	c -= a
	c -= b
	c ^= (b >> 15)

	return c
}
