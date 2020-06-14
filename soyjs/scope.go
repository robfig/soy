package soyjs

import "strconv"

// scope provides a lookup from Soy variable name to the JS name.
// it is pushed and popped upon entering and leaving loop scopes.
type scope struct {
	stack []map[string]string
	n     int
}

func (s *scope) push() {
	s.stack = append(s.stack, make(map[string]string))
}

func (s *scope) pop() {
	s.stack = s.stack[:len(s.stack)-1]
}

// makevar generates and returns a new JS name for the given variable name, adds
// that mapping to this scope.
func (s *scope) makevar(varname string) string {
	s.n++
	var genName = varname + strconv.Itoa(s.n)
	s.stack[len(s.stack)-1][varname] = genName
	return genName
}

func (s *scope) lookup(varname string) string {
	for i := range s.stack {
		val, ok := s.stack[len(s.stack)-i-1][varname]
		if ok {
			return val
		}
	}
	return ""
}

func (s *scope) pushForRange(loopVar string) (lVar, lLimit string) {
	s.n++
	n := strconv.Itoa(s.n)
	s.stack = append(s.stack, map[string]string{
		loopVar:   loopVar + n,
		"__limit": loopVar + "Limit" + n,
		"__index": loopVar + n,
	})
	return loopVar + n,
		loopVar + "Limit" + n
}

func (s *scope) pushForEach(loopVar string) (lVar, lList, lLen, lIndex string) {
	s.n++
	n := strconv.Itoa(s.n)
	s.stack = append(s.stack, map[string]string{
		loopVar:   loopVar + n,
		"__limit": loopVar + "Limit" + n,
		"__index": loopVar + "Index" + n,
	})
	return loopVar + n,
		loopVar + "List" + n,
		loopVar + "Limit" + n,
		loopVar + "Index" + n
}

// looplimit returns the JS variable name for the innermost loop limit.
func (s *scope) looplimit() string {
	return s.lookup("__limit")
}

// looplimit returns the JS variable name for the innermost loop index.
func (s *scope) loopindex() string {
	return s.lookup("__index")
}
