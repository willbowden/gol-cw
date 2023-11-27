package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

// Each benchmark has 1000 iterations
const benchLength = 1000

func BenchmarkGol(b *testing.B) {
	// 1 to 16 threads / workers
	for threads := 1; threads <= 16; threads++ {

		// Disable GOL output
		os.Stdout = nil
		p := gol.Params{
			// benchLength = each benchmark
			Turns:       benchLength,
			Threads:     threads,
			ImageWidth:  512,
			ImageHeight: 512,
		}
		// Unique name for each benchmark
		name := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)

		b.Run(name, func(b *testing.B) {
			// Running for pre-set number of times (b.N)
			for i := 0; i < b.N; i++ { // Running GOL each time
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events { // Consumes events until done
				}

			}
		})
	}
}
