package common

type Stack[T any] struct {
	data []T
}

func (s *Stack[T]) Push(element T) {
	s.data = append(s.data, element)
}

func (s *Stack[T]) Pop() (T, bool) {
	if s.isEmpty() {
		var empty T
		return empty, false
	}
	lastIndex := s.Size() - 1
	element := s.data[lastIndex]
	// Reslicing only shrinks len - the popped element is still reachable through
	// the backing array, so the GC cannot free it. For Stack[*T] or Stack[string]
	// that is a real memory leak, hence zeroing the slot first.
	var zero T
	s.data[lastIndex] = zero
	s.data = s.data[:lastIndex] // [0:lastIndex)
	return element, true
}

func (s *Stack[T]) isEmpty() bool {
	return s.Size() == 0
}

func (s *Stack[T]) Size() int {
	return len(s.data)
}

/*
type Stack struct {
	data []int
}

func NewStack() *Stack {
	return &Stack{}
}

func (s *Stack) Push(element int) {
	s.data = append(s.data, element)
}

func (s *Stack) Pop() (int, bool) {
	if s.isEmpty() {
		return 0, false
	}
	lastIndex := s.Size() - 1
	element := s.data[lastIndex]
	s.data = s.data[:lastIndex] // [0:lastIndex)
	return element, true
}

func (s *Stack) isEmpty() bool {
	return s.Size() == 0
}

func (s *Stack) Size() int {
	return len(s.data)
}
*/
