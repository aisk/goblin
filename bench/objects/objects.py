# User-defined types: construction, method calls, attribute reads and writes,
# and an overloaded operator.
class Vec:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __add__(self, other):
        return Vec(self.x + other.x, self.y + other.y)

    def norm(self):
        return self.x * self.x + self.y * self.y


class Particle:
    def __init__(self, pos, vel):
        self.pos = pos
        self.vel = vel

    def step(self):
        self.pos = self.pos + self.vel


particles = []
i = 0
while i < 400:
    particles.append(Particle(Vec(i, i + 1), Vec(1, 2)))
    i = i + 1

total = 0
t = 0
while t < 4500:
    j = 0
    while j < len(particles):
        p = particles[j]
        p.step()
        total = total + p.pos.norm()
        j = j + 1
    t = t + 1
print(total)
