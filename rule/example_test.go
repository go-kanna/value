package rule_test

import (
	"fmt"

	"github.com/go-kanna/value/rule"
)

func ExampleEmail() {
	fmt.Println(rule.Email("a@example.com"))
	fmt.Println(rule.Email("Kanna <a@example.com>"))
	// Output:
	// <nil>
	// rule: email
}
