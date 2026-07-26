// Mandelbrot set rendered as ASCII: a float-heavy inner loop.
package main

import (
	"bufio"
	"os"
)

func main() {
	const width = 160
	const height = 80
	const maxIter = 1000

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	line := make([]byte, 0, width)
	for y := 0; y < height; y++ {
		ci := float64(y)*2.0/height - 1.0
		line = line[:0]
		for x := 0; x < width; x++ {
			cr := float64(x)*3.0/width - 2.0
			zr := 0.0
			zi := 0.0
			iter := 0
			for iter < maxIter && zr*zr+zi*zi <= 4.0 {
				tmp := zr*zr - zi*zi + cr
				zi = 2.0*zr*zi + ci
				zr = tmp
				iter++
			}
			if iter == maxIter {
				line = append(line, '*')
			} else {
				line = append(line, ' ')
			}
		}
		out.Write(line)
		out.WriteByte('\n')
	}
}
