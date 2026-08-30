package concurrency

import "sync"

// Semaphore to semafor zliczający: przechowuje pulę `permissions` pozwoleń,
// Acquire pobiera jedno (blokując, gdy pula jest pusta), Release zwraca.
type Semaphore struct {
	mutex       *sync.Mutex
	condition   *sync.Cond
	permissions int
	max         int
}

func NewSemaphore(permissions int) *Semaphore {
	mutex := sync.Mutex{}
	condition := sync.NewCond(&mutex)
	return &Semaphore{
		mutex:       &mutex,
		condition:   condition,
		permissions: permissions,
		max:         permissions,
	}
}

func (s *Semaphore) Acquire() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for s.permissions == 0 {
		s.condition.Wait()
	}
	s.permissions--
}

// Release zwraca jedno pozwolenie. Wywołanie Release bez wcześniejszego
// Acquire tworzyłoby pozwolenia "z powietrza" i rozszczelniło limit, dlatego
// pula nigdy nie rośnie ponad wartość początkową.
func (s *Semaphore) Release() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.permissions >= s.max {
		panic("semaphore: Release bez odpowiadającego Acquire")
	}
	s.permissions++
	// Wystarczy Signal: zwolniliśmy dokładnie jedno pozwolenie, więc budzimy
	// dokładnie jedną goroutine czekającą w Acquire.
	s.condition.Signal()
}
