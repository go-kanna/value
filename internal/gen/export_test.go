package gen

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
)

var zeroSpellings = map[zeroTest]string{
	zeroMethod:  "zero=IsZero",
	zeroCompare: "zero=compare",
	zeroNil:     "zero=nil",
	zeroNone:    "zero=none",
}

// Targets finds the targets in pkgs, spelling each as its struct and what the
// generated code does with the value, such as "CompletedAt{t time.Time} Time()
// zero=IsZero Equal String NewCompletedAt/MustCompletedAt", where Equal and
// String are the value's own methods; and spelling each diagnostic as
// diag.Diag.String does.
func Targets(fset *token.FileSet, pkgs []*packages.Package) ([]string, []string) {
	found, diags := targets(fset, scan.Structs(pkgs))

	spelled := make([]string, len(found))
	for i, t := range found {
		words := []string{
			t.Name + "{" + t.field.Name + " " + typeString(t.Struct, t.field.Type) + "}",
			t.accessor + "()",
			zeroSpellings[t.zero],
		}
		if t.hasEqual {
			words = append(words, "Equal")
		}
		if t.hasString {
			words = append(words, "String")
		}
		if t.clone != "" {
			words = append(words, "clone="+t.clone)
		}
		words = append(words, t.newFunc+"/"+t.mustFunc)
		spelled[i] = strings.Join(words, " ")
	}
	msgs := make([]string, len(diags))
	for i, d := range diags {
		msgs[i] = d.String()
	}
	return spelled, msgs
}

// Emit renders the file the targets in pkgs, which make one package, are
// generated into. declared are the top-level names the package declares
// besides.
func Emit(fset *token.FileSet, pkgs []*packages.Package, declared []string) ([]byte, error) {
	found, diags := targets(fset, scan.Structs(pkgs))
	if len(diags) > 0 {
		return nil, fmt.Errorf("%v", diags)
	}
	return emit(pkgs[0].PkgPath, pkgs[0].Name, found, declared).Bytes() //nolint:wrapcheck // the test shows it as it is
}
