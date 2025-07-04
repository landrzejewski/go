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

type WaitingRoomRequest struct {
	action   string // "enter", "leave", "check"
	response chan int
}

type BarberShop struct {
	numBarbers        int
	waitingRoomSize   int
	clientsChan       chan *Client
	waitingRoomChan   chan WaitingRoomRequest
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
		waitingRoomChan:   make(chan WaitingRoomRequest),
		shopOpenDuration:  shopOpenDuration,
		haircutDuration:   haircutDuration,
		clientArrivalRate: clientArrivalRate,
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (bs *BarberShop) waitingRoomManager() {
	currentSize := 0
	for req := range bs.waitingRoomChan {
		switch req.action {
		case "enter":
			if currentSize < bs.waitingRoomSize {
				currentSize++
				req.response <- currentSize
			} else {
				req.response <- -1 // Room full
			}
		case "leave":
			if currentSize > 0 {
				currentSize--
			}
			req.response <- currentSize
		case "check":
			req.response <- currentSize
		}
	}
}

func (bs *BarberShop) barber(id int) {
	defer bs.wg.Done()
	barber := &Barber{id: id}
	fmt.Printf("Barber %d: Ready to work\n", id)

	for client := range bs.clientsChan {
		// Notify waiting room manager that a client is leaving
		req := WaitingRoomRequest{
			action:   "leave",
			response: make(chan int),
		}
		bs.waitingRoomChan <- req
		currentSize := <-req.response

		fmt.Printf("Barber %d: Cutting hair for client %d (waiting room: %d/%d)\n", barber.id, client.id, currentSize, bs.waitingRoomSize)
		time.Sleep(bs.haircutDuration)
		fmt.Printf("Barber %d: Finished cutting hair for client %d\n", barber.id, client.id)
	}

	fmt.Printf("Barber %d: Going home\n", barber.id)
}

func (bs *BarberShop) addClient(id int) {
	client := &Client{id: id}

	// Request to enter the waiting room
	req := WaitingRoomRequest{
		action:   "enter",
		response: make(chan int),
	}
	bs.waitingRoomChan <- req
	currentSize := <-req.response

	if currentSize == -1 {
		fmt.Printf("Client %d: Waiting room full, leaving\n", id)
		return
	}

	select {
	case bs.clientsChan <- client:
		fmt.Printf("Client %d: Entered waiting room (seats occupied: %d/%d)\n", id, currentSize, bs.waitingRoomSize)
	default:
		// This shouldn't happen since we already checked, but handle it gracefully
		// Request to leave since we couldn't actually enter
		leaveReq := WaitingRoomRequest{
			action:   "leave",
			response: make(chan int),
		}
		bs.waitingRoomChan <- leaveReq
		<-leaveReq.response
		fmt.Printf("Client %d: Waiting room full, leaving\n", id)
	}
}

func (bs *BarberShop) Start() {
	fmt.Println("Barber shop is opening!")
	fmt.Printf("Shop configuration: %d barbers, %d waiting room seats\n", bs.numBarbers, bs.waitingRoomSize)

	// Start the waiting room manager
	go bs.waitingRoomManager()

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
			variation := time.Duration(bs.rand.Intn(int(bs.clientArrivalRate / 2)))
			if variation > 0 {
				time.Sleep(variation)
			}

			fmt.Printf("Client %d: Arriving at shop\n", clientID)
			bs.addClient(clientID)
			clientID++

		case <-closingTimer:
			fmt.Println("\nBarber shop is closing! No new clients accepted.")
			shopOpen = false
		}
	}

	close(bs.clientsChan)
	bs.wg.Wait()

	// Close the waiting room channel
	close(bs.waitingRoomChan)

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
