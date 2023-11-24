package stubs

var ProcessTurns = "Gol.ProcessTurns"
var AliveCellsCount = "Gol.AliveCellsCount"
var ProcessSlice = "Worker.ProcessSlice"
var ClientQuit = "Gol.ClientQuit"
var Screenshot = "Gol.Screenshot"
var PauseBroker = "Gol.PauseBroker"
var KillWorker = "Worker.KillWorker"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	State       [][]uint8
	CurrentTurn int
	Paused      bool
}

type CellCount struct {
	Turn       int
	CellsCount int
}

type Request struct {
	Params       Params
	CurrentState [][]uint8
	Y1, Y2       int
}
