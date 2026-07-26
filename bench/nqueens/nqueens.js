// N-Queens: recursive backtracking with an O(row) safety check.
const n = 11;
const cols = new Array(n).fill(0);

function safe(cols, row, col) {
  for (let i = 0; i < row; i++) {
    const c = cols[i];
    if (c === col) {
      return false;
    }
    if (c - i === col - row) {
      return false;
    }
    if (c + i === col + row) {
      return false;
    }
  }
  return true;
}

function solve(cols, row, n) {
  if (row === n) {
    return 1;
  }
  let count = 0;
  for (let col = 0; col < n; col++) {
    if (safe(cols, row, col)) {
      cols[row] = col;
      count += solve(cols, row + 1, n);
    }
  }
  return count;
}

console.log(solve(cols, 0, n));
