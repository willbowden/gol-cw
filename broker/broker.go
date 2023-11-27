package main

import (
	//"errors"

	"flag"
	"fmt"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"

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

func callWorker(y1, y2 int, p stubs.Params, world [][]uint8, ch chan<- []util.Cell, client *rpc.Client) {
	request := stubs.Request{CurrentState: world, Params: stubs.Params(p), Y1: y1, Y2: y2}
	response := new(stubs.Response)
	client.Call(stubs.ProcessSlice, request, response)
	ch <- response.FlippedCells
}

func (g *Gol) calculateNewState(p stubs.Params) []util.Cell {
	// Make new 2D array for the next frame
	var flippedCells []util.Cell

	channels := make([]chan []util.Cell, p.Threads)
	for v := range channels {
		channels[v] = make(chan []util.Cell)
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
		newSection := <-channel
		flippedCells = append(flippedCells, newSection...)
	}

	g.lock.Lock()
	for _, cell := range flippedCells {
		g.state[cell.Y][cell.X] = 255 - g.state[cell.Y][cell.X]
	}
	g.lock.Unlock()

	return flippedCells

}

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

func (g *Gol) ProcessTurn(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)
	defer g.wg.Done()

	// req.Params.Threads = 4
	// If we're receiving a first-time call from distributor, and we're not paused, start from the new state.
	if req.CurrentState != nil && g.pause == false {
		g.lock.Lock()
		g.state = req.CurrentState
		g.turn = 0
		g.lock.Unlock()
		// Otherwise we are picking up from a client-side quit, so just resume from the existing state
		// Also, return the current state once so the distributor can display GUI correctly.
	} else if req.CurrentState != nil {
		g.lock.Lock()
		res.State = g.state
		g.pause = false
		g.lock.Unlock()
	}

	cellsFlipped := g.calculateNewState(req.Params)
	res.FlippedCells = cellsFlipped

	g.lock.Lock()
	res.CurrentTurn = g.turn
	g.lock.Unlock()

	if g.turn == req.Params.Turns-1 {
		g.lock.Lock()
		res.State = g.state
		g.lock.Unlock()
	}

	g.turn++

	for g.pause {
		// If we're paused, wait until we're unpaused
	}

	return
}

// alive cells count called by the distributor
func (g *Gol) AliveCellsCount(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)
	defer g.wg.Done()

	count := countAliveCells(req.Params, g.state)
	g.lock.Lock()
	res.CellCount = count
	res.CurrentTurn = g.turn
	g.lock.Unlock()

	return
}

func (g *Gol) Screenshot(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)
	defer g.wg.Done()

	g.lock.Lock()
	res.State = g.state
	res.CurrentTurn = g.turn
	g.lock.Unlock()

	return
}

func (g *Gol) PauseBroker(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)
	defer g.wg.Done()

	g.lock.Lock()
	g.pause = !g.pause
	res.CurrentTurn = g.turn
	res.Paused = g.pause
	g.lock.Unlock()

	return
}

func (g *Gol) ClientQuit(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)
	defer g.wg.Done()

	g.lock.Lock()
	res.State = g.state
	g.quit = true
	g.pause = true
	res.CurrentTurn = g.turn
	g.lock.Unlock()

	return
}

func (g *Gol) KillBroker(req stubs.Request, res *stubs.Response) (err error) {
	g.wg.Add(1)
	defer g.wg.Done()

	for _, client := range g.clients {
		req := new(stubs.Request)
		response := new(stubs.Response)
		client.Call(stubs.KillWorker, req, response)
	}

	g.lock.Lock()
	res.State = g.state
	res.CurrentTurn = g.turn
	g.quit = true
	g.pause = false
	g.lock.Unlock()

	g.signal <- "KILL"

	return
}

// Ensure clients have closed connections (and therefore have received responses) before closing server
func (g *Gol) serveConn(conn net.Conn) {
	g.wg.Add(1)
	defer g.wg.Done()
	rpc.ServeConn(conn)
}

func (g *Gol) startAccepting(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// If error is caused by us having intentionally closed the server, return
			if g.quit {
				return
			} else {
				fmt.Println("Accept error:", err)
			}
		} else {
			go g.serveConn(conn)
		}
	}
}

// Server Handling
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	// AWS Node IPs
	instances := []string{"3.89.204.130:8030", "54.237.230.235:8030"}

	// Local IPs for testing
	// instances := []string{"127.0.0.1:8031", "127.0.0.1:8032", "127.0.0.1:8033", "127.0.0.1:8034"}

	connections := []*rpc.Client{}

	for _, instance := range instances {
		client, _ := rpc.Dial("tcp", instance)
		connections = append(connections, client)
	}

	listener, _ := net.Listen("tcp", ":"+*pAddr)
	g := Gol{clients: connections, signal: make(chan string, 1)}
	rpc.Register(&g)
	fmt.Println("Server open on port", *pAddr)
	defer listener.Close()
	go g.startAccepting(listener)
	<-g.signal
	fmt.Println("Server closing...")
	g.wg.Wait()
	for _, client := range g.clients {
		client.Close()
	}
	close(g.signal)
}
