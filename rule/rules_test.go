package rule_test

import (
	"errors"
	"testing"

	"github.com/go-kanna/value/rule"
)

type ruleTest struct {
	in   string
	want bool
}

func TestEmail(t *testing.T) {
	t.Parallel()

	runRule(t, rule.Email, "email", []ruleTest{
		{in: "a@example.com", want: true},
		{in: "a@localhost", want: true},
		{in: "josé@example.com", want: true},
		{in: "a@[192.0.2.1]", want: true},
		{in: "", want: false},
		{in: "a", want: false},
		{in: "@example.com", want: false},
		{in: "a@", want: false},
		{in: "a@@example.com", want: false},
		{in: "José <jose@example.com>", want: false},
		{in: "<a@example.com>", want: false},
		{in: "a@example.com (comment)", want: false},
		{in: " a@example.com", want: false},
		{in: "a@example.com ", want: false},
	})
}

func TestURL(t *testing.T) {
	t.Parallel()

	runRule(t, rule.URL, "url", []ruleTest{
		{in: "https://example.com/path?q=1", want: true},
		{in: "http://localhost:8080", want: true},
		{in: "https://[::1]/", want: true},
		{in: "ftp://example.com", want: true},
		{in: "", want: false},
		{in: "example.com", want: false},
		{in: "/path", want: false},
		{in: "//example.com", want: false},
		{in: "http://", want: false},
		{in: "mailto:a@example.com", want: false},
		{in: "javascript:alert(1)", want: false},
		{in: "https://exa mple.com", want: false},
		{in: " https://example.com", want: false},
	})
}

func TestUUID(t *testing.T) {
	t.Parallel()

	runRule(t, rule.UUID, "uuid", []ruleTest{
		{in: "123e4567-e89b-12d3-a456-426614174000", want: true},
		{in: "123E4567-E89B-12D3-A456-426614174000", want: true},
		{in: "00000000-0000-0000-0000-000000000000", want: true},
		{in: "", want: false},
		{in: "123e4567e89b12d3a456426614174000", want: false},
		{in: "{123e4567-e89b-12d3-a456-426614174000}", want: false},
		{in: "123e4567-e89b-12d3-a456-42661417400", want: false},
		{in: "123e4567-e89b-12d3-a456-4266141740000", want: false},
		{in: "123e4567_e89b-12d3-a456-426614174000", want: false},
		{in: "123e4567-e89b-12d3-a456-42661417400g", want: false},
	})
}

// runRule checks that check accepts exactly the inputs marked want, and that it
// rejects the rest with code.
func runRule(t *testing.T, check func(string) error, code string, tests []ruleTest) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			err := check(tt.in)
			if tt.want {
				if err != nil {
					t.Errorf("check(%q) = %v, want nil", tt.in, err)
				}
				return
			}

			var rerr *rule.Error
			if !errors.As(err, &rerr) || rerr.Code() != code {
				t.Errorf("check(%q) = %v, want code %q", tt.in, err, code)
			}
		})
	}
}
