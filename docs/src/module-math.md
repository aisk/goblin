# math

The math module provides constants and floating-point functions. It is useful
when the basic arithmetic operators are not enough.

~~~goblin
import "math"

var radius = 3
var area = math.pi * math.pow(radius, 2)
print(area)
print(math.sqrt(81))
~~~

Common functions include abs(), ceil(), floor(), round(), trunc(), pow(),
sqrt(), log(), exp(), sin(), cos(), tan(), min(), and max(). Constants include
pi, e, inf, and nan.

~~~goblin
print(math.floor(3.8))
print(math.hypot(3, 4))
print(math.is_nan(math.nan))
~~~

The module accepts integers and floats where a numeric input is expected.
Functions that fundamentally produce fractional results return Float.
