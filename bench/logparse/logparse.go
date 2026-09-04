// Log line parsing: string building, trimming, splitting, prefix tests and
// integer conversion, with the results counted in a map.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

func main() {
	lines := []string{}
	i := 0
	for i < 4000 {
		lines = append(lines, "  level=info user=u"+strconv.Itoa(i%97)+" ms="+strconv.Itoa(i%31)+"  ")
		i = i + 1
	}

	users := map[string]int{}
	elapsed := 0
	round := 0
	for round < 280 {
		j := 0
		for j < len(lines) {
			fields := strings.Split(strings.TrimSpace(lines[j]), " ")
			k := 0
			for k < len(fields) {
				field := fields[k]
				if strings.HasPrefix(field, "user=") {
					user := strings.TrimPrefix(field, "user=")
					users[user] = users[user] + 1
				}
				if strings.HasPrefix(field, "ms=") {
					value, err := strconv.Atoi(strings.TrimPrefix(field, "ms="))
					if err != nil {
						panic(err)
					}
					elapsed = elapsed + value
				}
				k = k + 1
			}
			j = j + 1
		}
		round = round + 1
	}
	fmt.Println(len(users), elapsed)
}
