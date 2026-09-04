// User-defined types: construction, method calls, field reads and writes, and
// an Add method standing in for Goblin's overloaded operator.
package main

import "fmt"

type Vec struct {
	x, y int
}

func (v *Vec) Add(other *Vec) *Vec {
	return &Vec{v.x + other.x, v.y + other.y}
}

func (v *Vec) Norm() int {
	return v.x*v.x + v.y*v.y
}

type Particle struct {
	pos, vel *Vec
}

func (p *Particle) Step() {
	p.pos = p.pos.Add(p.vel)
}

func main() {
	particles := []*Particle{}
	i := 0
	for i < 400 {
		particles = append(particles, &Particle{&Vec{i, i + 1}, &Vec{1, 2}})
		i = i + 1
	}

	total := 0
	t := 0
	for t < 4500 {
		j := 0
		for j < len(particles) {
			p := particles[j]
			p.Step()
			total = total + p.pos.Norm()
			j = j + 1
		}
		t = t + 1
	}
	fmt.Println(total)
}
