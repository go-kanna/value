// Command kanna-value generates value types from structs marked
// //kanna:value, and enums from string types marked //kanna:enum.
package main

import (
	"os"

	"github.com/go-kanna/value/internal/gen"
)

func main() {
	os.Exit(gen.Command().Main(os.Args[1:]))
}
