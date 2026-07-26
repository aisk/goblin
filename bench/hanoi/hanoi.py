# Towers of Hanoi: deep recursion with three arguments, moves are counted
# instead of printed so the output stays small.
moves = 0


def hanoi(n, frm, to, via):
    global moves
    if n == 0:
        return
    hanoi(n - 1, frm, via, to)
    moves += 1
    hanoi(n - 1, via, to, frm)


hanoi(21, 1, 3, 2)
print(moves)
