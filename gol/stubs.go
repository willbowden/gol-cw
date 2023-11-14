package gol

var ProcessTurns = "Gol.ProcessTurns"

type Response struct {
	State [][]uint8
}

type Request struct {
	Params       Params
	CurrentState [][]uint8
}
