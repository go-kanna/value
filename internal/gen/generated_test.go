package gen_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-kanna/value/internal/gen/testdata/emit"
	"github.com/go-kanna/value/rule"
)

// The tests below call what testdata/emit/value_gen.go declares.

var (
	valid   = time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	invalid = time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC)
)

// wantCode fails t unless err is a *rule.Error coded code.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()

	var rerr *rule.Error
	if !errors.As(err, &rerr) || rerr.Code() != code {
		t.Errorf("err = %v, want code %q", err, code)
	}
}

func TestGenerated_New(t *testing.T) {
	t.Parallel()

	at, err := emit.NewCompletedAt(valid)
	if err != nil || !at.Time().Equal(valid) {
		t.Errorf("NewCompletedAt(%v) = %v, %v; want it held", valid, at, err)
	}

	at, err = emit.NewCompletedAt(invalid)
	wantCode(t, err, "greater_than_or_equal_to")
	if !at.IsZero() {
		t.Errorf("NewCompletedAt(%v) = %v, want the zero CompletedAt", invalid, at)
	}
}

func TestGenerated_Must(t *testing.T) {
	t.Parallel()

	if got := emit.MustEmail("a@example.com").String(); got != "a@example.com" {
		t.Errorf("MustEmail(...).String() = %q, want %q", got, "a@example.com")
	}

	defer func() {
		err, _ := recover().(error)
		wantCode(t, err, "email")
	}()
	emit.MustEmail("not an address")
}

func TestGenerated_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    fmt.Stringer
		want string
	}{
		{name: "the value's own String", v: emit.MustCompletedAt(valid), want: valid.String()},
		{name: "a string held", v: emit.MustEmail("a@example.com"), want: "a@example.com"},
		{name: "formatted by fmt", v: emit.MustCount(3), want: "3"},
		{name: "a slice formatted by fmt", v: emit.MustTags([]string{"go"}), want: "[go]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.v.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := fmt.Sprint(tt.v); got != tt.want {
				t.Errorf("fmt.Sprint = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerated_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    interface{ IsZero() bool }
		want bool
	}{
		{name: "zero, told by the value's own IsZero", v: emit.CompletedAt{}, want: true},
		{name: "given, told by the value's own IsZero", v: emit.MustCompletedAt(valid), want: false},
		{name: "zero, compared", v: emit.Count{}, want: true},
		{name: "given zero, compared", v: emit.MustCount(0), want: true},
		{name: "given, compared", v: emit.MustCount(3), want: false},
		{name: "zero slice", v: emit.Tags{}, want: true},
		{name: "empty slice, which is not nil", v: emit.MustTags([]string{}), want: false},
		{name: "zero map", v: emit.Labels{}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.v.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %t, want %t", got, tt.want)
			}
		})
	}

	if _, ok := any(emit.Address{}).(interface{ IsZero() bool }); ok {
		t.Error("Address has an IsZero method, want none: its value can be told zero neither by == nor by IsZero")
	}
}

func TestGenerated_Equal(t *testing.T) {
	t.Parallel()

	// Read back from a database, a time has lost its monotonic clock reading
	// and may be in another location.
	now := time.Now()
	a, b := emit.MustCompletedAt(now), emit.MustCompletedAt(now.Round(0).UTC())
	if a == b {
		t.Fatal("a == b, want the times to differ in their clock and location for the test to mean anything")
	}
	if !a.Equal(b) {
		t.Error("a.Equal(b) = false, want true for the same instant")
	}
	if a.Equal(emit.MustCompletedAt(valid)) {
		t.Error("a.Equal(...) = true for another instant, want false")
	}
}

func TestGenerated_Copies(t *testing.T) {
	t.Parallel()

	in := []string{"go"}
	tags := emit.MustTags(in)
	in[0] = "changed after New"
	out := tags.Tags()
	out[0] = "changed through the accessor"
	if got := tags.Tags()[0]; got != "go" {
		t.Errorf("Tags()[0] = %q, want %q: the value was changed from outside", got, "go")
	}

	m := map[string]string{"k": "v"}
	labels := emit.MustLabels(m)
	m["k"] = "changed after New"
	labels.Labels()["k"] = "changed through the accessor"
	if got := labels.Labels()["k"]; got != "v" {
		t.Errorf(`Labels()["k"] = %q, want %q: the value was changed from outside`, got, "v")
	}
}

