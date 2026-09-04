// Word frequency counting: string split, map updates, and a sort with a key.
package main

import (
	"fmt"
	"sort"
	"strings"
)

func main() {
	vocab := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta"}

	words := []string{}
	v := 0
	for v < 7 {
		n := 0
		for n <= v {
			words = append(words, vocab[v])
			n = n + 1
		}
		v = v + 1
	}
	line := strings.Join(words, " ")

	counts := map[string]int{}
	round := 0
	for round < 150000 {
		parts := strings.Split(line, " ")
		i := 0
		for i < len(parts) {
			w := parts[i]
			counts[w] = counts[w] + 1
			i = i + 1
		}
		round = round + 1
	}

	type item struct {
		word  string
		count int
	}
	items := []item{}
	for word, count := range counts {
		items = append(items, item{word, count})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].count > items[j].count })
	j := 0
	for j < len(items) {
		fmt.Println(items[j].word, items[j].count)
		j = j + 1
	}
}
