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

func calculateNewState(p Params, c distributorChannels, world [][]uint8, turn int, ch chan<- [][]uint8) {
	// Make new 2D array for the next frame
	var newFrame [][]uint8
	channels := make([]chan [][]uint8, p.Threads)
	for v := range channels {
		channels[v] = make(chan [][]uint8)
	}
	immutableWorld := makeImmutableWorld(world)
	sliceSize := p.ImageHeight / p.Threads
	remainder := p.ImageHeight % p.Threads

	for i, channel := range channels {
		i += 1
		y1 := (i - 1) * sliceSize
		y2 := (i * sliceSize) - 1
		if i == p.Threads {
			y2 += remainder
		}
		go worker(y1, y2, immutableWorld, c.events, channel, p, turn)

	}

	for _, channel := range channels {
		// Retrieve the new slices from the workers & append into new frame
		newSlice := <-channel
		newFrame = append(newFrame, newSlice...)
	}

	ch <- newFrame
}

func writeImage(c distributorChannels, filename string, world [][]uint8, p Params, turn int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
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
	ticker := time.NewTicker(2000 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.events <- AliveCellsCount{turn, len(calculateAliveCells(p, world))}
			}
		}
	}()

	outFilename := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	newFrames := make(chan [][]uint8)
	quit := false
	for turn = 0; turn < p.Turns && !quit; turn++ {
		go calculateNewState(p, c, world, turn, newFrames)
	next:
		for !quit {
			select {
			case nextFrame := <-newFrames:
				world = nextFrame
				c.events <- TurnComplete{CompletedTurns: turn}
				break next
			case key := <-c.keyPresses:
				switch key {
				case 'q':
					quit = true
					c.events <- StateChange{CompletedTurns: turn, NewState: Quitting}
					break
				case 's':
					writeImage(c, outFilename, world, p, turn)
				case 'p':
					c.events <- StateChange{CompletedTurns: turn, NewState: Paused}
					for {
						select {
						case nextKey := <-c.keyPresses:
							if nextKey == 'p' {
								c.events <- StateChange{CompletedTurns: turn, NewState: Executing}
								break next
							}
						}
					}
				}
			}
		}
	}

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
}
