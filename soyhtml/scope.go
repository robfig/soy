package soyhtml

import "github.com/robfig/soy/data"

type scope []data.Map // a stack of variable scopes

// push creates a new scope
func (s *scope) push() {
	*s = append(*s, make(data.Map))
}

// pop discards the last scope pushed.
func (s *scope) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *scope) augment(m map[string]interface{}) {
	*s = append(*s, data.New(m).(data.Map))
}

// set adds a new binding to the deepest scope
func (s scope) set(k string, v data.Value) {
	s[len(s)-1][k] = v
}

// lookup checks the variable scopes, deepest out, for the given key
func (s scope) lookup(k string) data.Value {
	for i := range s {
		var elem = s[len(s)-i-1]
		if val, ok := elem[k]; ok {
			return val
		}
	}
	return data.Undefined{}
}

// alldata returns a new scope for use when passing data="all" to a template.
func (s scope) alldata() scope {
	i := s.lookup("__all").(data.Int)
	return s[:i+1 : i+1]
}

// enter records that this is the frame where we enter a template.
// only the frames up to here will be passed in the next data="all"
func (s *scope) enter() {
	s.set("__all", data.Int(len(*s)-1))
	s.push()
}
