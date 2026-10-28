package gen_test

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-kanna/internal/output"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/pkgtest"

	"github.com/go-kanna/value/internal/gen"
)

// TestEmit compares the output for testdata/emit with its golden file,
// value_gen.go, a part of the package that TestGenerated compiles and calls.
// To accept a change in the output:
//
//	go test ./internal/gen -run TestEmit -update
func TestEmit(t *testing.T) {
	t.Parallel()

	const dir = "testdata/emit"
	set, err := packages.Load([]string{"./" + dir}, packages.Config{Regenerated: []string{"value_gen.go"}})
	if err != nil || len(set.Diags) > 0 {
		t.Fatalf("load: %v %v", err, set.Diags)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	declared := slices.Sorted(maps.Keys(output.DeclaredNames(abs, "value_gen.go", set.Packages[0].Name)))

	got, err := gen.Emit(set.Fset, set.Packages, declared)
	if err != nil {
		t.Fatalf("emit: %v\n%s", err, got)
	}
	pkgtest.Golden(t, filepath.Join(dir, "value_gen.go"), got)
}

func TestEmit_KeepsClearOfDeclaredNames(t *testing.T) {
	t.Parallel()

	pkg := pkgtest.Load(t, map[string]string{"test.go": `package test

type tags []string

//kanna:value
type Tags struct{ t tags }

func (Tags) validate(tags) error { return nil }
`})

	declared := []string{"x", "v", "err", "y", "data", "src", "json", "slices", "driver", "sql", "errors", "fmt"}
	got, err := gen.Emit(pkg.Fset, []*packages.Package{pkg}, declared)
	if err != nil {
		t.Fatalf("emit: %v\n%s", err, got)
	}

	for _, want := range []string{
		"func NewTags(v2 tags) (Tags, error) {",
		"v2 = slices2.Clone(v2)",
		"if err2 := (Tags{}).validate(v2); err2 != nil {",
		"func (x2 *Tags) UnmarshalJSON(data2 []byte) error {",
		"if err2 := json2.Unmarshal(data2, &v2); err2 != nil {",
		"func (x2 Tags) Value() (driver2.Value, error) {",
		"func (x2 *Tags) Scan(src2 any) error {",
		"var v2 sql2.Null[tags]",
		"return errors2.New(",
		"return fmt2.Sprint(x2.t)",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}
