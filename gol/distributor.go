package gol

import (
	"fmt"
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

// Returns a function (like a getter) allowing us to access data without risk of overwriting
func makeImmutableWorld(world [][]uint8) func(y, x int) uint8 {
	return func(y, x int) uint8 {
		return world[y][x]
	}
}

// Find all alive cells and return a list of live cells
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

// calculateNewState divides the image up by thread count, then reassembles the workers slices
func calculateNewState(p Params, c distributorChannels, world [][]uint8, turn int, ch chan<- [][]uint8) {
	// Make new 2D array for the next frame
	var newFrame [][]uint8

	// Make an output channel for each worker thread
	channels := make([]chan [][]uint8, p.Threads)
	for v := range channels {
		channels[v] = make(chan [][]uint8)
	}

	immutableWorld := makeImmutableWorld(world)

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
		go worker(y1, y2, immutableWorld, c.events, channel, p, turn)

	}

	// Receive new world data from workers and reassemble in correct order
	for _, channel := range channels {
		// Retrieve the new slices from the workers & append into new frame
		newSlice := <-channel
		newFrame = append(newFrame, newSlice...)
	}

	// Send complete new frame to distributor
	ch <- newFrame
}

func writeImage(c distributorChannels, filename string, world [][]uint8, p Params, turn int) {
	// Start IO output
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

// Pause Keypress
func handlePause(c distributorChannels, turn int) {
	// Continuously wait for a 'p' keypress before returning from the function.
	select {
	case nextKey := <-c.keyPresses:
		if nextKey == 'p' {
			// Set state to Executing
			c.events <- StateChange{CompletedTurns: turn, NewState: Executing}
			return
		}
	}
}

// Distributor takes in our world, and for each turn, calls a goroutine onto the calculate new state function, it also manages IO features like keypresses
func distributor(p Params, c distributorChannels) {

	c.events <- StateChange{CompletedTurns: 0, NewState: Executing}

	// Create 2D array for world
	var world = make([][]byte, p.ImageHeight)
	for col := range world {
		world[col] = make([]byte, p.ImageWidth)
	}

	// Start IO image reading
	filename := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	// Read each cell into our world from IO
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := <-c.ioInput
			if cell == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
			world[y][x] = cell
		}
	}

	turn := 0

	// Start a ticker to output the number of live cells every 2 seconds
	ticker := time.NewTicker(2000 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.events <- AliveCellsCount{turn, len(calculateAliveCells(p, world))}
			}
		}
	}()

	// Format filename for PGM image outputs
	outFilename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)

	// Channel to receive new state output from workers
	newFrames := make(chan [][]uint8)
	quit := false

	// Added waitgroup to ensure no race conditions when calculateNewState being called

	for turn = 0; turn < p.Turns && !quit; turn++ {
		// Start calculation of next frame
		go calculateNewState(p, c, world, turn, newFrames)
		// Await reception from channels
		select {
		// If next frame is finished, update world & send turn complete event
		case nextFrame := <-newFrames:
			world = nextFrame
			c.events <- TurnComplete{CompletedTurns: turn}
		// If we receive a keypress
		case key := <-c.keyPresses:
			switch key {
			// q: quit, change state to Quitting
			case 'q':
				quit = true
				c.events <- StateChange{CompletedTurns: turn, NewState: Quitting}
			// s: screenshot, output current world as PGM image
			case 's':
				writeImage(c, outFilename, world, p, turn)
			// p: pause, change state to Paused and await handlePause()
			case 'p':
				c.events <- StateChange{CompletedTurns: turn, NewState: Paused}

				// handlePause halts execution until the user presses 'p' for a second time.
				handlePause(c, turn)
				fmt.Println("Continuing")
			}
		}
	}

	// Stop ticker and output final state of world as image
	ticker.Stop()
	writeImage(c, outFilename, world, p, turn)

	liveCells := calculateAliveCells(p, world)
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: liveCells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
	close(newFrames)

}
