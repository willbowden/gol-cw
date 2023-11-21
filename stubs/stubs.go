package stubs

var ProcessTurns = "Gol.ProcessTurns"
var AliveCellsCount = "Gol.AliveCellsCount"
var ProcessSlice = "Worker.ProcessSlice"
var PingWorker = "Worker.Ping"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	State [][]uint8
}

type CellCount struct {
	Turn       int
	CellsCount int
}

type TestResponse struct {
	Message string
}

type Request struct {
	Params       Params
	CurrentState [][]uint8
	Y1, Y2       int
}