// body holds value types as a request body would.
type body struct {
	Done  *emit.CompletedAt `json:"done"`
	Email emit.Email        `json:"email,omitzero"`
	Tags  emit.Tags         `json:"tags,omitzero"`
}

func TestGenerated_JSON(t *testing.T) {
	t.Parallel()

	at := emit.MustCompletedAt(valid)
	b, err := json.Marshal(body{Done: &at, Tags: emit.MustTags([]string{"go"})})
	if want := `{"done":"2026-10-29T09:00:00Z","tags":["go"]}`; err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v; want %s", b, err, want)
	}

	var got body
	if err := json.Unmarshal(b, &got); err != nil || got.Done == nil || !got.Done.Equal(at) {
		t.Errorf("Unmarshal(%s) = %+v, %v; want the time back", b, got, err)
	}

	tests := []struct {
		name string
		body string
		code string // of the rule's error, or "" for encoding/json's own
	}{
		{name: "rejected by the rule", body: `{"email":"not an address"}`, code: "email"},
		{name: "rejected by the rule, through a pointer", body: `{"done":"1999-12-31T00:00:00Z"}`,
			code: "greater_than_or_equal_to"},
		{name: "not the type held", body: `{"email":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kept := emit.MustEmail("kept@example.com")
			got := body{Email: kept}
			err := json.Unmarshal([]byte(tt.body), &got)
			if tt.code != "" {
				wantCode(t, err, tt.code)
			} else if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
				t.Errorf("err = %v, want encoding/json's *UnmarshalTypeError", err)
			}
			if got.Email != kept {
				t.Errorf("Email = %v after an error, want it left as %v", got.Email, kept)
			}
		})
	}

	kept := emit.MustEmail("kept@example.com")
	if err := kept.UnmarshalJSON([]byte("null")); err != nil || kept.String() != "kept@example.com" {
		t.Errorf("UnmarshalJSON(null) = %v, leaving %v; want nil, leaving it as it was", err, kept)
	}
}

func TestGenerated_SQL(t *testing.T) {
	t.Parallel()

	if v, err := emit.MustCount(3).Value(); err != nil || v != int64(3) {
		t.Errorf("Value() = %#v, %v; want int64(3), as database/sql converts an int", v, err)
	}

	var at emit.CompletedAt
	if err := at.Scan(valid); err != nil || !at.Time().Equal(valid) {
		t.Errorf("Scan(%v) = %v, leaving %v; want it held", valid, err, at)
	}

	tests := []struct {
		name string
		src  any
		want string // the error's text, or its code for a rule's
	}{
		{name: "rejected by the rule", src: invalid, want: "greater_than_or_equal_to"},
		{name: "NULL", src: nil,
			want: "emit: cannot scan NULL into CompletedAt; scan a column that may be NULL into *CompletedAt"},
		{name: "not the type held", src: 1, want: "unsupported Scan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			at := emit.MustCompletedAt(valid)
			err := at.Scan(tt.src)
			if _, ok := errors.AsType[*rule.Error](err); ok {
				wantCode(t, err, tt.want)
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Scan(%v) = %v, want an error containing %q", tt.src, err, tt.want)
			}
			if !at.Time().Equal(valid) {
				t.Errorf("Scan(%v) left %v, want it left as %v", tt.src, at, valid)
			}
		})
	}
}

func TestGenerated_EnumNew(t *testing.T) {
	t.Parallel()

	if got, err := emit.NewStatus("paid"); err != nil || got != emit.StatusPaid {
		t.Errorf(`NewStatus("paid") = %q, %v; want StatusPaid`, got, err)
	}
	for _, v := range []string{"", "Paid", "refunded"} {
		got, err := emit.NewStatus(v)
		wantCode(t, err, "inclusion")
		if got != "" {
			t.Errorf("NewStatus(%q) = %q, want the zero Status", v, got)
		}
	}
}

func TestGenerated_EnumValidate(t *testing.T) {
	t.Parallel()

	for _, s := range emit.StatusValues() {
		if err := s.Validate(); err != nil {
			t.Errorf("%q.Validate() = %v, want nil", s, err)
		}
	}
	for _, s := range []emit.Status{"", "Paid", "refunded"} {
		wantCode(t, s.Validate(), "inclusion")
	}
}

func TestGenerated_EnumValues(t *testing.T) {
	t.Parallel()

	values := emit.StatusValues()
	if want := []emit.Status{emit.StatusPlaced, emit.StatusPaid, emit.StatusShipped}; !slices.Equal(values, want) {
		t.Errorf("StatusValues() = %q, want %q", values, want)
	}
	values[0] = "changed by a caller"
	if got := emit.StatusValues()[0]; got != emit.StatusPlaced {
		t.Errorf("StatusValues()[0] = %q after a caller changed its slice, want %q", got, emit.StatusPlaced)
	}
}

// order holds an enum as a request body would.
type order struct {
	Status   emit.Status         `json:"status"`
	Previous *emit.Status        `json:"previous"`
	Counts   map[emit.Status]int `json:"counts"`
}

func TestGenerated_EnumJSON(t *testing.T) {
	t.Parallel()

	paid := emit.StatusPaid
	b, err := json.Marshal(order{Status: emit.StatusShipped, Previous: &paid, Counts: map[emit.Status]int{paid: 1}})
	if want := `{"status":"shipped","previous":"paid","counts":{"paid":1}}`; err != nil || string(b) != want {
		t.Errorf("Marshal = %s, %v; want %s", b, err, want)
	}

	var got order
	if err := json.Unmarshal(b, &got); err != nil || got.Status != emit.StatusShipped ||
		got.Previous == nil || *got.Previous != paid || got.Counts[paid] != 1 {
		t.Errorf("Unmarshal(%s) = %+v, %v; want the values back", b, got, err)
	}

	tests := []struct {
		name string
		body string
		code string // of the rule's error, or "" for encoding/json's own
	}{
		{name: "not a constant", body: `{"status":"refunded"}`, code: "inclusion"},
		{name: "not a constant, through a pointer", body: `{"previous":"refunded"}`, code: "inclusion"},
		{name: "not a constant, as a key", body: `{"counts":{"refunded":1}}`, code: "inclusion"},
		{name: "not a string", body: `{"status":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := order{Status: emit.StatusPlaced}
			err := json.Unmarshal([]byte(tt.body), &got)
			if tt.code != "" {
				wantCode(t, err, tt.code)
			} else if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
				t.Errorf("err = %v, want encoding/json's *UnmarshalTypeError", err)
			}
			if got.Status != emit.StatusPlaced {
				t.Errorf("Status = %q after an error, want it left as %q", got.Status, emit.StatusPlaced)
			}
		})
	}

	got = order{Status: emit.StatusPlaced}
	if err := json.Unmarshal([]byte(`{"status":null}`), &got); err != nil || got.Status != emit.StatusPlaced {
		t.Errorf("Unmarshal of null = %v, leaving %q; want nil, leaving it as it was", err, got.Status)
	}
}

func TestGenerated_EnumScan(t *testing.T) {
	t.Parallel()

	for _, src := range []any{"paid", []byte("paid")} {
		var s emit.Status
		if err := s.Scan(src); err != nil || s != emit.StatusPaid {
			t.Errorf("Scan(%#v) = %v, leaving %q; want StatusPaid", src, err, s)
		}
	}

	tests := []struct {
		name string
		src  any
		want string // the error's text, or its code for a rule's
	}{
		{name: "not a constant", src: "refunded", want: "inclusion"},
		{name: "NULL", src: nil, want: "emit: cannot scan NULL into Status; scan a column that may be NULL into *Status"},
		{name: "nothing a string is scanned from", src: struct{}{}, want: "unsupported Scan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := emit.StatusPlaced
			err := s.Scan(tt.src)
			if _, ok := errors.AsType[*rule.Error](err); ok {
				wantCode(t, err, tt.want)
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Scan(%v) = %v, want an error containing %q", tt.src, err, tt.want)
			}
			if s != emit.StatusPlaced {
				t.Errorf("Scan(%v) left %q, want it left as %q", tt.src, s, emit.StatusPlaced)
			}
		})
	}
}
