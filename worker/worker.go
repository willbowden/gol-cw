package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
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
func worker(y1, y2 int, world [][]uint8, p stubs.Params) []util.Cell {
	var flippedCells []util.Cell
	for y := y1; y <= y2; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			neighbours := getNumNeighbours(y, x, world, p)
			cellValue := world[y][x]
			switch {
			// <2 neighbours, cell dies
			case neighbours < 2 && cellValue == 255:
				flippedCells = append(flippedCells, util.Cell{X: x, Y: y - y1})
			// >3 neighbours, live cell dies
			case neighbours > 3 && cellValue == 255:
				flippedCells = append(flippedCells, util.Cell{X: x, Y: y - y1})
			// exactly 3 neighbours, dead cell comes alive
			case neighbours == 3 && cellValue == 0:
				flippedCells = append(flippedCells, util.Cell{X: x, Y: y - y1})
			}
		}
	}

	return flippedCells
}

// Add rpc function(s)

type Worker struct {
	wg       sync.WaitGroup
	listener net.Listener
	signal   chan string
	quitting bool
}

func (w *Worker) ProcessSlice(req stubs.Request, res *stubs.Response) (err error) {
	w.wg.Add(1)
	defer w.wg.Done()
	flippedCells := worker(req.Y1, req.Y2, req.CurrentState, req.Params)
	res.FlippedCells = flippedCells
	return
}

func (w *Worker) KillWorker(req stubs.Request, res *stubs.Response) (err error) {
	w.quitting = true
	w.signal <- "KILL"
	return
}

// Ensure clients have closed connections (and therefore have received responses) before closing server
func (w *Worker) serveConn(conn net.Conn) {
	w.wg.Add(1)
	defer w.wg.Done()
	rpc.ServeConn(conn)
}

func (w *Worker) startAccepting(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		// If error is caused by us having intentionally closed the server, return
		if err != nil {
			if w.quitting {
				return
			} else {
				fmt.Println("Accept error:", err)
			}
		} else {
			go w.serveConn(conn)
		}
	}
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	w := Worker{listener: listener, signal: make(chan string, 1)}
	rpc.Register(&w)
	fmt.Println("Server open on port", *pAddr)
	defer listener.Close()
	go w.startAccepting(listener)
	<-w.signal
	fmt.Println("Server closing...")
	w.wg.Wait()
	close(w.signal)
}
