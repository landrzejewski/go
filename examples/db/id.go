package db

type IdGenerator interface {
	next() int64
}

// Sequence nie jest bezpieczne dla współbieżności - wołane jest wyłącznie
// z pojedynczej goroutine Database.run(), która serializuje wszystkie komendy.
type Sequence struct {
	counter int64
}

func (s *Sequence) next() int64 {
	s.counter++
	return s.counter
}

// seed ustawia licznik na ostatnie użyte id wczytane ze stanu bazy, dzięki
// czemu numeracja jest kontynuowana po restarcie zamiast startować od zera.
func (s *Sequence) seed(lastId int64) {
	if lastId > s.counter {
		s.counter = lastId
	}
}
