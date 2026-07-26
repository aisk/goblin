-- Towers of Hanoi: deep recursion with three arguments, moves are counted
-- instead of printed so the output stays small.
local moves = 0

local function hanoi(n, from, to, via)
    if n == 0 then
        return
    end
    hanoi(n - 1, from, via, to)
    moves = moves + 1
    hanoi(n - 1, via, to, from)
end

hanoi(21, 1, 3, 2)
print(moves)
