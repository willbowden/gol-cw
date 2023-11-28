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

func handlePause(c distributorChannels, client *rpc.Client) {
	req := new(stubs.Request)
	response := new(stubs.Response)
	client.Call(stubs.PauseBroker, req, response)
	if response.Paused {
		c.events <- StateChange{CompletedTurns: response.CurrentTurn, NewState: Paused}
	} else {
		c.events <- StateChange{CompletedTurns: response.CurrentTurn, NewState: Executing}
		fmt.Println("Continuing")
	}
}

func startGOL(client *rpc.Client, world [][]uint8, p Params, ch chan stubs.Response) {

	if p.Turns == 0 {
		response := stubs.Response{CurrentTurn: 0, State: world}
		ch <- response
	}

	for turn := 0; turn < p.Turns; turn++ {
		var request stubs.Request
		if turn == 0 {
			request = stubs.Request{CurrentState: world, Params: stubs.Params(p)}
		} else {
			request = stubs.Request{Params: stubs.Params(p)}
		}
		response := new(stubs.Response)
		client.Call(stubs.ProcessTurn, request, response)
		ch <- *response
	}

}

// distributor acts as the local controller
func distributor(p Params, c distributorChannels) {
	flag_server := "44.201.201.88:8030"
	client, _ := rpc.Dial("tcp", flag_server)
	defer client.Close()

	ch_response := make(chan stubs.Response)

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
			if cell == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}

	ticker := time.NewTicker(2000 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker.C:
				request := stubs.Request{CurrentState: world, Params: stubs.Params(p)}
				response := new(stubs.Response)
				client.Call(stubs.AliveCellsCount, request, response)
				c.events <- AliveCellsCount{CompletedTurns: response.CurrentTurn, CellsCount: response.CellCount}
			}
		}
	}()

	quit := false

	go startGOL(client, world, p, ch_response)

	for quit == false {
		select {
		case res := <-ch_response:
			// If we're receiving no state, we're just receiving flipped cells
			if res.State == nil {
				// Flip all the cells on the GUI that have been flipped in the world
				c.events <- TurnComplete{CompletedTurns: res.CurrentTurn}
				for _, cell := range res.FlippedCells {
					c.events <- CellFlipped{CompletedTurns: res.CurrentTurn, Cell: cell}
				}
				// If we're receiving state and we haven't finished all turns, we're resuming from a client-side quit
			} else if res.State != nil && res.CurrentTurn != p.Turns {
				fmt.Println("Picking up from turn ", res.CurrentTurn)
				// So we need to make sure the GUI reflects the current state, rather than from the loaded image
				for row := range res.State {
					for col := range res.State[row] {
						if res.State[row][col] != world[row][col] {
							c.events <- CellFlipped{CompletedTurns: res.CurrentTurn, Cell: util.Cell{X: col, Y: row}}
						}
					}
				}
				// Otherwise we're receiving a new state because execution has finished
			} else {
				// So quit, and write the final image
				ticker.Stop()
				quit = true
				c.events <- FinalTurnComplete{CompletedTurns: res.CurrentTurn, Alive: calculateAliveCells(p, res.State)}
				c.events <- StateChange{res.CurrentTurn, Quitting}
				writeImage(p, c, res.State, res.CurrentTurn)
			}
			// If we receive a keypress
		case key := <-c.keyPresses:
			switch key {
			// q: quit, change state to Quitting
			case 'q':
				ticker.Stop()
				quit = true
				req := new(stubs.Request)
				response := new(stubs.Response)
				client.Call(stubs.ClientQuit, req, response)
				writeImage(p, c, response.State, response.CurrentTurn)
				c.events <- StateChange{CompletedTurns: response.CurrentTurn, NewState: Quitting}
			// s: screenshot, output current world as PGM image
			case 's':
				req := new(stubs.Request)
				response := new(stubs.Response)
				client.Call(stubs.Screenshot, req, response)
				writeImage(p, c, response.State, response.CurrentTurn)
			//p: pause, change state to Paused
			case 'p':
				// Running pause logic in goroutine allows other keypresses to work while paused
				go handlePause(c, client)
			case 'k':
				ticker.Stop()
				quit = true
				req := new(stubs.Request)
				response := new(stubs.Response)
				client.Call(stubs.KillBroker, req, response)
				client.Close()
				writeImage(p, c, response.State, response.CurrentTurn)
				c.events <- StateChange{CompletedTurns: response.CurrentTurn, NewState: Quitting}
			}
		}
	}

	// Execute all turns of the Game of Life.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
