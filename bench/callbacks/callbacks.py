# Functions as values: closures, composition, and list helpers that call back
# into a function for every element.
def compose(f, g):
    return lambda x: f(g(x))


double = lambda x: x * 2
increment = lambda x: x + 1
step = compose(increment, double)

xs = []
i = 0
while i < 2000:
    xs.append(i)
    i = i + 1


def map_(fn, values):
    out = []
    k = 0
    while k < len(values):
        out.append(fn(values[k]))
        k = k + 1
    return out


def filter_(fn, values):
    out = []
    k = 0
    while k < len(values):
        if fn(values[k]):
            out.append(values[k])
        k = k + 1
    return out


def reduce_(fn, values, initial):
    acc = initial
    k = 0
    while k < len(values):
        acc = fn(acc, values[k])
        k = k + 1
    return acc


total = 0
round_ = 0
while round_ < 1600:
    mapped = map_(step, xs)
    kept = filter_(lambda x: x % 3 == 0, mapped)
    total = total + reduce_(lambda acc, x: acc + x, kept, 0)
    round_ = round_ + 1
print(total)
