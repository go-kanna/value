package gen_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-kanna/internal/deps"
)

// Code calling what the run writes does not compile before it, which is when
// the kanna command asks.
func TestDeps(t *testing.T) {
	t.Parallel()

	dir := writeModule(t, map[string]string{
		"p/p.go": "package p\n\n//kanna:value\ntype Email struct{ s string }\n\n" +
			"func (Email) validate(string) error { return nil }\n\n//kanna:enum\ntype Color string\n\n" +
			"const ColorRed Color = \"red\"\n\nvar _, _ = NewEmail(\"a\")\n",
		"q/q.go": "package q\n",
	})

	code, stdout, stderr := run(t, filepath.Join(dir, "p"), "-deps")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var r deps.Report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(r.Packages, []string{"example.com/app/p"}) {
		t.Errorf("Packages = %q", r.Packages)
	}
	if !r.SourcesOnly {
		t.Error("SourcesOnly = false, want true: kanna-value reads what is written by hand")
	}
	if len(r.Writes) != 1 || !strings.HasSuffix(r.Writes[0], filepath.Join("p", "value_gen.go")) {
		t.Errorf("Writes = %q", r.Writes)
	}
	for _, name := range []string{"example.com/app/p.NewEmail", "example.com/app/p.NewColor"} {
		if !slices.Contains(r.Declares, name) {
			t.Errorf("Declares = %q, want %s among them", r.Declares, name)
		}
	}
}

func TestDeps_NothingMarked(t *testing.T) {
	t.Parallel()

	dir := writeModule(t, map[string]string{"p/p.go": "package p\n"})

	code, stdout, stderr := run(t, filepath.Join(dir, "p"), "-deps")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var r deps.Report
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Writes) != 0 {
		t.Errorf("Writes = %q, want none", r.Writes)
	}
}
