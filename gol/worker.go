package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

// calculate number of neighbours around a cell at given coords, wrapping around world edges using modulus function
func getNumNeighbours(y, x int, world func(y, x int) uint8, p Params) int {
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

// Sends change in cell value to events to alert and render on the output
func setCell(y, x int, world func(y, x int) uint8, newValue uint8, events chan<- Event, turn int) {
	if world(y, x) != newValue {
		events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
	}
}

// Calculates the next state of the world within its given y bounds, and returns the new chunk via a channel
func worker(y1, y2 int, world func(y, x int) uint8, events chan<- Event, c chan<- [][]uint8, p Params, turn int) {

	sliceHeight := (y2 - y1) + 1
	var newSlice = make([][]uint8, sliceHeight)

	// Create empty new slice
	for i := 0; i < sliceHeight; i++ {
		newSlice[i] = make([]uint8, p.ImageWidth)
	}

	// Iterate each cell
	for y := y1; y <= y2; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			neighbours := getNumNeighbours(y, x, world, p)
			cellValue := world(y, x)
			switch {
			// <2 neighbours, cell dies
			case neighbours < 2:
				setCell(y, x, world, 0, events, turn)
				newSlice[y-y1][x] = 0
			// >3 neighbours, live cell dies
			case neighbours > 3 && cellValue == 255:
				setCell(y, x, world, 0, events, turn)
				newSlice[y-y1][x] = 0
			// exactly 3 neighbours, dead cell comes alive
			case neighbours == 3 && cellValue == 0:
				setCell(y, x, world, 255, events, turn)
				newSlice[y-y1][x] = 255
			// otherwise send current cell value to new state
			default:
				newSlice[y-y1][x] = cellValue
			}
		}
	}
	// Send new world slice down output channel
	c <- newSlice
}
