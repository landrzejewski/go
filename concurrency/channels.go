package concurrency

import (
	"fmt"
	"time"
)

func Run() {
	/*channel := make(chan int)

	go func() {
		channel <- 1
		println("after first")
		channel <- 2
	}()
	value, isNotClosed := <-channel
	fmt.Printf("%d (isClosed: %v)\n", value, !isNotClosed)
	value, isNotClosed = <-channel
	fmt.Printf("%d (isClosed: %v)\n", value, !isNotClosed)*/

	// var sendChannel chan<- int = make(chan int)    // Send-only channel
	// var receiveChannel <-chan int = make(chan int) // Receive-only channel

	/*buffChannel := make(chan int, 3) // Create a buffered channel with a capacity of 3
	buffChannel <- 1                 // Non-blocking, buffer has space
	buffChannel <- 2                 // Non-blocking, buffer has space
	buffChannel <- 3                 // Non-blocking, buffer is now full
	fmt.Println("After")
	*/
	/*
		You can close a channel using the close function to indicate that no more
		values will be sent on it. Receiving from a closed channel will continue
		to return values until the buffer is empty, after which it will return
		the zero value for the channel’s type.
	*/

	//close(buffChannel)

	/*channel := make(chan int, 3)

	go func() {
		for i := 0; i < 10; i++ {
			channel <- i
		}
		close(channel)
		// channel <- 2 // panic, channel already closed
	}()

	for value := range channel {
		fmt.Println(value)
		time.Sleep(time.Millisecond * 1000)
	}*/

	channel1 := make(chan int)
	channel2 := make(chan int)

	go func() { channel1 <- 1 }()
	go func() { channel2 <- 2 }()

	/*
		The select statement in Go allows you to wait on multiple channel operations.
		It chooses a case that is ready to proceed, allowing you to handle multiple channels
		concurrently. If several cases are ready, one is picked uniformly at random -
		the order of the cases in the source code does not give any of them priority.
	*/

	for range 2 {
		select {
		case msg1 := <-channel1:
			fmt.Println("msg1", msg1)
		case msg2 := <-channel2:
			fmt.Println("msg2", msg2)
		case <-time.After(time.Second * 5):
		}
	}

	channel := make(chan string)
	listeners := make([]chan string, 3)
	for i := range listeners {
		listeners[i] = make(chan string)
		go listener(i, listeners[i])
	}
	go broadcaster(channel, listeners)

	for i := 0; i < 5; i++ {
		channel <- fmt.Sprintf("Message %d", i)
	}

	// Zamknięcie kanału źródłowego kończy broadcaster, który w swoim defer
	// zamyka kanały słuchaczy - dzięki temu wszystkie goroutines kończą się
	// same i nie zostaje żaden wyciek.
	close(channel)
	// Czekamy, aż słuchacze wypiszą to, co jeszcze mają w kolejce.
	time.Sleep(1 * time.Second)
}

func listener(id int, channel <-chan string) {
	for msg := range channel {
		fmt.Println(msg, id)
	}
}

func broadcaster(channel <-chan string, listeners []chan string) {
	// Broadcaster jest jedynym nadawcą dla kanałów słuchaczy, więc to on musi
	// je zamknąć. Bez tego `for range` w każdym listenerze nigdy się nie kończy
	// i po zamknięciu `channel` zostają trzy zawieszone goroutines.
	defer func() {
		for _, listener := range listeners {
			close(listener)
		}
	}()
	for msg := range channel {
		for _, listener := range listeners {
			listener <- msg
		}
	}
}
