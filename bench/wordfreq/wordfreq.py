# Word frequency counting: string split, dict updates, and a sort with a key.
vocab = ["alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta"]

words = []
v = 0
while v < 7:
    n = 0
    while n <= v:
        words.append(vocab[v])
        n = n + 1
    v = v + 1
line = " ".join(words)

counts = {}
round_ = 0
while round_ < 150000:
    parts = line.split(" ")
    i = 0
    while i < len(parts):
        w = parts[i]
        counts[w] = counts.get(w, 0) + 1
        i = i + 1
    round_ = round_ + 1

items = list(counts.items())
items.sort(key=lambda item: item[1], reverse=True)
j = 0
while j < len(items):
    print(items[j][0], items[j][1])
    j = j + 1
