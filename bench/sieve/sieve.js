// Sieve of Eratosthenes: tight loops over a large flat array.
const limit = 4000000;

const flags = new Array(limit + 1).fill(true);

let count = 0;
for (let p = 2; p <= limit; p++) {
  if (flags[p]) {
    count++;
    for (let m = p * p; m <= limit; m += p) {
      flags[m] = false;
    }
  }
}

console.log(count);
