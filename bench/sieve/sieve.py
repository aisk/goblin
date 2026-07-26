# Sieve of Eratosthenes: tight loops over a large flat array.
limit = 4000000

flags = [True] * (limit + 1)

count = 0
for p in range(2, limit + 1):
    if flags[p]:
        count += 1
        m = p * p
        while m <= limit:
            flags[m] = False
            m += p

print(count)
