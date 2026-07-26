-- Sieve of Eratosthenes: tight loops over a large flat array.
local limit = 4000000

local flags = {}
for i = 0, limit do
    flags[i] = true
end

local count = 0
for p = 2, limit do
    if flags[p] then
        count = count + 1
        local m = p * p
        while m <= limit do
            flags[m] = false
            m = m + p
        end
    end
end

print(count)
