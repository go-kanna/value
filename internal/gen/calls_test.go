package gen_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kanna/internal/cli"
	"github.com/go-kanna/internal/packages"
)

// emailSrc marks Email, whose constructors and methods the code below calls.
const emailSrc = "package app\n\n//kanna:value\ntype Email struct{ s string }\n\n" +
	"func (Email) validate(string) error { return nil }\n"

// compiles reports whether the packages of dir load without errors.
func compiles(t *testing.T, dir string) {
	t.Helper()

	set, err := packages.Load([]string{"./..."}, packages.Config{Dir: dir})
	if err != nil || len(set.Diags) > 0 {
		t.Errorf("the module does not compile: %v %v", err, set.Diags)
	}
}

func TestCommand_GeneratesWhatCodeAlreadyCalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		args  []string
	}{
		{
			name: "through interfaces",
			files: map[string]string{"use.go": `package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

var (
	_ fmt.Stringer   = Email{}
	_ json.Marshaler = Email{}
	_ sql.Scanner    = (*Email)(nil)
)

func show(s fmt.Stringer) string { return s.String() }
func label(e Email) string       { return show(e) }
`},
		},
		{
			name: "a method promoted",
			files: map[string]string{
				"use.go": "package app\n\ntype User struct{ Email }\n\nfunc (u User) label() string { return u.String() }\n",
			},
		},
		{
			name: "an enum",
			files: map[string]string{
				"status.go": "package app\n\n//kanna:enum\ntype Status string\n\nconst StatusOpen Status = \"open\"\n",
				"use.go": `package app

import "encoding"

var _ encoding.TextUnmarshaler = (*Status)(nil)

func parse(s string) (Status, error) { return NewStatus(s) }

func statuses() []Status { return StatusValues() }
`,
			},
		},
		{
			name: "another package",
			files: map[string]string{
				"app.go":         "package app\n",
				"model/model.go": strings.Replace(emailSrc, "package app", "package model", 1),
				"api/api.go": "package api\n\nimport \"example.com/app/model\"\n\n" +
					"var admin = model.MustEmail(\"admin@example.com\")\n\n" +
					"func domainOf(e model.Email) string { return e.String() }\n",
			},
			args: []string{"./..."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := map[string]string{"app.go": emailSrc}
			maps.Copy(files, tt.files)
			dir := writeModule(t, files)
			if code, stdout, stderr := run(t, dir, tt.args...); code != cli.ExitOK || stdout != "" || stderr != "" {
				t.Fatalf("exit %d, stdout %q, stderr %q; want success", code, stdout, stderr)
			}
			compiles(t, dir)
		})
	}
}

