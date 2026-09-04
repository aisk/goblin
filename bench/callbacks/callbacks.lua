-- Functions as values: closures, composition, and list helpers that call back
-- into a function for every element.
local function compose(f, g)
    return function(x) return f(g(x)) end
end

local double = function(x) return x * 2 end
local increment = function(x) return x + 1 end
local step = compose(increment, double)

local xs = {}
local i = 0
while i < 2000 do
    xs[#xs + 1] = i
    i = i + 1
end

local function map(fn, values)
    local out = {}
    local k = 1
    while k <= #values do
        out[k] = fn(values[k])
        k = k + 1
    end
    return out
end

local function filter(fn, values)
    local out = {}
    local k = 1
    while k <= #values do
        if fn(values[k]) then out[#out + 1] = values[k] end
        k = k + 1
    end
    return out
end

local function reduce(fn, values, initial)
    local acc = initial
    local k = 1
    while k <= #values do
        acc = fn(acc, values[k])
        k = k + 1
    end
    return acc
end

local total = 0
local round = 0
while round < 1600 do
    local mapped = map(step, xs)
    local kept = filter(function(x) return x % 3 == 0 end, mapped)
    total = total + reduce(function(acc, x) return acc + x end, kept, 0)
    round = round + 1
end
print(total)
