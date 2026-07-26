// Mandelbrot set rendered as ASCII: a float-heavy inner loop.
const width = 160;
const height = 80;
const maxIter = 1000;

const out = [];
for (let y = 0; y < height; y++) {
  const ci = (y * 2.0) / height - 1.0;
  let line = "";
  for (let x = 0; x < width; x++) {
    const cr = (x * 3.0) / width - 2.0;
    let zr = 0.0;
    let zi = 0.0;
    let iter = 0;
    while (iter < maxIter && zr * zr + zi * zi <= 4.0) {
      const tmp = zr * zr - zi * zi + cr;
      zi = 2.0 * zr * zi + ci;
      zr = tmp;
      iter++;
    }
    line += iter === maxIter ? "*" : " ";
  }
  out.push(line);
}
console.log(out.join("\n"));
