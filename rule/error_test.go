package rule_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-kanna/value/rule"
)

func TestError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *rule.Error
		want string
	}{
		{
			name: "code only",
			err:  rule.Fail("email"),
			want: "rule: email",
		},
		{
			name: "params in key order",
			err:  rule.FailWith("range", map[string]any{"min": 18, "max": 130}),
			want: "rule: range (max=130, min=18)",
		},
		{
			name: "empty params",
			err:  rule.FailWith("positive", map[string]any{}),
			want: "rule: positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFailWith_CopiesParams(t *testing.T) {
	t.Parallel()

	params := map[string]any{"max": 20}
	err := rule.FailWith("too_long", params)
	params["max"] = 99

	if got := err.Params()["max"]; got != 20 {
		t.Errorf(`Params()["max"] = %v, want 20`, got)
	}
}

func TestError_ParamsReturnsCopy(t *testing.T) {
	t.Parallel()

	err := rule.FailWith("too_long", map[string]any{"max": 20})
	err.Params()["max"] = 99

	if got := err.Params()["max"]; got != 20 {
		t.Errorf(`Params()["max"] = %v, want 20`, got)
	}
}

func TestError_As(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("decode email: %w", rule.Fail("email"))

	var rerr *rule.Error
	if !errors.As(err, &rerr) {
		t.Fatalf("errors.As(%v) = false, want true", err)
	}
	if got := rerr.Code(); got != "email" {
		t.Errorf("Code() = %q, want %q", got, "email")
	}
}
