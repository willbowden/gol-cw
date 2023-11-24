package main

import (
	//"errors"

	"flag"
	"fmt"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	//	"fmt"

	"net"
)

// GOL Logic as in Parallel Implementation

func countAliveCells(p stubs.Params, world [][]byte) int {
	count := 0
	for _, row := range world {
		for _, cellValue := range row {
			if cellValue == 255 {
				count++
			}
		}
	}

	return count
}

func callWorker(y1, y2 int, p stubs.Params, world [][]uint8, ch chan<- [][]uint8, client *rpc.Client) {
	request := stubs.Request{CurrentState: world, Params: stubs.Params(p), Y1: y1, Y2: y2}
	response := new(stubs.Response)
	client.Call(stubs.ProcessSlice, request, response)
	ch <- response.State
}

func calculateNewState(p stubs.Params, g *Gol) [][]uint8 {
	// Make new 2D array for the next frame
	var newFrame [][]uint8

	channels := make([]chan [][]uint8, p.Threads)
	for v := range channels {
		channels[v] = make(chan [][]uint8)
	}

	// Values for dividing up world between n threads
	sliceSize := p.ImageHeight / p.Threads
	remainder := p.ImageHeight % p.Threads

	for i, channel := range channels {
		i += 1
		// Calculate y bounds for thread
		y1 := (i - 1) * sliceSize
		y2 := (i * sliceSize) - 1
		if i == p.Threads {
			y2 += remainder
		}
		// Start worker on its slice
		go callWorker(y1, y2, p, g.state, channel, g.clients[i-1])

	}

	for _, channel := range channels {
		newSlice := <-channel
		newFrame = append(newFrame, newSlice...)
	}

	return newFrame

	// Send complete new frame back to RPC func
}

// Returns a function allowing us to access data without risk of overwriting
func makeImmutableWorld(world [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

// distributor divides the work between workers and interacts with other goroutines.
// func engine(p stubs.Params, state [][]uint8) [][]uint8 {

// 	// Channel to receive new state output from workers

// }

// RPC Requests

type Gol struct {
	state   [][]uint8
	turn    int
	lock    sync.Mutex
	clients []*rpc.Client
	pause   bool
	quit    bool
	signal  chan string
	wg      sync.WaitGroup
}

// calculate new state
func (g *Gol) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)

	req.Params.Threads = 2

	g.quit = false

	// If we're not paused because of a client quit, start from new state.
	// Otherwise, it will just resume processing on the already existing state
	if g.pause == false {
		g.state = req.CurrentState
		g.turn = 0
	}

	for g.turn = g.turn; g.turn < req.Params.Turns && g.quit == false; g.turn++ {
		newFrame := calculateNewState(req.Params, g)
		g.lock.Lock()
		g.state = newFrame
		g.lock.Unlock()
	}

	res.State = g.state

	g.wg.Done()

	return
}

// alive cells count called by the distributor
func (g *Gol) AliveCellsCount(req stubs.Request, res *stubs.CellCount) (err error) {
	g.wg.Add(1)

	g.lock.Lock()
	count := countAliveCells(req.Params, g.state)
	g.lock.Unlock()
	res.Turn = g.turn
	res.CellsCount = count

	g.wg.Done()

	return
}

func (g *Gol) Screenshot(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)

	g.lock.Lock()
	res.State = g.state
	g.lock.Unlock()
	res.CurrentTurn = g.turn

	g.wg.Done()

	return
}

func (g *Gol) PauseBroker(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)

	if g.pause == false {
		g.lock.Lock()
		g.pause = true
	} else {
		g.lock.Unlock()
		g.pause = false
	}

	res.CurrentTurn = g.turn
	res.Paused = g.pause

	g.wg.Done()

	return
}

func (g *Gol) ClientQuit(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)

	g.lock.Lock()
	res.State = g.state
	g.quit = true
	g.lock.Unlock()
	res.CurrentTurn = g.turn
	g.pause = true

	g.wg.Done()

	return
}

func (g *Gol) KillBroker(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)

	for _, client := range g.clients {
		req := new(stubs.Request)
		response := new(stubs.Response)
		client.Call(stubs.KillWorker, req, response)
	}

	g.lock.Lock()
	res.State = g.state
	res.CurrentTurn = g.turn
	g.quit = true
	g.lock.Unlock()

	g.wg.Done()

	defer func() { g.signal <- "KILL" }()

	return
}

func startAccepting(listener net.Listener) {
	rpc.Accept(listener)
}

// Server Handling
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	instances := []string{"54.157.203.179:8030", "54.161.69.22:8030"}
	connections := []*rpc.Client{}

	for _, instance := range instances {
		client, _ := rpc.Dial("tcp", instance)
		connections = append(connections, client)
		defer client.Close()
	}

	listener, _ := net.Listen("tcp", ":"+*pAddr)
	g := Gol{clients: connections, signal: make(chan string, 1)}
	rpc.Register(&g)
	fmt.Println("Server open on port", *pAddr)
	defer listener.Close()
	go startAccepting(listener)
	<-g.signal
	fmt.Println("Server closing...")
	g.wg.Wait()
	close(g.signal)
}
