package gen

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/go-kanna/internal/gofile"
)

// generator names the tool in the header of the file it writes.
const generator = "kanna-value"

// emit renders the value types of targets into a file of the package pkgName
// at pkgPath. declared are the top-level names the package declares outside
// the file, which nothing the file introduces may take.
func emit(pkgPath, pkgName string, targets []target, declared []string) *gofile.File {
	f := gofile.New(generator, pkgPath, pkgName)
	f.Reserve(declared...)
	for _, t := range targets {
		f.Reserve(t.newFunc, t.mustFunc)
	}

	for _, t := range targets {
		w := writer{
			f:       f,
			target:  t,
			pkgName: pkgName,
			typ:     types.TypeString(t.field.Type, f.Qualifier),
			desc:    typeString(t.Struct, t.field.Type),
		}
		w.writeNew()
		w.writeMust()
		w.writeAccessor()
		if t.accessor != "String" {
			w.writeString()
		}
		w.writeIsZero()
		if t.hasEqual {
			w.writeEqual()
		}
		w.writeJSON()
		w.writeSQL()
	}
	return f
}

// writer writes the declarations of one value type.
type writer struct {
	target

	f *gofile.File

	// pkgName starts the errors the generated code reports itself.
	pkgName string

	// typ spells the type of the value as the file refers to it, and desc as a
	// reader of the package would.
	typ, desc string
}

