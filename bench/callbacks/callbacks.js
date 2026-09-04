// Functions as values: closures, composition, and list methods that call back
// into a function for every element.
function compose(f, g) {
    return (x) => f(g(x));
}

const double = (x) => x * 2;
const increment = (x) => x + 1;
const step = compose(increment, double);

const xs = [];
let i = 0;
while (i < 2000) {
    xs.push(i);
    i = i + 1;
}

let total = 0;
let round = 0;
while (round < 1600) {
    const mapped = xs.map(step);
    const kept = mapped.filter((x) => x % 3 === 0);
    total = total + kept.reduce((acc, x) => acc + x, 0);
    round = round + 1;
}
console.log(total);
