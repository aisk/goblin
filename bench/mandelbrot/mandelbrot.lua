-- Mandelbrot set rendered as ASCII: a float-heavy inner loop.
local width = 160
local height = 80
local max_iter = 1000

for y = 0, height - 1 do
    local ci = y * 2.0 / height - 1.0
    local line = {}
    for x = 0, width - 1 do
        local cr = x * 3.0 / width - 2.0
        local zr = 0.0
        local zi = 0.0
        local iter = 0
        while iter < max_iter and zr * zr + zi * zi <= 4.0 do
            local tmp = zr * zr - zi * zi + cr
            zi = 2.0 * zr * zi + ci
            zr = tmp
            iter = iter + 1
        end
        if iter == max_iter then
            line[#line + 1] = "*"
        else
            line[#line + 1] = " "
        end
    end
    print(table.concat(line))
end
