package gen

import (
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/directive"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
	"github.com/go-kanna/internal/valuetype"
)

// directiveKey spells the directive //kanna:value.
const directiveKey = "value"

// outputFile is the file generated in each package.
const outputFile = "value_gen.go"

// target is a struct marked //kanna:value, with what the generated code needs
// to know about the value it holds.
type target struct {
	scan.Struct

	// field holds the value.
	field scan.Field

	// accessor is the name of the method returning the value, after its type:
	// Time for a time.Time, String for a string.
	accessor string

	// newFunc and mustFunc name the constructors.
	newFunc, mustFunc string

	// zero is how the generated IsZero tells the zero value.
	zero zeroTest

	// hasEqual and hasString report whether the value has an Equal or String
	// method of its own, which the generated method of the same name calls.
	hasEqual, hasString bool

	// clone names the package whose Clone copies the value, "slices" or "maps",
	// when the value is a slice or a map. The generated code copies it on the
	// way in and on the way out, so that nobody else holds what was validated.
	clone string
}

// zeroTest is how the generated IsZero tells the zero value.
type zeroTest int

const (
	// zeroMethod calls the value's own IsZero.
	zeroMethod zeroTest = iota
	// zeroCompare compares the value with the zero value of its type.
	zeroCompare
	// zeroNil compares a slice, map, or func with nil, as a required one is
	// told apart in kanna-validate.
	zeroNil
	// zeroNone leaves IsZero out, for a value that none of the above can tell,
	// such as a struct holding a slice. encoding/json's omitzero falls back on
	// reflection for it.
	zeroNone
)

// targets returns the structs marked //kanna:value that a value type can be
// generated from, in the order scan reports them, and says what is wrong with
// the rest.
func targets(fset *token.FileSet, structs []scan.Struct) ([]target, []diag.Diag) {
	var (
		found []target
		diags []diag.Diag
	)
	for _, s := range structs {
		d, ds := directive.Find(fset, s.Doc, directiveKey)
		diags = append(diags, ds...)
		if !d.Found {
			continue
		}

		t, ds := check(fset, s, d)
		diags = append(diags, ds...)
		if len(ds) == 0 {
			found = append(found, t)
		}
	}
	return found, diags
}

// check works out the target s, marked by d, and says why it cannot be one.
func check(fset *token.FileSet, s scan.Struct, d directive.Directive) (target, []diag.Diag) {
	var diags []diag.Diag
	if d.Args != "" {
		diags = append(diags, diag.Errorf(d.Pos, "//kanna:%s takes no arguments, got %q", directiveKey, d.Args))
	}

	// A method on a generic type would need its type parameters, and its rule
	// could not be checked until it is instantiated.
	if s.Named.TypeParams().Len() > 0 {
		return target{}, append(diags, diag.Errorf(s.Pos, "generic struct %s is not supported", s.Name))
	}

	f, ds := valueField(s)
	if diags = append(diags, ds...); len(ds) > 0 {
		return target{}, diags
	}
	if packages.Invalid(f.Type) {
		return target{}, append(diags, packages.NotTypeChecked(f.Pos))
	}
	t := target{Struct: s, field: f}

	accessor, ds := accessorOf(s, f)
	if diags = append(diags, ds...); len(ds) > 0 {
		return target{}, diags
	}
	t.accessor = accessor
	t.zero = zeroOf(f.Type)
	t.hasEqual = hasMethod(f.Type, "Equal", f.Type, types.Typ[types.Bool])
	t.hasString = hasMethod(f.Type, "String", nil, types.Typ[types.String])
	switch f.Type.Underlying().(type) {
	case *types.Slice:
		t.clone = "slices"
	case *types.Map:
		t.clone = "maps"
	}

	t.newFunc, t.mustFunc = valuetype.Constructors(s.Name)
	diags = append(diags, checkRule(fset, s, f.Type)...)
	diags = append(diags, checkMethods(fset, s.Named, append(generatedMethods(f.Type), t.accessor), directiveKey)...)
	diags = append(diags, checkFuncs(fset, s.Named, []string{t.newFunc, t.mustFunc}, directiveKey,
		"as a constructor of "+s.Name)...)
	return t, diags
}

// valueField returns the one field of s and says why s does not have one that
// only the generated code can set.
func valueField(s scan.Struct) (scan.Field, []diag.Diag) {
	if len(s.Fields) != 1 {
		return scan.Field{}, []diag.Diag{diag.Errorf(s.Pos,
			"%s must have exactly one field, the value it holds, but has %d", s.Name, len(s.Fields))}
	}

	f := s.Fields[0]
	switch {
	case f.Embedded:
		// An embedded field passes the methods of its type on, past the rule.
		return f, []diag.Diag{diag.Errorf(f.Pos,
			"%s embeds %s; give the field a name", s.Name, typeString(s, f.Type))}
	case f.Exported:
		return f, []diag.Diag{diag.Errorf(f.Pos,
			"%s.%s must be unexported, so that a value is set only through the generated code", s.Name, f.Name).
			WithHints("rename it to " + lowerFirst(f.Name))}
	case f.Name == "_":
		return f, []diag.Diag{diag.Errorf(f.Pos,
			"the field of %s needs a name for the generated code to refer to it by", s.Name)}
	}
	return f, nil
}

