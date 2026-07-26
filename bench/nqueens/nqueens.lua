-- N-Queens: recursive backtracking with an O(row) safety check.
local n = 11
local cols = {}
for i = 0, n - 1 do
    cols[i] = 0
end

local function safe(cols, row, col)
    for i = 0, row - 1 do
        local c = cols[i]
        if c == col then
            return false
        end
        if c - i == col - row then
            return false
        end
        if c + i == col + row then
            return false
        end
    end
    return true
end

local function solve(cols, row, n)
    if row == n then
        return 1
    end
    local count = 0
    for col = 0, n - 1 do
        if safe(cols, row, col) then
            cols[row] = col
            count = count + solve(cols, row + 1, n)
        end
    end
    return count
end

print(solve(cols, 0, n))
