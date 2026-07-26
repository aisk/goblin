// Dense matrix multiplication: nested loops over indexed 2-D slices.
package main

import "fmt"

func main() {
	const n = 240

	a := make([][]int, n)
	b := make([][]int, n)
	for i := 0; i < n; i++ {
		a[i] = make([]int, n)
		b[i] = make([]int, n)
		for j := 0; j < n; j++ {
			a[i][j] = i + j
			b[i][j] = i - j
		}
	}

	total := 0
	for i := 0; i < n; i++ {
		ai := a[i]
		for j := 0; j < n; j++ {
			sum := 0
			for k := 0; k < n; k++ {
				sum += ai[k] * b[k][j]
			}
			total += sum
		}
	}

	fmt.Println(total)
}
