package gol

import (
	"fmt"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
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

func writeImage(p Params, c distributorChannels, world [][]uint8, turn int) {
	// Start IO output
	filename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
	c.ioCommand <- ioOutput
	c.ioFilename <- filename

	// Send our world data, pixel by pixel
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Send IO complete event to notify user
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

// distributor acts as the local controller
func distributor(p Params, c distributorChannels) {
	flag_server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", flag_server)
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

	ticker := time.NewTicker(2000 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				request := stubs.Request{CurrentState: world, Params: stubs.Params(p)}
				response := new(stubs.CellCount)
				client.Call(stubs.AliveCellsCount, request, response)
				c.events <- AliveCellsCount{CompletedTurns: response.Turn, CellsCount: response.CellsCount}
			}
		}
	}()

	// Execute all turns of the Game of Life.

	request := stubs.Request{CurrentState: world, Params: stubs.Params(p)}
	response := new(stubs.Response)
	client.Call(stubs.ProcessTurns, request, response)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, response.State)}

	writeImage(p, c, response.State, p.Turns)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	ticker.Stop()
	close(c.events)
}
