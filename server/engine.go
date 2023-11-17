package main

import (
	//"errors"

	"flag"
	"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
	//	"fmt"

	"net"
)

// GOL Logic as in Parallel Implementation

// calculate number of neighbours around a cell at given coords, wrapping around world edges
func getNumNeighbours(y, x int, world func(y, x int) uint8, p stubs.Params) int {
	numNeighbours := 0
	// Look 1 to left, right, above and below the chosen cell
	for yInc := -1; yInc <= 1; yInc++ {
		var testY int = (y + yInc + p.ImageHeight) % p.ImageHeight
		for xInc := -1; xInc <= 1; xInc++ {
			var testX int = (x + xInc + p.ImageWidth) % p.ImageWidth
			if (testX != x || testY != y) && world(testY, testX) == 255 {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}

// worker() calculates the next state of the world within its given y bounds, and returns the new chunk via a channel
func worker(y1, y2 int, world func(y, x int) uint8, c chan<- [][]uint8, p stubs.Params, turn int) {
	sliceHeight := y2
	var newSlice = make([][]uint8, sliceHeight)
	for i := 0; i < sliceHeight; i++ {
		newSlice[i] = make([]uint8, p.ImageWidth)
	}
	for y := y1; y <= y2; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			neighbours := getNumNeighbours(y, x, world, p)
			cellValue := world(y, x)
			switch {
			// <2 neighbours, cell dies
			case neighbours < 2:
				newSlice[y-y1][x] = 0
			// >3 neighbours, live cell dies
			case neighbours > 3 && cellValue == 255:
				newSlice[y-y1][x] = 0
			// exactly 3 neighbours, dead cell comes alive
			case neighbours == 3 && cellValue == 0:
				newSlice[y-y1][x] = 255
			// otherwise send current cell value to new state
			default:
				newSlice[y-y1][x] = cellValue
			}
		}
	}
	// send new world slice down output channel
	c <- newSlice
}

func calculateNewState(p stubs.Params, world [][]uint8, turn int, ch chan<- [][]uint8) {
	// Make new 2D array for the next frame
	var newFrame [][]uint8
	immutableWorld := makeImmutableWorld(world)

	ch_out := make(chan [][]uint8)

	go worker(0, p.ImageHeight, immutableWorld, ch_out, p, turn)

	newSlice := <-ch_out
	newFrame = append(newFrame, newSlice...)

	// Wait for the worker goroutine to finish
	ch <- newFrame

	close(ch_out)

	// Send complete new frame to distributor
}

// Returns a function allowing us to access data without risk of overwriting
func makeImmutableWorld(world [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func engine(p stubs.Params, state [][]uint8) [][]uint8 {

	// Channel to receive new state output from workers
	newFrames := make(chan [][]uint8)

	for turn := 0; turn <= p.Turns; turn++ {
		go calculateNewState(p, state, turn, newFrames)
	}

	for i := 1; i < p.Turns; i++ {
		<-newFrames
	}
	close(newFrames)

	return state

}

// RPC Requests

type Gol struct{}

// calculate new state
func (g *Gol) ProcessTurns(req stubs.Request, res *stubs.Response) (err error) {
	// get new state : set for response state
	res.State = engine(req.Params, req.CurrentState)
	return

}

// Server Handling
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Gol{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	fmt.Println("Server open on port", *pAddr)
	defer listener.Close()
	// boilerplate for registering type Gol -> we can use Gol methods
	rpc.Accept(listener)
}
