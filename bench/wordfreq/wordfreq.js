// Word frequency counting: string split, dict updates, and a sort with a key.
const vocab = ["alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta"];

const words = [];
let v = 0;
while (v < 7) {
    let n = 0;
    while (n <= v) {
        words.push(vocab[v]);
        n = n + 1;
    }
    v = v + 1;
}
const line = words.join(" ");

const counts = new Map();
let round = 0;
while (round < 150000) {
    const parts = line.split(" ");
    let i = 0;
    while (i < parts.length) {
        const w = parts[i];
        counts.set(w, (counts.get(w) ?? 0) + 1);
        i = i + 1;
    }
    round = round + 1;
}

const items = Array.from(counts.entries());
items.sort((a, b) => b[1] - a[1]);
let j = 0;
while (j < items.length) {
    console.log(items[j][0], items[j][1]);
    j = j + 1;
}
