// Functions as values: closures, composition, and list helpers that call back
// into a function for every element.
package main

import "fmt"

func compose(f, g func(int) int) func(int) int {
	return func(x int) int { return f(g(x)) }
}

func mapSlice(fn func(int) int, values []int) []int {
	out := []int{}
	k := 0
	for k < len(values) {
		out = append(out, fn(values[k]))
		k = k + 1
	}
	return out
}

func filterSlice(fn func(int) bool, values []int) []int {
	out := []int{}
	k := 0
	for k < len(values) {
		if fn(values[k]) {
			out = append(out, values[k])
		}
		k = k + 1
	}
	return out
}

func reduceSlice(fn func(int, int) int, values []int, initial int) int {
	acc := initial
	k := 0
	for k < len(values) {
		acc = fn(acc, values[k])
		k = k + 1
	}
	return acc
}

func main() {
	double := func(x int) int { return x * 2 }
	increment := func(x int) int { return x + 1 }
	step := compose(increment, double)

	xs := []int{}
	i := 0
	for i < 2000 {
		xs = append(xs, i)
		i = i + 1
	}

	total := 0
	round := 0
	for round < 1600 {
		mapped := mapSlice(step, xs)
		kept := filterSlice(func(x int) bool { return x%3 == 0 }, mapped)
		total = total + reduceSlice(func(acc, x int) int { return acc + x }, kept, 0)
		round = round + 1
	}
	fmt.Println(total)
}
