// Package vet is kanna-value vet, which finds the constants made values of an
// enum that are none of its constants.
package vet

import (
	"cmp"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/directive"
	"github.com/go-kanna/internal/enumtype"
	"github.com/go-kanna/internal/gofile"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
	"github.com/go-kanna/internal/suggest"
)

// enumKey spells the directive //kanna:enum.
const enumKey = "enum"

// generator names kanna-value in the header of the files it writes.
const generator = "kanna-value"

// Run checks the packages patterns match in dir, and returns what it finds.
// What they import is read to tell the enums, but not checked.
func Run(dir, tags string, patterns []string) []diag.Diag {
	// What the packages import is read from source, as an enum declared there
	// may have unexported constants, which export data may leave out.
	set, err := packages.Load(patterns, packages.Config{Dir: dir, Tags: tags, TypesInfo: true, Deps: true})
	if err != nil {
		return []diag.Diag{diag.Errorf(token.Position{}, "%v", err)}
	}
	if diag.HasErrors(set.Diags) {
		return set.Diags
	}
	if len(set.Packages) == 0 {
		return []diag.Diag{diag.Errorf(token.Position{}, "%s matched no packages", strings.Join(patterns, " "))}
	}

	c := checker{
		fset:   set.Fset,
		loaded: map[string]*packages.Package{},
		enums:  map[*types.TypeName]*enum{},
		marked: map[string]map[*types.TypeName]bool{},
	}
	for _, pkg := range set.Packages {
		c.index(pkg)
	}
	for _, pkg := range set.Packages {
		c.check(pkg)
	}
	slices.SortFunc(c.diags, func(a, b diag.Diag) int {
		return cmp.Or(strings.Compare(a.Pos.Filename, b.Pos.Filename),
			cmp.Compare(a.Pos.Line, b.Pos.Line), cmp.Compare(a.Pos.Column, b.Pos.Column))
	})
	return append(set.Diags, c.diags...)
}

// checker finds the constants of the packages it checks that are values of an
// enum but none of its constants.
type checker struct {
	fset *token.FileSet

	// loaded holds every package of the load, by import path.
	loaded map[string]*packages.Package

	// enums caches what enumOf finds of a type, nil for one that is no enum.
	enums map[*types.TypeName]*enum

	// marked caches, by import path, the types of a package marked
	// //kanna:enum.
	marked map[string]map[*types.TypeName]bool

	diags []diag.Diag
}

// enum is a string type marked //kanna:enum.
type enum struct {
	named *types.Named

	// consts are its constants in the order they are declared, the
	// unexported ones included.
	consts []*types.Const
}

// holds reports whether v is the value of one of e's constants.
func (e *enum) holds(v string) bool {
	return slices.ContainsFunc(e.consts, func(c *types.Const) bool {
		return c.Val().Kind() == constant.String && constant.StringVal(c.Val()) == v
	})
}

// index adds pkg and what it imports to c.loaded.
func (c *checker) index(pkg *packages.Package) {
	if _, ok := c.loaded[pkg.PkgPath]; ok {
		return
	}
	c.loaded[pkg.PkgPath] = pkg
	for _, imp := range pkg.Imports {
		c.index(imp)
	}
}

// check reports the constants of pkg that are values of an enum but none of
// its constants.
func (c *checker) check(pkg *packages.Package) {
	for _, f := range pkg.Syntax {
		ast.PreorderStack(f, nil, func(n ast.Node, stack []ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			if c.constant(pkg, e, stack[len(stack)-1]) {
				return false
			}
			if call, ok := e.(*ast.CallExpr); ok {
				c.constructed(pkg, call)
			}
			return true
		})
	}
}

// constant reports whether e is a constant of an enum, which what it is made
// of has no more to tell about, and reports it if it is none of the enum's
// constants.
//
// Where e is ordered against another string, or joined to one, it need not be
// a value of the enum, so it is let through. So is the empty string, the zero
// value, which a program may write to tell or to reset a value unset.
func (c *checker) constant(pkg *packages.Package, e ast.Expr, parent ast.Node) bool {
	tv := pkg.TypesInfo.Types[e]
	if tv.Value == nil || tv.Value.Kind() != constant.String {
		return false
	}
	en := c.enumOf(tv.Type)
	if en == nil {
		return false
	}
	// A constant of the enum's type is told of where its value is written.
	if k, ok := pkg.TypesInfo.Uses[ident(e)].(*types.Const); ok && types.Identical(k.Type(), en.named) {
		return true
	}
	switch p := parent.(type) {
	case *ast.BinaryExpr:
		switch p.Op { //nolint:exhaustive // the operators taking strings but no values of the enum
		case token.ADD, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return true
		}
	case *ast.AssignStmt:
		if p.Tok == token.ADD_ASSIGN {
			return true
		}
	}
	if v := constant.StringVal(tv.Value); v != "" && !en.holds(v) {
		c.report(pkg, e, v, en)
	}
	return true
}

