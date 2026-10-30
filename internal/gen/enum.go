package gen

import (
	"go/constant"
	"go/token"
	"go/types"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/directive"
	"github.com/go-kanna/internal/enumtype"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
)

// enumKey spells the directive //kanna:enum.
const enumKey = "enum"

// enum is a string type marked //kanna:enum, which takes only the values of
// the constants declared beside it.
type enum struct {
	scan.Type

	// consts are the constants of the type, in the order they are declared.
	consts []*types.Const

	// newFunc and valuesFunc name the functions generated for the type.
	newFunc, valuesFunc string
}

// enumMethods are the methods generated on an enum.
var enumMethods = []string{"UnmarshalText", "Scan"}

// enums returns the types marked //kanna:enum that an enum can be generated
// from, in the order scan reports them, and says what is wrong with the rest.
// It also tells a directive on the wrong kind of type: //kanna:value on a type
// other than a struct, which scan.Structs does not read, and //kanna:enum on a
// struct.
func enums(fset *token.FileSet, decls []scan.Type) ([]enum, []diag.Diag) {
	var (
		found []enum
		diags []diag.Diag
	)
	for _, t := range decls {
		if _, ok := t.Named.Underlying().(*types.Struct); ok {
			d, ds := directive.Find(fset, t.Doc, enumKey)
			diags = append(diags, ds...)
			if d.Found {
				diags = append(diags, diag.Errorf(t.Pos, "%s is a struct, which //kanna:%s does not mark", t.Name, enumKey).
					WithHints("mark it //kanna:"+directiveKey+" to generate a value type"))
			}
			continue
		}

		d, ds := directive.Find(fset, t.Doc, directiveKey)
		diags = append(diags, ds...)
		if d.Found {
			hint := "hold the value in a struct with one unexported field"
			if isString(t.Named) {
				hint = "mark it //kanna:" + enumKey + " to accept only the constants declared beside it"
			}
			diags = append(diags, diag.Errorf(t.Pos, "%s is not a struct, which //kanna:%s marks", t.Name, directiveKey).
				WithHints(hint))
		}

		d, ds = directive.Find(fset, t.Doc, enumKey)
		diags = append(diags, ds...)
		if !d.Found {
			continue
		}
		e, ds := checkEnum(fset, t, d)
		diags = append(diags, ds...)
		if len(ds) == 0 {
			found = append(found, e)
		}
	}
	return found, diags
}

// checkEnum works out the enum t, marked by d, and says why it cannot be one.
func checkEnum(fset *token.FileSet, t scan.Type, d directive.Directive) (enum, []diag.Diag) {
	var diags []diag.Diag
	if d.Args != "" {
		diags = append(diags, diag.Errorf(d.Pos, "//kanna:%s takes no arguments, got %q", enumKey, d.Args))
	}

	if t.Named.TypeParams().Len() > 0 {
		return enum{}, append(diags, diag.Errorf(t.Pos, "generic type %s is not supported", t.Name))
	}
	if packages.Invalid(t.Named.Underlying()) {
		return enum{}, append(diags, packages.NotTypeChecked(t.Pos))
	}
	if !isString(t.Named) {
		return enum{}, append(diags, diag.Errorf(t.Pos,
			"%s must be a string type to be an enum, but its underlying type is %s",
			t.Name, types.TypeString(t.Named.Underlying(), types.RelativeTo(t.Named.Obj().Pkg()))).
			WithHints("declare it as type "+t.Name+" string"))
	}

	e := enum{Type: t, consts: enumtype.Constants(t.Named)}
	e.newFunc, e.valuesFunc = enumtype.Funcs(t.Name)
	if len(e.consts) == 0 {
		return enum{}, append(diags, diag.Errorf(t.Pos, "%s has no constants to be one of", t.Name).
			WithHints("declare them beside it, such as const "+t.Name+"A "+t.Name+` = "a"`))
	}

	held := make(map[string]string)
	for _, c := range e.consts {
		pos := fset.Position(c.Pos())
		if c.Val().Kind() != constant.String {
			diags = append(diags, packages.NotTypeChecked(pos))
			continue
		}
		v := constant.StringVal(c.Val())
		switch first, ok := held[v]; {
		case v == "":
			diags = append(diags, diag.Errorf(pos, `%s holds "", the zero value of %s, which stands for none`, c.Name(), t.Name).
				WithHints("remove it, and use a *"+t.Name+" where a value may be absent"))
		case ok:
			diags = append(diags, diag.Errorf(pos, "%s holds %q, as %s does", c.Name(), v, first).
				WithHints("give each constant a value of its own"))
		default:
			held[v] = c.Name()
		}
	}

	diags = append(diags, checkMethods(fset, t.Named, enumMethods, enumKey)...)
	diags = append(diags, checkFuncs(fset, t.Named, []string{e.newFunc, e.valuesFunc}, enumKey, "for "+t.Name)...)
	return e, diags
}

// isString reports whether named is a string type.
func isString(named *types.Named) bool {
	return types.Identical(named.Underlying(), types.Typ[types.String])
}
