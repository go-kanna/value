package rule

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Error reports a value that a rule rejected.
//
// The code names the rule and the params carry what a message needs, such as
// the limit a value exceeded. The rejected value itself is left out: errors end
// up in logs, and the value may be personal data.
//
// Error is used through a pointer. The params make the struct incomparable, so
// comparing two Error values with == would panic; pointers compare by identity.
type Error struct {
	code   string
	params map[string]any
}

// Fail returns an Error with the given code. A validate method returns it to
// reject a value.
func Fail(code string) *Error {
	return &Error{code: code}
}

// FailWith is Fail with parameters for the message. params is copied, so the
// caller may go on to modify it.
func FailWith(code string, params map[string]any) *Error {
	return &Error{code: code, params: maps.Clone(params)}
}

// Code returns the code naming the rule that rejected the value.
func (e *Error) Code() string {
	return e.code
}

// Params returns a copy of the parameters.
func (e *Error) Params() map[string]any {
	return maps.Clone(e.params)
}

// Error formats the code followed by the parameters in key order, such as
// "rule: range (max=130, min=18)".
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("rule: ")
	b.WriteString(e.code)
	if len(e.params) == 0 {
		return b.String()
	}

	b.WriteString(" (")
	for i, k := range slices.Sorted(maps.Keys(e.params)) {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=%v", k, e.params[k])
	}
	b.WriteByte(')')
	return b.String()
}
