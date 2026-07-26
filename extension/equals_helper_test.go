package extension

import "github.com/aisk/goblin/object"

// objectEquals answers the tests' equality checks. Comparing two built-in
// values never fails, so an error here can only be a bug in the test setup.
func objectEquals(a, b object.Object) bool {
	eq, err := object.Equals(a, b)
	if err != nil {
		panic(err)
	}
	return eq
}
