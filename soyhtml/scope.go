package soyhtml

import "github.com/robfig/soy/data"

// scope handles variable assignment and lookup within a template.
// it is a stack of data maps, each of which corresponds to variable scope.
// assignments made deeper in the stack take precedence over earlier ones.
type scope []scopeframe

// scopeframe is a single piece of the overall variable assignment.
type scopeframe struct {
	vars    data.Map // map of variable name to value
	entered bool     // true if this was the initial frame for a template
}

func newScope(m data.Map) scope {
	return scope{{m, false}}
}

// push creates a new scope
func (s *scope) push() {
	*s = append(*s, scopeframe{make(data.Map), false})
}

// pop discards the last scope pushed.
func (s *scope) pop() {
	*s = (*s)[:len(*s)-1]
}

// set adds a new binding to the deepest scope
func (s scope) set(k string, v data.Value) {
	s[len(s)-1].vars[k] = v
}

// lookup checks the variable scopes, deepest out, for the given key
func (s scope) lookup(k string) data.Value {
	for i := range s {
		var elem = s[len(s)-i-1].vars
		if val, ok := elem[k]; ok {
			return val
		}
	}
	return data.Undefined{}
}

// alldata returns a new scope for use when passing data="all" to a template.
func (s scope) alldata() scope {
	for i := range s {
		var ri = len(s) - i - 1
		if s[ri].entered {
			return s[: ri+1 : ri+1]
		}
	}
	panic("impossible")
}

// enter records that this is the frame where we enter a template.
// only the frames up to here will be passed in the next data="all"
func (s *scope) enter() {
	(*s)[len(*s)-1].entered = true
	s.push()
}
