package concurrency

import (
	"fmt"
	"sync"
	"time"
)

func channelProducer(index int, channel chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Sending message", index, i)
		channel <- fmt.Sprintf("Data from producer %d - %d", index, i)
	}
	fmt.Println("Producer finished", index)
}

// channelConsumer kończy się dopiero wtedy, gdy kanał zostanie zamknięty
// I opróżniony - `for range` po kanale robi dokładnie to. Dzięki temu żadna
// wiadomość nie ginie i nie potrzeba żadnego timeoutu.
func channelConsumer(index int, channel <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for message := range channel {
		fmt.Printf("Consumer %d received: %v\n", index, message)
	}
	fmt.Println("Consumer finished", index)
}

func ProducerConsumerChannels() {
	channel := make(chan string, 5)

	// Dwie osobne grupy oczekiwania: kanał zamykamy po producentach (bo to oni
	// są nadawcami), a dopiero potem czekamy na konsumentów, żeby zdążyli
	// wybrać resztę z bufora.
	//
	// Wcześniejsza wersja kończyła konsumentów przez time.After(15s). Było to
	// błędne z trzech powodów: (1) gdyby oba konsumenty odpadły przy wciąż
	// pracujących producentach, 5-elementowy bufor by się zapełnił i wszyscy
	// producenci zablokowaliby się na stałe na `channel <-`, a wg.Wait()
	// zakleszczyłby się na zawsze; (2) wiadomości pozostałe w buforze ginęły;
	// (3) close(channel) po zakończeniu wszystkich było już tylko no-opem.
	var producers, consumers sync.WaitGroup

	for i := 0; i < 5; i++ {
		producers.Add(1)
		go channelProducer(i, channel, &producers)
	}

	for i := 0; i < 2; i++ {
		consumers.Add(1)
		go channelConsumer(i, channel, &consumers)
	}

	producers.Wait()
	close(channel) // zamyka strona nadawcza, gdy wszyscy nadawcy skończyli
	consumers.Wait()
}
