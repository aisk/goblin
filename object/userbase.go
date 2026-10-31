package object

// UserBase implements the Object methods every user value shares: printing,
// truth, hashing, comparison, operators, iteration and indexing, all
// dispatched to the value's trait impls. The interpreter's instances and the
// structs the transpiler generates embed it and add only what differs
// between them: field storage, attribute access and method calls. A
// generated struct may still override an operator to call its impl
// directly.
//
// A UserBase must be wired to the value embedding it, which NewUser does.
type UserBase struct {
	self UserValue
}

// userBase lets NewUser reach the embedded base. Being unexported, it can only
// come from embedding UserBase.
func (b *UserBase) userBase() *UserBase { return b }

// NewUser wires a freshly allocated user value to its embedded UserBase and
// returns it.
func NewUser[V interface {
	UserValue
	userBase() *UserBase
}](v V) V {
	v.userBase().self = v
	return v
}

func (b *UserBase) String() string            { return userString(b.self) }
func (b *UserBase) ToString() (string, error) { return userToString(b.self) }
func (b *UserBase) ToBool() (bool, error)     { return userToBool(b.self) }
func (b *UserBase) Hash() (uint64, error)     { return userHash(b.self) }

func (b *UserBase) Equals(other Object) (bool, error) { return userEquals(b.self, other) }
func (b *UserBase) Compare(other Object) (int, error) { return userCompare(b.self, other) }

func (b *UserBase) Add(other Object) (Object, error) { return userArith(b.self, ArithAdd, other) }
func (b *UserBase) Minus(other Object) (Object, error) {
	return userArith(b.self, ArithSub, other)
}
func (b *UserBase) Multiply(other Object) (Object, error) {
	return userArith(b.self, ArithMul, other)
}
func (b *UserBase) Divide(other Object) (Object, error) {
	return userArith(b.self, ArithDiv, other)
}
func (b *UserBase) Modulo(other Object) (Object, error) {
	return userArith(b.self, ArithMod, other)
}

func (b *UserBase) RAdd(left Object) (Object, bool, error) {
	return userReflected(b.self, ArithAdd, left)
}
func (b *UserBase) RMinus(left Object) (Object, bool, error) {
	return userReflected(b.self, ArithSub, left)
}
func (b *UserBase) RMultiply(left Object) (Object, bool, error) {
	return userReflected(b.self, ArithMul, left)
}
func (b *UserBase) RDivide(left Object) (Object, bool, error) {
	return userReflected(b.self, ArithDiv, left)
}
func (b *UserBase) RModulo(left Object) (Object, bool, error) {
	return userReflected(b.self, ArithMod, left)
}

func (b *UserBase) Iter() ([]Object, error)            { return userIter(b.self) }
func (b *UserBase) Index(index Object) (Object, error) { return userIndex(b.self, index) }
func (b *UserBase) SetIndex(index, value Object) (bool, error) {
	return userSetIndex(b.self, index, value)
}