func TestCommand_Mistakes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		args  []string
		want  string
	}{
		{
			// Before the constructors are written, go/types checks no further
			// than undefined: MustEmail.
			name: "mistakes behind calls to what is generated",
			files: map[string]string{"use.go": `package app

var _ = MustEmail("a@b").Bogus()

func assigned() string {
	e := MustEmail("a@b")
	return e.Bogus()
}

func takesInt(int) {}
func passed()      { takesInt(MustEmail("a@b")) }

func multi() Email {
	e := NewEmail("a@b")
	return e
}

func result(e Email) int { return e.String() }

var _ = MustEmail(42)
`},
			want: "./use.go:3:26: error: MustEmail(\"a@b\").Bogus undefined (type Email has no field or method Bogus)\n" +
				"./use.go:7:11: error: e.Bogus undefined (type Email has no field or method Bogus)\n" +
				"./use.go:11:31: error: cannot use MustEmail(\"a@b\") (value of struct type Email) as int value " +
				"in argument to takesInt\n" +
				"./use.go:14:7: error: assignment mismatch: 1 variable but NewEmail returns 2 values\n" +
				"./use.go:18:35: error: cannot use e.String() (value of type string) as int value in return statement\n" +
				"./use.go:20:19: error: cannot use 42 (untyped int constant) as string value in argument to MustEmail\n",
		},
		{
			name: "an error not about what is generated",
			files: map[string]string{
				"use.go": "package app\n\nvar admin = MustEmail(\"admin@example.com\")\n\nvar broken int = \"x\"\n",
			},
			want: "./use.go:5:18: error: cannot use \"x\" (untyped string constant) as int value " +
				"in variable declaration\n",
		},
		{
			// MustEmail is not written either, for the error of Email.
			name: "an error of a value type, and a call to its constructor",
			files: map[string]string{
				"app.go": strings.Replace(emailSrc, "validate(string) error { return nil }",
					"validate(s string) bool { return s != \"\" }", 1),
				"use.go": "package app\n\nvar admin = MustEmail(\"admin@example.com\")\n",
			},
			want: "./use.go:3:13: error: undefined: MustEmail\n" +
				"./app.go:6:14: error: Email.validate must be func (Email) validate(string) error " +
				"for the generated code to check values with\n",
		},
		{
			// MustEmail is written, so its call says nothing.
			name: "an error of a value type, and a call to the constructor of another",
			files: map[string]string{
				"app.go": emailSrc + "\n//kanna:value\ntype Phone struct{ s string }\n\n" +
					"func (Phone) validate(s string) bool { return s != \"\" }\n",
				"use.go": "package app\n\nvar admin = MustEmail(\"admin@example.com\")\n",
			},
			want: "./app.go:11:14: error: Phone.validate must be func (Phone) validate(string) error " +
				"for the generated code to check values with\n",
		},
		{
			// What would be generated for an invalid type is left to the type
			// error.
			name:  "a held type that does not type-check",
			files: map[string]string{"app.go": strings.Replace(emailSrc, "s string }", "s strng }", 1)},
			want:  "./app.go:4:22: error: undefined: strng\n",
		},
		{
			// Only what the run generates is let past, in whichever package.
			name: "a constructor of a struct not marked in another package",
			files: map[string]string{
				"app.go": "package app\n",
				"a/a.go": strings.Replace(emailSrc, "package app", "package a", 1),
				"b/b.go": "package b\n\ntype Email struct{ s string }\n\n" +
					"var admin, _ = NewEmail(\"admin@example.com\")\n\nfunc domainOf(e Email) string { return e.String() }\n",
			},
			args: []string{"./..."},
			want: "b/b.go:5:16: error: undefined: NewEmail\n" +
				"b/b.go:7:42: error: e.String undefined (type Email has no field or method String)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := map[string]string{"app.go": emailSrc}
			maps.Copy(files, tt.files)
			dir := writeModule(t, files)
			if code, _, stderr := run(t, dir, tt.args...); code != cli.ExitFailure || stderr != tt.want {
				t.Errorf("exit %d, stderr:\n%s\nwant %d and\n%s", code, stderr, cli.ExitFailure, tt.want)
			}
			if _, err := os.Stat(filepath.Join(dir, "value_gen.go")); err == nil {
				t.Error("value_gen.go was written")
			}
		})
	}
}

