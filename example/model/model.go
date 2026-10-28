// Package model declares the values a task service deals in. kanna-value
// turns each struct marked //kanna:value into a type, in value_gen.go, that
// only ever holds what its validate method accepts.
package model

import (
	"strings"
	"time"

	"github.com/go-kanna/value/rule"
)

//go:generate go tool kanna-value

// Email is the address the owner of a task is reached at.
//
//kanna:value
type Email struct{ s string }

// Domain returns the part after the @, such as "example.com". A value type is
// a type of its own, so it takes methods like any other.
func (e Email) Domain() string {
	_, domain, _ := strings.Cut(e.s, "@")
	return domain
}

func (Email) validate(s string) error { return rule.Email(s) }

// opened is the day the service began; nothing was completed before it.
var opened = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// CompletedAt is when a task was done.
//
//kanna:value
type CompletedAt struct{ t time.Time }

func (CompletedAt) validate(t time.Time) error {
	if t.Before(opened) {
		return rule.FailWith("greater_than_or_equal_to", map[string]any{"count": opened})
	}
	return nil
}

// tags is what Tags holds. A value type holds a named type, after which the
// method returning it is named.
type tags []string

// Tags label a task, three at most. The generated code copies the slice on
// the way in and out, so that nothing changes it past the rule.
//
//kanna:value
type Tags struct{ t tags }

func (Tags) validate(t tags) error {
	if len(t) > 3 {
		return rule.FailWith("too_long", map[string]any{"count": 3})
	}
	return nil
}

// Task is a thing to be done, decoded from a request as it stands: each value
// checks itself while it is decoded.
type Task struct {
	Title       string       `json:"title"`
	Owner       Email        `json:"owner"`
	Tags        Tags         `json:"tags,omitzero"`
	CompletedAt *CompletedAt `json:"completed_at,omitempty"`
}
