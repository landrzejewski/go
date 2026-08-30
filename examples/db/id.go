package db

// IDGenerator produces record ids. The interface is deliberately sealed: next and
// seed are unexported, so only this package can implement it.
type IDGenerator interface {
	next() int64
	// seed sets the counter to the last id read back from the stored state.
	// It belongs on the interface rather than being fished out with an ad-hoc
	// type assertion at the call site.
	seed(lastID int64)
}

// Sequence is not safe for concurrent use - it is called only from the single
// Database.run goroutine, which serialises every command.
type Sequence struct {
	counter int64
}

func (s *Sequence) next() int64 {
	s.counter++
	return s.counter
}

// seed continues the numbering after a restart instead of starting from zero.
func (s *Sequence) seed(lastID int64) {
	if lastID > s.counter {
		s.counter = lastID
	}
}
