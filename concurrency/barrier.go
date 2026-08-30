package concurrency

import "sync"

// Barrier to bariera cykliczna: Wait() blokuje, dopóki nie zbierze się
// `total` uczestników, po czym zwalnia wszystkich naraz i przygotowuje się
// do kolejnej rundy.
type Barrier struct {
	total int
	count int
	// generation rośnie przy każdym zwolnieniu bariery. Bez niego czekające
	// goroutines nie miałyby żadnego predykatu do sprawdzenia po obudzeniu -
	// b.count jest już wtedy zresetowany do b.total.
	generation int
	mutex      *sync.Mutex
	cond       *sync.Cond
}

func NewBarrier(size int) *Barrier {
	mutex := &sync.Mutex{}
	condition := sync.NewCond(mutex)
	return &Barrier{total: size, count: size, mutex: mutex, cond: condition}
}

func (b *Barrier) Wait() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	generation := b.generation
	b.count -= 1

	if b.count == 0 {
		// Ostatni uczestnik: otwiera barierę i rozpoczyna nową rundę.
		b.count = b.total
		b.generation++
		b.cond.Broadcast()
		return
	}

	// sync.Cond.Wait zwalnia mutex na czas oczekiwania, więc po obudzeniu
	// warunek NIE jest automatycznie spełniony (możliwe też wybudzenie przez
	// Broadcast adresowany do innej rundy). Dlatego Wait zawsze wołamy w pętli
	// sprawdzającej predykat - tutaj: "czy moja runda już się zakończyła".
	for generation == b.generation {
		b.cond.Wait()
	}
}