func (w writer) writeNew() {
	sc := w.f.Scope()
	v, err := sc.Name("v"), sc.Name("err")

	copied := v
	if w.clone != "" {
		copied = "a copy of " + v
	}
	w.doc(fmt.Sprintf("%s returns the %s holding %s if the validate method of %s accepts it, "+
		"and the method's error otherwise.", w.newFunc, w.Name, copied, w.Name))
	fmt.Fprintf(w.f, "func %s(%s %s) (%s, error) {\n", w.newFunc, v, w.typ, w.Name)
	if w.clone != "" {
		fmt.Fprintf(w.f, "%s = %s.Clone(%s)\n", v, w.f.Import(w.clone, w.clone), v)
	}
	fmt.Fprintf(w.f, "if %s := (%s{}).validate(%s); %s != nil {\n", err, w.Name, v, err)
	fmt.Fprintf(w.f, "return %s{}, %s\n", w.Name, err)
	fmt.Fprintf(w.f, "}\n")
	fmt.Fprintf(w.f, "return %s{%s: %s}, nil\n", w.Name, w.field.Name, v)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeMust() {
	sc := w.f.Scope()
	v, x, err := sc.Name("v"), sc.Name("x"), sc.Name("err")

	w.doc(fmt.Sprintf("%s is %s for values known to be valid, such as constants. It panics if %s is rejected.",
		w.mustFunc, w.newFunc, v))
	fmt.Fprintf(w.f, "func %s(%s %s) %s {\n", w.mustFunc, v, w.typ, w.Name)
	fmt.Fprintf(w.f, "%s, %s := %s(%s)\n", x, err, w.newFunc, v)
	fmt.Fprintf(w.f, "if %s != nil {\n", err)
	fmt.Fprintf(w.f, "panic(%s)\n", err)
	fmt.Fprintf(w.f, "}\n")
	fmt.Fprintf(w.f, "return %s\n", x)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeAccessor() {
	x := w.f.Scope().Name("x")

	value := x + "." + w.field.Name
	what := "the " + w.desc
	if w.clone != "" {
		value = w.f.Import(w.clone, w.clone) + ".Clone(" + value + ")"
		what = "a copy of the " + w.desc
	}
	w.doc(fmt.Sprintf("%s returns %s that %s holds.", w.accessor, what, x))
	fmt.Fprintf(w.f, "func (%s %s) %s() %s {\n", x, w.Name, w.accessor, w.typ)
	fmt.Fprintf(w.f, "return %s\n", value)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeString() {
	x := w.f.Scope().Name("x")

	value := x + "." + w.field.Name + ".String()"
	if !w.hasString {
		value = w.f.Import("fmt", "fmt") + ".Sprint(" + x + "." + w.field.Name + ")"
	}
	w.doc(fmt.Sprintf("String formats the %s that %s holds as fmt's %%v does.", w.desc, x))
	fmt.Fprintf(w.f, "func (%s %s) String() string {\n", x, w.Name)
	fmt.Fprintf(w.f, "return %s\n", value)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeIsZero() {
	x := w.f.Scope().Name("x")

	var test string
	switch w.zero {
	case zeroMethod:
		test = x + "." + w.field.Name + ".IsZero()"
	case zeroCompare:
		test = x + " == " + w.Name + "{}"
	case zeroNil:
		test = x + "." + w.field.Name + " == nil"
	case zeroNone:
		return
	}
	w.doc(fmt.Sprintf("IsZero reports whether %s holds the zero value of %s, as the zero %s does.", x, w.desc, w.Name))
	fmt.Fprintf(w.f, "func (%s %s) IsZero() bool {\n", x, w.Name)
	fmt.Fprintf(w.f, "return %s\n", test)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeEqual() {
	sc := w.f.Scope()
	x, y := sc.Name("x"), sc.Name("y")

	w.doc(fmt.Sprintf("Equal reports whether %s and %s hold equal values of %s, as the Equal method of %s tells.",
		x, y, w.desc, w.desc))
	fmt.Fprintf(w.f, "func (%s %s) Equal(%s %s) bool {\n", x, w.Name, y, w.Name)
	fmt.Fprintf(w.f, "return %s.%s.Equal(%s.%s)\n", x, w.field.Name, y, w.field.Name)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeJSON() {
	json := w.f.Import("encoding/json", "json")

	x := w.f.Scope().Name("x")
	w.doc(fmt.Sprintf("MarshalJSON encodes the %s that %s holds as encoding/json encodes it.", w.desc, x))
	fmt.Fprintf(w.f, "func (%s %s) MarshalJSON() ([]byte, error) {\n", x, w.Name)
	fmt.Fprintf(w.f, "return %s.Marshal(%s.%s)\n", json, x, w.field.Name)
	fmt.Fprintf(w.f, "}\n\n")

	sc := w.f.Scope()
	x, data, v, y, err := sc.Name("x"), sc.Name("data"), sc.Name("v"), sc.Name("y"), sc.Name("err")
	w.doc(fmt.Sprintf("UnmarshalJSON decodes %s as encoding/json decodes a value of %s, and keeps the value "+
		"only if the validate method of %s accepts it. JSON null, like an error, leaves %s as it was.",
		data, w.desc, w.Name, x))
	fmt.Fprintf(w.f, "func (%s *%s) UnmarshalJSON(%s []byte) error {\n", x, w.Name, data)
	fmt.Fprintf(w.f, "if string(%s) == \"null\" {\n", data)
	fmt.Fprintf(w.f, "return nil\n")
	fmt.Fprintf(w.f, "}\n")
	fmt.Fprintf(w.f, "var %s %s\n", v, w.typ)
	fmt.Fprintf(w.f, "if %s := %s.Unmarshal(%s, &%s); %s != nil {\n", err, json, data, v, err)
	fmt.Fprintf(w.f, "return %s\n", err)
	fmt.Fprintf(w.f, "}\n")
	w.keep(x, y, err, v)
	fmt.Fprintf(w.f, "}\n\n")
}

func (w writer) writeSQL() {
	driver := w.f.Import("database/sql/driver", "driver")

	x := w.f.Scope().Name("x")
	w.doc(fmt.Sprintf("Value returns the %s that %s holds for a database driver, converted as "+
		"database/sql converts an argument of %s.", w.desc, x, w.desc))
	fmt.Fprintf(w.f, "func (%s %s) Value() (%s.Value, error) {\n", x, w.Name, driver)
	fmt.Fprintf(w.f, "return %s.DefaultParameterConverter.ConvertValue(%s.%s)\n", driver, x, w.field.Name)
	fmt.Fprintf(w.f, "}\n\n")

	sc := w.f.Scope()
	x, src, v, y, err := sc.Name("x"), sc.Name("src"), sc.Name("v"), sc.Name("y"), sc.Name("err")
	w.doc(fmt.Sprintf("Scan reads a value of %s from a database column, and keeps it only if the validate "+
		"method of %s accepts it. On error %s is left as it was. NULL is an error: scan a column "+
		"that may be NULL into a pointer to %s.", w.desc, w.Name, x, w.Name))
	fmt.Fprintf(w.f, "func (%s *%s) Scan(%s any) error {\n", x, w.Name, src)
	fmt.Fprintf(w.f, "var %s %s.Null[%s]\n", v, w.f.Import("database/sql", "sql"), w.typ)
	fmt.Fprintf(w.f, "if %s := %s.Scan(%s); %s != nil {\n", err, v, src, err)
	fmt.Fprintf(w.f, "return %s\n", err)
	fmt.Fprintf(w.f, "}\n")
	fmt.Fprintf(w.f, "if !%s.Valid {\n", v)
	fmt.Fprintf(w.f, "return %s.New(%q)\n", w.f.Import("errors", "errors"),
		w.pkgName+": cannot scan NULL into "+w.Name+"; scan a column that may be NULL into *"+w.Name)
	fmt.Fprintf(w.f, "}\n")
	w.keep(x, y, err, v+".V")
	fmt.Fprintf(w.f, "}\n\n")
}

// doc writes text as a comment, wrapped at 80 columns.
func (w writer) doc(text string) {
	line := "//"
	for word := range strings.FieldsSeq(text) {
		if len(line)+1+len(word) > 80 && line != "//" {
			fmt.Fprintln(w.f, line)
			line = "//"
		}
		line += " " + word
	}
	fmt.Fprintln(w.f, line)
}

// keep ends a method on the pointer x that has decoded value: it builds y from
// value through the constructor, and keeps it in x unless the constructor
// returns err.
func (w writer) keep(x, y, err, value string) {
	fmt.Fprintf(w.f, "%s, %s := %s(%s)\n", y, err, w.newFunc, value)
	fmt.Fprintf(w.f, "if %s != nil {\n", err)
	fmt.Fprintf(w.f, "return %s\n", err)
	fmt.Fprintf(w.f, "}\n")
	fmt.Fprintf(w.f, "*%s = %s\n", x, y)
	fmt.Fprintf(w.f, "return nil\n")
}
