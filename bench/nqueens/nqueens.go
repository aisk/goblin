// N-Queens: recursive backtracking with an O(row) safety check.
package main

import "fmt"

func safe(cols []int, row, col int) bool {
	for i := 0; i < row; i++ {
		c := cols[i]
		if c == col {
			return false
		}
		if c-i == col-row {
			return false
		}
		if c+i == col+row {
			return false
		}
	}
	return true
}

func solve(cols []int, row, n int) int {
	if row == n {
		return 1
	}
	count := 0
	for col := 0; col < n; col++ {
		if safe(cols, row, col) {
			cols[row] = col
			count += solve(cols, row+1, n)
		}
	}
	return count
}

func main() {
	const n = 11
	cols := make([]int, n)
	fmt.Println(solve(cols, 0, n))
}
