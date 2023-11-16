package stubs

var ProcessTurns = "Gol.ProcessTurns"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Response struct {
	State [][]uint8
}

type Request struct {
	Params       Params
	CurrentState [][]uint8
}
