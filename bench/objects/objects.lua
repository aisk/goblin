-- User-defined types: construction, method calls, attribute reads and writes,
-- and an overloaded operator through a metatable.
local Vec = {}
Vec.__index = Vec

function Vec.new(x, y)
    return setmetatable({x = x, y = y}, Vec)
end

function Vec.__add(a, b)
    return Vec.new(a.x + b.x, a.y + b.y)
end

function Vec:norm()
    return self.x * self.x + self.y * self.y
end

local Particle = {}
Particle.__index = Particle

function Particle.new(pos, vel)
    return setmetatable({pos = pos, vel = vel}, Particle)
end

function Particle:step()
    self.pos = self.pos + self.vel
end

local particles = {}
local i = 0
while i < 400 do
    particles[#particles + 1] = Particle.new(Vec.new(i, i + 1), Vec.new(1, 2))
    i = i + 1
end

local total = 0
local t = 0
while t < 4500 do
    local j = 1
    while j <= #particles do
        local p = particles[j]
        p:step()
        total = total + p.pos:norm()
        j = j + 1
    end
    t = t + 1
end
print(total)
