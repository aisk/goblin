// Log line parsing: string building, trimming, splitting, prefix tests and
// integer conversion, with the results counted in a map.
const lines = [];
let i = 0;
while (i < 4000) {
    lines.push("  level=info user=u" + String(i % 97) + " ms=" + String(i % 31) + "  ");
    i = i + 1;
}

const users = new Map();
let elapsed = 0;
let round = 0;
while (round < 280) {
    let j = 0;
    while (j < lines.length) {
        const fields = lines[j].trim().split(" ");
        let k = 0;
        while (k < fields.length) {
            const field = fields[k];
            if (field.startsWith("user=")) {
                const user = field.slice("user=".length);
                users.set(user, (users.get(user) ?? 0) + 1);
            }
            if (field.startsWith("ms=")) {
                elapsed = elapsed + parseInt(field.slice("ms=".length), 10);
            }
            k = k + 1;
        }
        j = j + 1;
    }
    round = round + 1;
}
console.log(users.size, elapsed);
