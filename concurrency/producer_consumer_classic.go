package concurrency

import (
	"fmt"
	"sync"

	"training.pl/go/common"
)

var storage = common.Stack[int]{}
var mutex = sync.Mutex{}
var producerCond = sync.NewCond(&mutex)
var consumerCond = sync.NewCond(&mutex)

func producer(wg *sync.WaitGroup) {
	for range 100 {
		mutex.Lock()
		for storage.Size() >= 10 {
			fmt.Println("Producer waiting - storage is full")
			producerCond.Wait()
		}
		fmt.Println("Producing")
		storage.Push(0)
		consumerCond.Broadcast()
		mutex.Unlock()
	}
	wg.Done()
}

func consumer(wg *sync.WaitGroup) {
	for range 200 {
		mutex.Lock()
		for storage.Size() == 0 {
			fmt.Println("Consumer waiting - storage is empty")
			consumerCond.Wait()
		}
		fmt.Println("Consuming")
		storage.Pop()
		producerCond.Broadcast()
		mutex.Unlock()
	}
	wg.Done()
}

func ProducerConsumerClassic() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go producer(&wg)
	go producer(&wg)
	go consumer(&wg)
	wg.Wait()
}
