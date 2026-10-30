// Package emit holds one of every kind of value type kanna-value generates.
// Its value_gen.go is the golden file of the emit test, and the test calls what
// it declares.
package emit

import (
	"time"

	"github.com/go-kanna/value/rule"
)

var earliest = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// CompletedAt holds a time.Time, whose own IsZero, Equal, and String the
// generated methods call.
//
//kanna:value
type CompletedAt struct{ t time.Time }

func (CompletedAt) validate(t time.Time) error {
	if t.Before(earliest) {
		return rule.FailWith("greater_than_or_equal_to", map[string]any{"count": earliest})
	}
	return nil
}

// Email holds a string, whose accessor is its String method.
//
//kanna:value
type Email struct{ s string }

func (Email) validate(s string) error { return rule.Email(s) }

// Count holds an int, which fmt formats.
//
//kanna:value
type Count struct{ n int }

func (Count) validate(n int) error {
	if n < 0 {
		return rule.FailWith("greater_than_or_equal_to", map[string]any{"count": 0})
	}
	return nil
}

type cents int64

// price is unexported, and so are its constructors.
//
//kanna:value
type price struct{ c cents }

func (price) validate(cents) error { return nil }

type tags []string

// Tags holds a slice, which the generated code copies.
//
//kanna:value
type Tags struct{ t tags }

func (Tags) validate(t tags) error {
	if len(t) > 3 {
		return rule.FailWith("too_long", map[string]any{"count": 3})
	}
	return nil
}

type labels map[string]string

// Labels holds a map, which the generated code copies.
//
//kanna:value
type Labels struct{ l labels }

func (Labels) validate(labels) error { return nil }

type address struct{ lines []string }

// Address holds a struct that can be told zero neither by == nor by an IsZero
// method, so it gets no IsZero.
//
//kanna:value
type Address struct{ a address }

func (Address) validate(address) error { return nil }

// Status is an enum.
//
//kanna:enum
type Status string

const (
	StatusPlaced  Status = "placed"
	StatusPaid    Status = "paid"
	StatusShipped Status = "shipped"
)

// kind is unexported, and so are its functions.
//
//kanna:enum
type kind string

const kindA kind = "a"
