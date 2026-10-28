// Package gen generates value types from structs marked //kanna:value.
// cmd/kanna-value runs it.
//
// A marked struct holds its value in one unexported field and declares the rule
// as a validate method. The generated code builds, decodes, and scans values
// only through that method, so a value from outside the package has always
// been accepted by it.
package gen
