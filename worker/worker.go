package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// calculate number of neighbours around a cell at given coords, wrapping around world edges
func getNumNeighbours(y, x int, world [][]uint8, p stubs.Params) int {
	numNeighbours := 0
	// Look 1 to left, right, above and below the chosen cell
	for yInc := -1; yInc <= 1; yInc++ {
		var testY int = (y + yInc + p.ImageHeight) % p.ImageHeight
		for xInc := -1; xInc <= 1; xInc++ {
			var testX int = (x + xInc + p.ImageWidth) % p.ImageWidth
			if (testX != x || testY != y) && world[testY][testX] == 255 {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}

// worker() calculates the next state of the world within its given y bounds, and returns the new chunk via a channel
func worker(y1, y2 int, world [][]uint8, p stubs.Params) [][]uint8 {
	sliceHeight := (y2 - y1) + 1
	var newSlice = make([][]uint8, sliceHeight)
	for i := 0; i < sliceHeight; i++ {
		newSlice[i] = make([]uint8, p.ImageWidth)
	}
	for y := y1; y <= y2; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			neighbours := getNumNeighbours(y, x, world, p)
			cellValue := world[y][x]
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

	return newSlice
}

// Add rpc function(s)

type Worker struct {
	listener net.Listener
	signal   chan os.Signal
}

func (w *Worker) ProcessSlice(req stubs.Request, res *stubs.Response) (err error) {
	newSlice := worker(req.Y1, req.Y2, req.CurrentState, req.Params)
	res.State = newSlice
	return
}

func (w *Worker) KillWorker(req stubs.Request, res *stubs.Response) (err error) {
	signal.Notify(w.signal, syscall.SIGINT, syscall.SIGTERM)
	return
}

func startAccepting(listener net.Listener) {
	rpc.Accept(listener)
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	w := Worker{listener: listener, signal: make(chan os.Signal, 1)}
	rpc.Register(&w)
	fmt.Println("Server open on port", *pAddr)
	defer listener.Close()
	go startAccepting(listener)
	<-w.signal
	fmt.Println("Server closing...")
}
