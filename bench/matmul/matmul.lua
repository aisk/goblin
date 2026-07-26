-- Dense matrix multiplication: nested loops over indexed 2-D tables.
local n = 240

local a = {}
local b = {}
for i = 0, n - 1 do
    local ra = {}
    local rb = {}
    for j = 0, n - 1 do
        ra[j] = i + j
        rb[j] = i - j
    end
    a[i] = ra
    b[i] = rb
end

local total = 0
for i = 0, n - 1 do
    local ai = a[i]
    for j = 0, n - 1 do
        local sum = 0
        for k = 0, n - 1 do
            sum = sum + ai[k] * b[k][j]
        end
        total = total + sum
    end
end

print(total)
