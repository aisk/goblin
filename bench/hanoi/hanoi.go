// Towers of Hanoi: deep recursion with three arguments, moves are counted
// instead of printed so the output stays small.
package main

import "fmt"

var moves int

func hanoi(n, from, to, via int) {
	if n == 0 {
		return
	}
	hanoi(n-1, from, via, to)
	moves++
	hanoi(n-1, via, to, from)
}

func main() {
	hanoi(21, 1, 3, 2)
	fmt.Println(moves)
}
