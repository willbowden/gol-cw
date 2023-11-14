package gol

import (
	//	"errors"

	"net/rpc"

	//	"fmt"

	"net"
)

// GOL Logic as in Parallel Implementation

func calculateNewState(p Params, world [][]uint8, turn int, ch chan<- [][]uint8) {
	// Make new 2D array for the next frame
	var newFrame [][]uint8
	immutableWorld := makeImmutableWorld(world)

	ch_out := make(chan [][]uint8)

	go worker(0, p.ImageHeight, immutableWorld, ch_out, p, turn)

	newSlice := <-ch_out
	newFrame = append(newFrame, newSlice...)

	// Send complete new frame to distributor
	ch <- newFrame
}

// Returns a function allowing us to access data without risk of overwriting
func makeImmutableWorld(world [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func engine(p Params, state [][]uint8) [][]uint8 {

	// Channel to receive new state output from workers
	newFrames := make(chan [][]uint8)

	for turn := 0; turn < p.Turns; turn++ {
		// Start calculation of next frame
		go calculateNewState(p, state, turn, newFrames)
		// Await reception from channels
		nextFrame := <-newFrames
		state = nextFrame

	}

	return state

}

// RPC Requests

type Gol struct{}

func (g *Gol) ProcessTurns(req Request, res *Response) (err error) {
	res.State = engine(req.Params, req.CurrentState)
	return

}

// Server Handling
func startEngine() {
	// pAddr := flag.String("port", "8030", "Port to listen on")
	// flag.Parse()
	rpc.Register(&Gol{})
	listener, _ := net.Listen("tcp", ":8030")
	defer listener.Close()
	rpc.Accept(listener)
}
