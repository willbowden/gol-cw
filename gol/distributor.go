package gol

import (
	"fmt"

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

func makeImmutableWorld(world [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

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
func distributor(p Params, c distributorChannels) {

	c.events <- StateChange{CompletedTurns: 0, NewState: Executing}

	var world = make([][]byte, p.ImageHeight)
	for col := range world {
		world[col] = make([]byte, p.ImageWidth)
	}
	// Start IO image reading
	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename
	// Read each cell into our world
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := <-c.ioInput
			world[y][x] = cell
		}
	}

	turn := 0
	for turn = 0; turn < p.Turns; turn++ {
		// Make new 2D array for the next frame
		var newFrame [][]uint8
		workerChannel := make(chan [][]uint8)
		immutableWorld := makeImmutableWorld(world)
		for i := 1; i <= p.Threads; i++ {
			// Divide up the world and send to our workers
			y1 := (i - 1) * (p.ImageHeight / p.Threads)
			y2 := i*(p.ImageHeight/p.Threads) - 1
			go worker(y1, y2, immutableWorld, c.events, workerChannel, p, turn)
		}
		for j := 0; j < p.Threads; j++ {
			// Retrieve the new slices from the workers & append into new frame
			newSlice := <-workerChannel
			newFrame = append(newFrame, newSlice...)
		}
		// Overwrite the world with our new frame
		for col := range newFrame {
			copy(newFrame[col], world[col])
		}

		// c.events <- TurnComplete{CompletedTurns: turn}
	}

	liveCells := calculateAliveCells(p, world)
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: liveCells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
