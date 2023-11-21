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
	quit    bool
}

// calculate new state
func (g *Gol) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {
	// get new state : set for response state
	req.Params.Threads = 2
	g.state = req.CurrentState
	for g.turn = 0; g.turn < req.Params.Turns && g.quit == false; g.turn++ {
		newFrame := calculateNewState(req.Params, g)
		g.lock.Lock()
		g.state = newFrame
		g.lock.Unlock()
	}

	res.State = g.state
	return
}

// alive cells count called by the distributor
func (g *Gol) AliveCellsCount(req stubs.Request, res *stubs.CellCount) (err error) {
	g.lock.Lock()
	count := countAliveCells(req.Params, g.state)
	g.lock.Unlock()
	res.Turn = g.turn
	res.CellsCount = count
	return
}

func (g *Gol) QuitBroker(req stubs.Request, res *stubs.Response) (err error) {
	g.lock.Lock()
	res.State = g.state
	g.lock.Unlock()
	res.CurrentTurn = g.turn
	g.quit = true
	return

}

// Server Handling
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	instances := []string{"54.196.76.157:8030", "52.55.224.116:8030"}
	connections := []*rpc.Client{}

	for _, instance := range instances {
		client, _ := rpc.Dial("tcp", instance)
		connections = append(connections, client)
		fmt.Println(client)
		defer client.Close()
	}

	rpc.Register(&Gol{clients: connections})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	fmt.Println("Server open on port", *pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
