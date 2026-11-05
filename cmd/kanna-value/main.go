// Command kanna-value generates value types from structs marked
// //kanna:value, and enums from string types marked //kanna:enum. kanna-value
// vet finds the constants given to an enum that are none of its constants.
package main

import (
	"os"

	"github.com/go-kanna/value/internal/gen"
	"github.com/go-kanna/value/internal/vet"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "vet" {
		os.Exit(vet.Main(os.Args[2:]))
	}
	os.Exit(gen.Command().Main(os.Args[1:]))
}
