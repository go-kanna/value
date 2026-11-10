package gen

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/directive"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
)

// marks are the shapes of the declarations each directive marks. An enum is a
// string type, which checkEnum tells apart from the other types.
var marks = map[string]directive.Shape{
	directiveKey: directive.StructType,
	enumKey:      directive.OtherType,
}

// misplaced tells each //kanna:value and //kanna:enum above a declaration of a
// shape it does not mark, in the order they are written. A struct or a string
// type is hinted to the other directive.
func misplaced(fset *token.FileSet, pkgs []*packages.Package) []diag.Diag {
	hints := make(map[token.Position]string)
	for _, t := range scan.Types(pkgs) {
		key, hint := enumKey, "or mark it //kanna:"+directiveKey+" to generate a value type"
		if _, ok := t.Named.Underlying().(*types.Struct); !ok {
			if !isString(t.Named) {
				continue
			}
			key, hint = directiveKey, "or mark it //kanna:"+enumKey+" to accept only the constants declared beside it"
		}
		if d, _ := directive.Find(fset, t.Doc, key); d.Found {
			hints[d.Pos] = hint
		}
	}

	var diags []diag.Diag
	for _, pkg := range pkgs {
		syntax := slices.DeleteFunc(slices.Clone(pkg.Syntax), ast.IsGenerated)
		for _, l := range directive.Lines(fset, syntax) {
			s, ok := marks[l.Key]
			if !ok {
				continue
			}
			for _, d := range directive.Misplaced([]directive.Line{l}, pkg.Types, s, l.Key) {
				if hint, ok := hints[l.Pos]; ok {
					d = d.WithHints(hint)
				}
				diags = append(diags, d)
			}
		}
	}
	return diags
}