// accessorOf names the method returning the value of f after its type, and
// says why it cannot be named so.
func accessorOf(s scan.Struct, f scan.Field) (string, []diag.Diag) {
	if _, named := types.Unalias(f.Type).(*types.Named); named && types.IsInterface(f.Type) {
		return "", []diag.Diag{diag.Errorf(f.Pos,
			"%s holds an interface, %s, whose dynamic value no rule can be written for",
			s.Name, typeString(s, f.Type))}
	}
	name, ok := valuetype.Accessor(f.Type)
	if !ok {
		return "", []diag.Diag{diag.Errorf(f.Pos,
			"%s holds a %s, which has no name to call the method returning it after",
			s.Name, typeString(s, f.Type)).
			WithHints("declare a named type for it, and hold that")}
	}

	if slices.Contains(generatedMethods(f.Type), name) {
		return "", []diag.Diag{diag.Errorf(f.Pos,
			"the accessor of %s would be named %s, after its type %s, which collides with a generated method",
			s.Name, name, typeString(s, f.Type))}
	}
	return name, nil
}

// generatedMethods returns the methods generated on a struct holding a value
// of type t, besides the accessor.
func generatedMethods(t types.Type) []string {
	methods := []string{"MarshalJSON", "UnmarshalJSON", "Value", "Scan"}
	// The accessor of a string is its String method.
	if !types.Identical(t, types.Typ[types.String]) {
		methods = append(methods, "String")
	}
	if zeroOf(t) != zeroNone {
		methods = append(methods, "IsZero")
	}
	if hasMethod(t, "Equal", t, types.Typ[types.Bool]) {
		methods = append(methods, "Equal")
	}
	return methods
}

// zeroOf tells how the generated IsZero tells the zero value of type t.
func zeroOf(t types.Type) zeroTest {
	switch {
	case hasMethod(t, "IsZero", nil, types.Typ[types.Bool]):
		return zeroMethod
	case types.Comparable(t):
		return zeroCompare
	}
	switch t.Underlying().(type) {
	case *types.Slice, *types.Map, *types.Signature:
		return zeroNil
	}
	return zeroNone
}

// hasMethod reports whether a value of type t, held in a field the generated
// code can address, has the method name taking param, or nothing when param is
// nil, and returning result.
func hasMethod(t types.Type, name string, param, result types.Type) bool {
	obj, _, _ := types.LookupFieldOrMethod(t, true, nil, name)
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Signature()
	if sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), result) {
		return false
	}
	if param == nil {
		return sig.Params().Len() == 0
	}
	return sig.Params().Len() == 1 && types.Identical(sig.Params().At(0).Type(), param)
}

// checkRule says why s has no validate method that the generated code can
// check a value of type v with.
func checkRule(fset *token.FileSet, s scan.Struct, v types.Type) []diag.Diag {
	want := "func (" + s.Name + ") validate(" + typeString(s, v) + ") error"
	for m := range s.Named.Methods() {
		if m.Name() != "validate" {
			continue
		}
		sig := m.Signature()
		_, ptr := sig.Recv().Type().(*types.Pointer)
		if !ptr && sig.Params().Len() == 1 && types.Identical(sig.Params().At(0).Type(), v) &&
			sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.Universe.Lookup("error").Type()) {
			return nil
		}
		return []diag.Diag{diag.Errorf(fset.Position(m.Pos()),
			"%s.validate must be %s for the generated code to check values with", s.Name, want)}
	}
	return []diag.Diag{diag.Errorf(s.Pos, "%s has no validate method to check its values with", s.Name).
		WithHints("add " + want + ", returning nil to accept the value")}
}

// checkMethods says which methods of named the generated ones, names, would
// collide with. The ones an earlier run generated are about to be replaced.
// key spells the directive that marks named.
func checkMethods(fset *token.FileSet, named *types.Named, names []string, key string) []diag.Diag {
	name := named.Obj().Name()
	var diags []diag.Diag
	for m := range named.Methods() {
		if pos := fset.Position(m.Pos()); slices.Contains(names, m.Name()) && filepath.Base(pos.Filename) != outputFile {
			diags = append(diags, diag.Errorf(pos, "%s already has a method named %s", name, m.Name()).
				WithHints("remove it, or the //kanna:"+key+" on "+name))
		}
	}
	return diags
}

// checkFuncs says which declarations of the package of named the functions
// generated for it, funcs, would collide with. key spells the directive that
// marks named, and as says what the functions are generated as.
func checkFuncs(fset *token.FileSet, named *types.Named, funcs []string, key, as string) []diag.Diag {
	var diags []diag.Diag
	for _, fn := range funcs {
		obj := named.Obj().Pkg().Scope().Lookup(fn)
		if obj == nil {
			continue
		}
		if pos := fset.Position(obj.Pos()); filepath.Base(pos.Filename) != outputFile {
			diags = append(diags, diag.Errorf(pos, "%s is already declared, so it cannot be generated %s", fn, as).
				WithHints("remove it, or the //kanna:"+key+" on "+named.Obj().Name()))
		}
	}
	return diags
}

// lowerFirst lowers the first letter of s, which may take more than one byte.
func lowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

// typeString spells t as the package of s refers to it.
func typeString(s scan.Struct, t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == s.Named.Obj().Pkg() {
			return ""
		}
		return p.Name()
	})
}
