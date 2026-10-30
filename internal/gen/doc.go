// Package gen generates value types from structs marked //kanna:value, and
// enums from string types marked //kanna:enum. cmd/kanna-value runs it.
//
// A marked struct holds its value in one unexported field and declares the rule
// as a validate method. The generated code builds, decodes, and scans values
// only through that method, so a value from outside the package has always
// been accepted by it.
//
// A marked string type takes the values of the constants declared beside it.
// The generated code decodes and scans only those, so a value from outside the
// package is always one of them.
package gen
