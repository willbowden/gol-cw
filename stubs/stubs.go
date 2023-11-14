package stubs

var ProcessTurns = "Gol.ProcessTurns"

type Response struct {
	State [][]uint8
}

type Request struct {
	Turns int
}
