// Sieve of Eratosthenes: tight loops over a large flat array.
package main

import "fmt"

func main() {
	const limit = 4000000

	flags := make([]bool, limit+1)
	for i := 0; i <= limit; i++ {
		flags[i] = true
	}

	count := 0
	for p := 2; p <= limit; p++ {
		if flags[p] {
			count++
			for m := p * p; m <= limit; m += p {
				flags[m] = false
			}
		}
	}

	fmt.Println(count)
}
