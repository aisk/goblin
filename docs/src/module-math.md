# math

The math module holds Float functions, constants and basic statistics. Its
members are plain functions: `x` is an Int or a Float everywhere, and results
are Floats unless a row says otherwise.

## Common tasks

~~~goblin
import "math"

# Geometry
var area = math.PI * math.pow(radius, 2)
var distance = math.sqrt(math.pow(x2 - x1, 2) + math.pow(y2 - y1, 2))
var heading = math.atan2(dy, dx) * 180 / math.PI
print("heading {heading:.1f} degrees, {distance:,.0f} m away")

# Whole numbers from measurements
var pages = math.ceil(items.size / 20.0)
var percent = math.round(100.0 * done / total)

# Two decimal places: in the value, or only in the text
var price = math.round(total * 100) / 100.0
print("total: {total:.2f}")

# Statistics of a set of measurements
var timings = [0.82, 0.79, 1.31, 0.80, 0.85]
print("mean {math.mean(timings):.2f} s, median {math.median(timings):.2f} s")
print("spread {math.stdev(timings):.2f} s, 95th percentile {math.percentile(timings, 95):.2f} s")
print("total {sum(timings):.1f} s, slowest {max(timings):.2f} s")
var ages = people.map(func(p) { return p["age"] })
var typical_age = math.round(math.median(ages))

# Orders of magnitude
var digits = math.floor(math.log(n, base=10)) + 1
~~~

`docs/examples/math-tasks.goblin` runs these tasks; `examples/math.goblin`
covers the rest of the module.

## Members

| Member | Returns | Behaviour |
| --- | --- | --- |
| `PI`, `E` | Float | Constants |
| `INF` | Float | Positive infinity; `-math.INF` is the negative one |
| `NAN` | Float | Not-a-number. It equals nothing, itself included: test with `is_nan` |
| `sqrt(x)` | Float | Square root; `NAN` for a negative `x` |
| `pow(x, y)` | Float | `x` to the power `y`, also for two Ints (the language has no power operator; an exact Int power is repeated multiplication) |
| `exp(x)` | Float | `E` to the power `x` |
| `log(x, base=nil)` | Float | Logarithm; `base=nil` is the natural logarithm. `log(8, base=2)` and `log(1000, base=10)` are exact. `-INF` for 0, `NAN` for a negative `x`. ValueError unless `base` is positive and not 1 |
| `sin(x)`, `cos(x)`, `tan(x)` | Float | `x` in radians |
| `asin(x)`, `acos(x)`, `atan(x)` | Float | Radians; `NAN` outside the domain |
| `atan2(y, x)` | Float | The angle of the point `(x, y)`, −π to π |
| `abs(x)` | Int for an Int, Float for a Float | Magnitude. OverflowError for the smallest Int |
| `floor(x)` | **Int** | Largest whole number ≤ `x` |
| `ceil(x)` | **Int** | Smallest whole number ≥ `x` |
| `round(x)` | **Int** | Nearest whole number, halves away from zero (`round(2.5)` is 3, `round(-2.5)` is −3) |
| `is_nan(x)` | Bool | Whether `x` is `NAN` |

`floor`, `ceil` and `round` return an Int because that is what the result is
used as: an index, a count. An Int argument is returned as it is. `NAN` or an
infinity is a ValueError (`floor() cannot convert NaN`), and a result that
does not fit 64 bits an OverflowError. Truncation toward zero is `Int(x)`.

Every function raises TypeError for a non-number:
`sqrt() argument 'x' must be Int or Float, got String`. Domain errors follow
IEEE 754: they produce `NAN` or an infinity and do not raise.

## Statistics

Four functions over a set of measurements. `values` is any iterable of Ints
and Floats (a List, a `range`, the values of a Dict); it is read once and not
changed.

| Member | Returns | Behaviour |
| --- | --- | --- |
| `mean(values)` | Float | The arithmetic mean |
| `median(values)` | Float | The middle value in sorted order; for an even count, the mean of the two middle values |
| `stdev(values)` | Float | The **sample** standard deviation (divides by n − 1): how far a measurement typically lies from the mean |
| `percentile(values, p)` | Float | The value below which `p` percent of the values fall, `p` being an Int or Float from 0 to 100. Between two values it interpolates linearly, as spreadsheet `PERCENTILE` and NumPy do by default. `p=0` is the smallest value, `p=100` the largest, `p=50` the median |

- **The result is always a Float**, also for Ints and also when it is whole
  (`math.mean([1, 2, 3])` is `2.0`). `math.round(…)` makes an Int.
- **Empty input raises** ValueError (`mean() argument 'values' is empty`;
  `stdev() needs at least two values, got 1`). There is no mean of nothing,
  and `0.0` or `NAN` would travel silently into a report: test `values.size`
  first when empty is expected.
- An element that is not an Int or Float is a TypeError
  (`mean() argument 'values' must contain only Int or Float, got String`); a
  Bool is not a number. A `NAN` among the values gives `NAN` from `mean` and
  `stdev`, whose arithmetic carries it through, and is a ValueError from
  `median` and `percentile` (`median() cannot order NaN`): they sort the
  values, and a NaN has no place in an order, as with `sort()`.
- A `p` outside 0–100 is a ValueError.
- The population deviation is `math.stdev(xs) * math.sqrt((n - 1.0) / n)`,
  and the variance `math.pow(math.stdev(xs), 2)`.

Sums and extremes are the globals `sum`, `max` and `min`, which work on any
values that add or compare, not only numbers.

## Formatting and powers

Printing a number a certain way is not this module's job: format specifiers
do it (`"{x:.2f}"`, `"{n:,}"`, `format(x, spec)`; see
[Strings](./strings.md)). Rounding a value to decimals is
`math.round(x * 100) / 100.0`; showing two decimals is `{x:.2f}`. They are
different operations, and the first is rare.

## Left out

`round(x, digits)` (format specifiers); a power operator (deferred in the
language, so `pow` stays); `trunc` (`Int(x)`); `log2`/`log10`
(`log(x, base=2)`); `cbrt` (`math.pow(x, 1.0 / 3)`); `hypot`
(`math.sqrt(x * x + y * y)`); `is_inf` (`x == math.INF`); the hyperbolic
functions; `max`/`min` (the globals); variance, the population deviation,
mode and correlation.
