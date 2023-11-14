package gol

import (
	"fmt"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
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

// distributor acts as the local controller
func distributor(p Params, c distributorChannels) {
	// server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	// flag.Parse()
	// fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", "127.0.0.1:8030")
	defer client.Close()

	// Create a 2D slice to store the world.
	world := make([][]uint8, p.ImageHeight)
	for y := range world {
		world[y] = make([]uint8, p.ImageWidth)
	}

	// Start IO image reading
	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	// Read each cell into our world from IO
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := <-c.ioInput
			world[y][x] = cell
		}
	}

	// Execute all turns of the Game of Life.

	request := Request{Params: p, CurrentState: world}
	response := new(Response)

	client.Call(ProcessTurns, request, response)

	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, response.State)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