// constructed reports call if it calls the constructor kanna-value generated
// for an enum with a constant that is none of the enum's, which it rejects
// every time.
func (c *checker) constructed(pkg *packages.Package, call *ast.CallExpr) {
	fn, ok := pkg.TypesInfo.Uses[ident(call.Fun)].(*types.Func)
	if !ok || len(call.Args) != 1 {
		return
	}
	sig := fn.Signature()
	if sig.Recv() != nil || sig.Results().Len() != 2 {
		return
	}
	en := c.enumOf(sig.Results().At(0).Type())
	if en == nil || fn.Pkg() != en.named.Obj().Pkg() {
		return
	}
	if newFunc, _ := enumtype.Funcs(en.named.Obj().Name()); fn.Name() != newFunc || !c.generated(fn) {
		return
	}
	arg := call.Args[0]
	tv := pkg.TypesInfo.Types[arg]
	if tv.Value == nil || tv.Value.Kind() != constant.String {
		return
	}
	if v := constant.StringVal(tv.Value); !en.holds(v) {
		c.report(pkg, arg, v, en)
	}
}

// generated reports whether fn is declared in a file kanna-value wrote.
func (c *checker) generated(fn *types.Func) bool {
	pkg := c.loaded[fn.Pkg().Path()]
	if pkg == nil {
		return false
	}
	for _, f := range pkg.Syntax {
		if f.FileStart <= fn.Pos() && fn.Pos() < f.FileEnd {
			return gofile.GeneratedBy(f, generator)
		}
	}
	return false
}

// ident returns the identifier e names something by, or nil.
func ident(e ast.Expr) *ast.Ident {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}
	return nil
}

// enumOf returns t as an enum, or nil if it is none.
func (c *checker) enumOf(t types.Type) *enum {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.TypeParams().Len() > 0 || named.Obj().Pkg() == nil ||
		!types.Identical(named.Underlying(), types.Typ[types.String]) {
		return nil
	}
	obj := named.Obj()
	if en, ok := c.enums[obj]; ok {
		return en
	}
	var en *enum
	if c.isMarked(obj) {
		en = &enum{named: named, consts: enumtype.Constants(named, c.fset)}
	}
	c.enums[obj] = en
	return en
}

// isMarked reports whether obj is declared with //kanna:enum.
func (c *checker) isMarked(obj *types.TypeName) bool {
	path := obj.Pkg().Path()
	marked, ok := c.marked[path]
	if !ok {
		marked = map[*types.TypeName]bool{}
		if pkg := c.loaded[path]; pkg != nil {
			for _, t := range scan.Types([]*packages.Package{pkg}) {
				if d, _ := directive.Find(c.fset, t.Doc, enumKey); d.Found {
					marked[t.Named.Obj()] = true
				}
			}
		}
		c.marked[path] = marked
	}
	return marked[obj]
}

// report reports v, at e in pkg, as none of the constants of en.
func (c *checker) report(pkg *packages.Package, e ast.Expr, v string, en *enum) {
	qualifier := func(p *types.Package) string {
		if p == pkg.Types {
			return ""
		}
		return p.Name()
	}
	d := diag.Errorf(c.fset.Position(e.Pos()), "%q is none of the constants of %s",
		v, types.TypeString(en.named, qualifier))

	// Only a constant pkg can name is worth suggesting.
	var named []*types.Const
	var values []string
	for _, k := range en.consts {
		if k.Exported() || k.Pkg() == pkg.Types {
			named = append(named, k)
			values = append(values, constant.StringVal(k.Val()))
		}
	}
	// A value differing in case alone is suggested however many letters differ.
	i := slices.IndexFunc(values, func(s string) bool { return strings.EqualFold(s, v) })
	if i < 0 {
		if closest, ok := suggest.Closest(v, values); ok {
			i = slices.Index(values, closest)
		}
	}
	if i >= 0 {
		k := named[i]
		name := k.Name()
		if q := qualifier(k.Pkg()); q != "" {
			name = q + "." + name
		}
		d = d.WithHints("did you mean " + name + "?")
	}
	c.diags = append(c.diags, d)
}
