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

## Rounding and bounds

abs(), ceil(), floor(), round(), and trunc() are useful when converting a
calculated value into a display or storage value. min() and max() choose among
numeric arguments and are often used to clamp a value.

~~~goblin
var requested = 1.8
var whole = math.ceil(requested)
var bounded = math.min(math.max(whole, 1), 4)
print(bounded)
~~~

## Geometry and special values

pow(), sqrt(), cbrt(), and hypot() cover common geometric calculations. The
trigonometric functions use radians. is_nan() and is_inf() help detect special
floating-point values before they affect later calculations.

~~~goblin
var distance = math.hypot(3, 4)
print(distance) # 5
print(math.sin(math.pi / 2))
print(math.is_inf(math.inf))
~~~

Some domain operations can produce nan instead of a conventional value. Check
with is_nan() before serializing or displaying the result when inputs may be
outside the mathematical domain.
