-- Word frequency counting: string split, dict updates, and a sort with a key.
local vocab = {"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta"}

local words = {}
local v = 1
while v <= 7 do
    local n = 0
    while n < v do
        words[#words + 1] = vocab[v]
        n = n + 1
    end
    v = v + 1
end
local line = table.concat(words, " ")

local function split(s, sep)
    local parts = {}
    local start = 1
    while true do
        local at = string.find(s, sep, start, true)
        if at == nil then
            parts[#parts + 1] = string.sub(s, start)
            return parts
        end
        parts[#parts + 1] = string.sub(s, start, at - 1)
        start = at + #sep
    end
end

local counts = {}
local round = 0
while round < 150000 do
    local parts = split(line, " ")
    local i = 1
    while i <= #parts do
        local w = parts[i]
        counts[w] = (counts[w] or 0) + 1
        i = i + 1
    end
    round = round + 1
end

local items = {}
for word, count in pairs(counts) do
    items[#items + 1] = {word, count}
end
table.sort(items, function(a, b) return a[2] > b[2] end)
local j = 1
while j <= #items do
    print(items[j][1] .. " " .. items[j][2])
    j = j + 1
end
