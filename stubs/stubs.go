package stubs

import "uk.ac.bris.cs/gameoflife/util"

var ProcessTurn = "Gol.ProcessTurn"
var AliveCellsCount = "Gol.AliveCellsCount"
var ProcessSlice = "Worker.ProcessSlice"
var ClientQuit = "Gol.ClientQuit"
var Screenshot = "Gol.Screenshot"
var PauseBroker = "Gol.PauseBroker"
var KillBroker = "Gol.KillBroker"
var KillWorker = "Worker.KillWorker"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	State        [][]uint8
	FlippedCells []util.Cell
	CurrentTurn  int
	CellCount    int
	Paused       bool
}

type Request struct {
	Params       Params
	CurrentState [][]uint8
	Y1, Y2       int
}
