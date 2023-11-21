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
	keyPresses <-chan rune
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

func startGOL(client *rpc.Client, world [][]uint8, p Params) [][]uint8 {

	request := stubs.Request{CurrentState: world, Params: stubs.Params(p)}
	response := new(stubs.Response)
	client.Call(stubs.ProcessTurns, request, response)

	return response.State

}

// distributor acts as the local controller
func distributor(p Params, c distributorChannels) {
	flag_server := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", flag_server)
	defer client.Close()

	ch_state := make(chan [][]uint8)

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

	quit := false

	go startGOL(client, world, p)

	for quit == false {
		select {
		// If we receive a keypress
		case state := <-ch_state:
			quit = true
			world = state
			c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, state)}
		case key := <-c.keyPresses:
			switch key {
			// q: quit, change state to Quitting
			case 'q':
				ticker.Stop()
				quit = true
				fmt.Println("RECEIVED KEYPRESS")
				req := new(stubs.Request)
				response := new(stubs.Response)
				client.Call(stubs.QuitBroker, req, response)
				writeImage(p, c, world, response.CurrentTurn)
				c.events <- StateChange{CompletedTurns: response.CurrentTurn, NewState: Quitting}
				// s: screenshot, output current world as PGM image
				//case 's':
				//writeImage(p, c, world, response.CurrentTurn)
				// p: pause, change state to Paused and await handlePause()
				// case 'p':
				// 	c.events <- StateChange{CompletedTurns: response.CurrentTurn, NewState: Paused}
				// 	handlePause halts execution until the user presses 'p' for a second time.
				// 	handlePause(c, response.CurrentTurn)
				// 	fmt.Println("Continuing")
			}
		}
	}

	// Execute all turns of the Game of Life.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
