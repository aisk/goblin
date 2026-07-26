# Mandelbrot set rendered as ASCII: a float-heavy inner loop.
width = 160
height = 80
max_iter = 1000

for y in range(height):
    ci = y * 2.0 / height - 1.0
    line = []
    for x in range(width):
        cr = x * 3.0 / width - 2.0
        zr = 0.0
        zi = 0.0
        iter = 0
        while iter < max_iter and zr * zr + zi * zi <= 4.0:
            tmp = zr * zr - zi * zi + cr
            zi = 2.0 * zr * zi + ci
            zr = tmp
            iter += 1
        line.append("*" if iter == max_iter else " ")
    print("".join(line))
