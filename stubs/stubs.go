package stubs

var ProcessTurns = "Gol.ProcessTurns"
var AliveCellsCount = "Gol.AliveCellsCount"

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

type Request struct {
	Params       Params
	CurrentState [][]uint8
	y1, y2       int
}