func TestCommand_KeepsWhatCodeStillCalls(t *testing.T) {
	t.Parallel()

	const phone = "\n//kanna:value\ntype Phone struct{ s string }\n\nfunc (Phone) validate(string) error { return nil }\n"
	tests := []struct {
		name, app, use, edit, with, want string
	}{
		{
			name: "the directive goes", app: emailSrc,
			use: "package app\n\nvar admin = MustEmail(\"admin@example.com\")\n\n" +
				"func domainOf(e Email) string { return e.String() }\n",
			edit: "//kanna:value\n",
			want: "./use.go:3:13: error: undefined: MustEmail\n" +
				"./use.go:5:42: error: e.String undefined (type Email has no field or method String)\n",
		},
		{
			name: "the directive of one of two goes", app: emailSrc + phone,
			use:  "package app\n\nvar support = MustPhone(\"0120-000-000\")\n",
			edit: "//kanna:value\ntype Phone", with: "type Phone",
			want: "./use.go:3:15: error: undefined: MustPhone\n",
		},
		{
			// The output of the last run stays as it was, with its bodies
			// stubbed for the load.
			name: "the held type changes",
			app:  "package app\n\n//kanna:value\ntype Age struct{ n int }\n\nfunc (Age) validate(int) error { return nil }\n",
			use:  "package app\n\nfunc ageOf(n int) Age { return MustAge(n) }\n\nfunc years(a Age) int { return a.Int() }\n",
			edit: "int", with: "uint8",
			want: "./use.go:3:40: error: cannot use n (variable of type int) as uint8 value in argument to MustAge\n" +
				"./use.go:5:34: error: a.Int undefined (type Age has no field or method Int)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := writeModule(t, map[string]string{"app.go": tt.app, "use.go": tt.use})
			if code, _, stderr := run(t, dir); code != cli.ExitOK {
				t.Fatalf("first run: exit %d, stderr %q", code, stderr)
			}
			generated := readFile(t, filepath.Join(dir, "value_gen.go"))

			writeFile(t, filepath.Join(dir, "app.go"), strings.ReplaceAll(tt.app, tt.edit, tt.with))
			if code, _, stderr := run(t, dir); code != cli.ExitFailure || stderr != tt.want {
				t.Errorf("exit %d, stderr:\n%s\nwant %d and\n%s", code, stderr, cli.ExitFailure, tt.want)
			}
			if got := readFile(t, filepath.Join(dir, "value_gen.go")); got != generated {
				t.Errorf("value_gen.go was rewritten:\n%s", got)
			}
		})
	}
}

func TestCommand_RegeneratesWhatWasSetAside(t *testing.T) {
	t.Parallel()

	// Renaming Raw leaves the last run's output declaring what is gone, so the
	// load sets it aside whole: Email has no String then.
	const raw = "package app\n\ntype Raw string\n\n//kanna:value\ntype Email struct{ s Raw }\n\n" +
		"func (Email) validate(Raw) error { return nil }\n"
	tests := []struct {
		name, use, want string
	}{
		{
			name: "an interface Email satisfies",
			use: "package app\n\nimport \"fmt\"\n\nvar admin = MustEmail(\"admin@example.com\")\n\n" +
				"var _ fmt.Stringer = Email{}\n",
		},
		{
			name: "a mistake behind a call",
			use:  "package app\n\nvar admin = MustEmail(\"admin@example.com\")\n\nvar _ = MustEmail(\"a@b\").Bogus()\n",
			want: "./use.go:5:26: error: MustEmail(\"a@b\").Bogus undefined (type Email has no field or method Bogus)\n" +
				"./value_gen.go: warning: set aside because it no longer compiles; it is regenerated once the errors " +
				"above are fixed\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := writeModule(t, map[string]string{
				"app.go": raw,
				"use.go": "package app\n\nvar admin = MustEmail(\"admin@example.com\")\n",
			})
			if code, _, stderr := run(t, dir); code != cli.ExitOK {
				t.Fatalf("first run: exit %d, stderr %q", code, stderr)
			}
			writeFile(t, filepath.Join(dir, "use.go"), tt.use)
			writeFile(t, filepath.Join(dir, "app.go"), strings.ReplaceAll(raw, "Raw", "Addr"))

			code, _, stderr := run(t, dir)
			if tt.want == "" {
				if code != cli.ExitOK || stderr != "" {
					t.Fatalf("exit %d, stderr %q; want success", code, stderr)
				}
				compiles(t, dir)
				if got := readFile(t, filepath.Join(dir, "value_gen.go")); !strings.Contains(got, "func NewEmail(v Addr)") {
					t.Errorf("value_gen.go lacks NewEmail(v Addr):\n%s", got)
				}
				return
			}
			if code != cli.ExitFailure || stderr != tt.want {
				t.Errorf("exit %d, stderr:\n%s\nwant %d and\n%s", code, stderr, cli.ExitFailure, tt.want)
			}
		})
	}
}
