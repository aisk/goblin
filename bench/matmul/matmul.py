# Dense matrix multiplication: nested loops over indexed 2-D lists.
n = 240

a = [[i + j for j in range(n)] for i in range(n)]
b = [[i - j for j in range(n)] for i in range(n)]

total = 0
for i in range(n):
    ai = a[i]
    for j in range(n):
        s = 0
        for k in range(n):
            s += ai[k] * b[k][j]
        total += s

print(total)
