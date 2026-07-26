// Dense matrix multiplication: nested loops over indexed 2-D arrays.
const n = 240;

const a = [];
const b = [];
for (let i = 0; i < n; i++) {
  const ra = new Array(n);
  const rb = new Array(n);
  for (let j = 0; j < n; j++) {
    ra[j] = i + j;
    rb[j] = i - j;
  }
  a.push(ra);
  b.push(rb);
}

let total = 0;
for (let i = 0; i < n; i++) {
  const ai = a[i];
  for (let j = 0; j < n; j++) {
    let sum = 0;
    for (let k = 0; k < n; k++) {
      sum += ai[k] * b[k][j];
    }
    total += sum;
  }
}

console.log(total);
