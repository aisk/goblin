# N-Queens: recursive backtracking with an O(row) safety check.
n = 11
cols = [0] * n


def safe(cols, row, col):
    for i in range(row):
        c = cols[i]
        if c == col:
            return False
        if c - i == col - row:
            return False
        if c + i == col + row:
            return False
    return True


def solve(cols, row, n):
    if row == n:
        return 1
    count = 0
    for col in range(n):
        if safe(cols, row, col):
            cols[row] = col
            count += solve(cols, row + 1, n)
    return count


print(solve(cols, 0, n))
