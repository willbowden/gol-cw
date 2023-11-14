package main

import (
	//	"errors"
	"flag"
	"net/rpc"

	//	"fmt"
	"math/rand"
	"net"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// GOL Logic as in Parallel Implementation

func calculateNewState(p Params, c distributorChannels, world [][]uint8, turn int, ch chan<- [][]uint8) {
	// Make new 2D array for the next frame
	var newFrame [][]uint8
	immutableWorld := makeImmutableWorld(world)

	go worker(0, p.ImageHeight, immutableWorld, c.events, channel, p, turn)

	newSlice := <-channel
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

// Check all cells and add live ones to output list
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var liveCells []util.Cell
	for y, row := range world {
		for x, cellValue := range row {
			if cellValue == 255 {
				liveCells = append(liveCells, util.Cell{X: x, Y: y})
			}
		}
	}

	return liveCells
}

// distributor divides the work between workers and interacts with other goroutines.
func engine(turns int, startState [][]uint8s) [][]uint8 {

	// Channel to receive new state output from workers
	newFrames := make(chan [][]uint8)

	for turn = 0; turn < turns; turns++ {
		// Start calculation of next frame
		go calculateNewState(p, c, world, turn, newFrames)
		// Await reception from channels
		nextFrame := <-newFrames
		world = nextFrame

	}

	return world

}

// RPC Requests

type Gol struct {}

func (g *Gol) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {
	res.State = engine(req.Turns, req.CurrentState)
	return

	
}


// Server Handling
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
