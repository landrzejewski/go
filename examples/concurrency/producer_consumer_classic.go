package concurrency

import (
	"fmt"
	"sync"

	"training.pl/go/examples/common"
)

// The pre-channel way to write producer/consumer: shared state under one mutex,
// with a separate sync.Cond per waiting side. Two conditions on ONE mutex is the
// point of the example - a producer blocked on "storage is full" and a consumer
// blocked on "storage is empty" must not wake each other up.
//
// Package-level state keeps the example short, but it is also its main weakness:
// the demo cannot be run twice in one process and is untestable in parallel. The
// channel version in producer_consumer_channels.go needs none of this.
var (
	storage      = common.Stack[int]{}
	mutex        = sync.Mutex{}
	producerCond = sync.NewCond(&mutex)
	consumerCond = sync.NewCond(&mutex)
)

func producer(wg *sync.WaitGroup) {
	defer wg.Done()
	for range 100 {
		mutex.Lock()
		// Always test the predicate in a loop: Wait releases the mutex, so the
		// condition may no longer hold by the time we are woken up.
		for storage.Size() >= 10 {
			fmt.Println("Producer waiting - storage is full")
			producerCond.Wait()
		}
		fmt.Println("Producing")
		storage.Push(0)
		consumerCond.Broadcast()
		mutex.Unlock()
	}
}

func consumer(wg *sync.WaitGroup) {
	defer wg.Done()
	for range 200 {
		mutex.Lock()
		for storage.Size() == 0 {
			fmt.Println("Consumer waiting - storage is empty")
			consumerCond.Wait()
		}
		fmt.Println("Consuming")
		storage.Pop() // the guard above guarantees this succeeds
		producerCond.Broadcast()
		mutex.Unlock()
	}
}

// ProducerConsumerClassic terminates only because the counts balance exactly:
// 2 producers x 100 pushes == 1 consumer x 200 pops. Change any one of those three
// numbers and the demo deadlocks, because the losing side waits on a condition
// nobody will signal again. A production version needs a "no more work" flag that
// the Wait loops also test.
func ProducerConsumerClassic() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go producer(&wg)
	go producer(&wg)
	go consumer(&wg)
	wg.Wait()
}
