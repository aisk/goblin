// Towers of Hanoi: deep recursion with three arguments, moves are counted
// instead of printed so the output stays small.
let moves = 0;

function hanoi(n, from, to, via) {
  if (n === 0) {
    return;
  }
  hanoi(n - 1, from, via, to);
  moves++;
  hanoi(n - 1, via, to, from);
}

hanoi(21, 1, 3, 2);
console.log(moves);
