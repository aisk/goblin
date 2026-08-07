package object

// Protocol dunder-method names. A user-defined type customizes built-in
// behavior (operators, comparison, conversion, iteration, indexing) by
// defining a method with one of these names. The semantic checker and both
// backends reference these constants, so the spelling cannot drift.
const (
	ProtoAdd     = "__add"
	ProtoSub     = "__sub"
	ProtoMul     = "__mul"
	ProtoDiv     = "__div"
	ProtoMod     = "__mod"
	ProtoRAdd    = "__radd"
	ProtoRSub    = "__rsub"
	ProtoRMul    = "__rmul"
	ProtoRDiv    = "__rdiv"
	ProtoRMod    = "__rmod"
	ProtoCmp     = "__cmp"
	ProtoNot     = "__not"
	ProtoStr     = "__str"
	ProtoBool    = "__bool"
	ProtoIter    = "__iter"
	ProtoGetItem = "__getitem"
	ProtoSetItem = "__setitem"
)

// ProtocolArity is the authoritative list of protocol methods, mapping each
// name to the exact number of parameters it must declare, including the
// leading self. The semantic checker validates arities against it.
var ProtocolArity = map[string]int{
	ProtoAdd: 2, ProtoSub: 2, ProtoMul: 2, ProtoDiv: 2, ProtoMod: 2,
	ProtoRAdd: 2, ProtoRSub: 2, ProtoRMul: 2, ProtoRDiv: 2, ProtoRMod: 2,
	ProtoCmp: 2, ProtoGetItem: 2,
	ProtoNot: 1, ProtoStr: 1, ProtoBool: 1, ProtoIter: 1,
	ProtoSetItem: 3,
}

// Fallback diagnostics raised when a user type does not define the protocol
// backing an operation. Both backends format them with the type's declared
// name, so the error reads identically whichever backend runs the program.
const (
	ErrFmtCannotAdd        = "cannot add %s"
	ErrFmtCannotSubtract   = "cannot subtract %s"
	ErrFmtCannotMultiply   = "cannot multiply %s"
	ErrFmtCannotDivide     = "cannot divide %s"
	ErrFmtCannotModulo     = "cannot modulo %s"
	ErrFmtCannotCompare    = "cannot compare %s"
	ErrFmtNotIterable      = "%s does not support iteration"
	ErrFmtNotIndexable     = "%s is not indexable"
	ErrFmtCmpMustReturnInt = "%s.__cmp must return Int, got %s"
)
