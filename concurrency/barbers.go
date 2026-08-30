package concurrency

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Barber struct {
	id int
}

type Client struct {
	id int
}

type BarberShop struct {
	numBarbers      int
	waitingRoomSize int
	// clientsChan JEST poczekalnią: jego bufor ma dokładnie waitingRoomSize
	// miejsc. Wcześniejsza wersja trzymała dodatkowo licznik currentSize
	// w osobnej goroutine - dwa źródła prawdy o tym samym stanie rozjeżdżały
	// się (klient zwiększał licznik przed wysłaniem do kanału, a fryzjer
	// zmniejszał go dopiero po odbiorze), przez co klient bywał odsyłany mimo
	// wolnego miejsca. Tutaj wysyłka z `select`/`default` jest jednocześnie
	// sprawdzeniem i zajęciem miejsca, więc nie ma okna na wyścig (TOCTOU).
	clientsChan       chan *Client
	shopOpenDuration  time.Duration
	haircutDuration   time.Duration
	clientArrivalRate time.Duration
	wg                sync.WaitGroup
	rand              *rand.Rand
}

func NewBarberShop(numBarbers, waitingRoomSize int, shopOpenDuration, haircutDuration, clientArrivalRate time.Duration) *BarberShop {
	return &BarberShop{
		numBarbers:        numBarbers,
		waitingRoomSize:   waitingRoomSize,
		clientsChan:       make(chan *Client, waitingRoomSize),
		shopOpenDuration:  shopOpenDuration,
		haircutDuration:   haircutDuration,
		clientArrivalRate: clientArrivalRate,
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (bs *BarberShop) barber(id int) {
	defer bs.wg.Done()
	barber := &Barber{id: id}
	fmt.Printf("Barber %d: Ready to work\n", id)

	// range po kanale: fryzjer śpi (blokuje się), gdy nie ma klientów, budzi
	// się na nowego, a kończy dopiero gdy kanał jest zamknięty I pusty -
	// czyli po zamknięciu zakładu i obsłużeniu całej poczekalni.
	for client := range bs.clientsChan {
		fmt.Printf("Barber %d: Cutting hair for client %d (waiting room: %d/%d)\n",
			barber.id, client.id, len(bs.clientsChan), bs.waitingRoomSize)
		time.Sleep(bs.haircutDuration)
		fmt.Printf("Barber %d: Finished cutting hair for client %d\n", barber.id, client.id)
	}

	fmt.Printf("Barber %d: Going home\n", barber.id)
}

func (bs *BarberShop) addClient(id int) {
	client := &Client{id: id}

	// Sprawdzenie i zajęcie miejsca w jednej niepodzielnej operacji.
	select {
	case bs.clientsChan <- client:
		fmt.Printf("Client %d: Entered waiting room (seats occupied: %d/%d)\n",
			id, len(bs.clientsChan), bs.waitingRoomSize)
	default:
		fmt.Printf("Client %d: Waiting room full, leaving\n", id)
	}
}

func (bs *BarberShop) Start() {
	fmt.Println("Barber shop is opening!")
	fmt.Printf("Shop configuration: %d barbers, %d waiting room seats\n", bs.numBarbers, bs.waitingRoomSize)

	for i := 1; i <= bs.numBarbers; i++ {
		bs.wg.Add(1)
		go bs.barber(i)
	}

	closingTimer := time.After(bs.shopOpenDuration)
	clientID := 1

	clientTicker := time.NewTicker(bs.clientArrivalRate)
	defer clientTicker.Stop()

	shopOpen := true

	for shopOpen {
		select {
		case <-clientTicker.C:
			// rand.Intn panikuje dla n <= 0, więc losujemy tylko wtedy, gdy
			// przedział jest dodatni.
			if half := int(bs.clientArrivalRate / 2); half > 0 {
				if variation := time.Duration(bs.rand.Intn(half)); variation > 0 {
					time.Sleep(variation)
				}
			}

			fmt.Printf("Client %d: Arriving at shop\n", clientID)
			bs.addClient(clientID)
			clientID++

		case <-closingTimer:
			fmt.Println("\nBarber shop is closing! No new clients accepted.")
			shopOpen = false
		}
	}

	// Zamknięcie kanału oznacza "koniec przyjęć". Fryzjerzy dokańczają tych,
	// którzy siedzą już w poczekalni, i dopiero potem wychodzą - zgodnie
	// z regułą "barbers cannot leave until the waiting room is empty".
	close(bs.clientsChan)
	bs.wg.Wait()

	fmt.Println("\nAll barbers have gone home. Shop is closed!")
}

func Barbers() {
	numBarbers := 2
	waitingRoomSize := 5
	shopOpenDuration := 30 * time.Second
	haircutDuration := 10 * time.Second
	clientArrivalRate := 1 * time.Second

	shop := NewBarberShop(numBarbers, waitingRoomSize, shopOpenDuration, haircutDuration, clientArrivalRate)
	shop.Start()
}
