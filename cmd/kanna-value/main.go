// Command kanna-value generates value types from structs marked
// //kanna:value.
package main

import (
	"os"

	"github.com/go-kanna/value/internal/gen"
)

func main() {
	os.Exit(gen.Command().Main(os.Args[1:]))
}
