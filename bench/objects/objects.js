// User-defined types: construction, method calls, attribute reads and writes,
// and an add method standing in for Goblin's overloaded operator.
class Vec {
    constructor(x, y) {
        this.x = x;
        this.y = y;
    }

    add(other) {
        return new Vec(this.x + other.x, this.y + other.y);
    }

    norm() {
        return this.x * this.x + this.y * this.y;
    }
}

class Particle {
    constructor(pos, vel) {
        this.pos = pos;
        this.vel = vel;
    }

    step() {
        this.pos = this.pos.add(this.vel);
    }
}

const particles = [];
let i = 0;
while (i < 400) {
    particles.push(new Particle(new Vec(i, i + 1), new Vec(1, 2)));
    i = i + 1;
}

let total = 0;
let t = 0;
while (t < 4500) {
    let j = 0;
    while (j < particles.length) {
        const p = particles[j];
        p.step();
        total = total + p.pos.norm();
        j = j + 1;
    }
    t = t + 1;
}
console.log(total);
