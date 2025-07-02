package common

type Stack struct {
	data []int
}

func (s *Stack) Push(element int) {
	s.data = append(s.data, element)
}

func (s *Stack) Pop() (int, bool) {
	if s.isEmpty() {
		return 0, false
	}
	lastIndex := s.lastIndex()
	element := s.data[lastIndex]
	s.data = s.data[:lastIndex] // [0:lastIndex)
	return element, true
}

func (s *Stack) lastIndex() int {
	return s.Size() - 1
}

func (s *Stack) isEmpty() bool {
	return s.Size() == 0
}

func (s *Stack) Size() int {
	return len(s.data)
}
