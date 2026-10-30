// Command example builds and decodes the values of a task service, showing
// what kanna-value generates: constructors and JSON methods that let in only
// what each value's validate method accepts, and only the constants of an
// enum.
//
// Regenerate with:
//
//	go generate ./...
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-kanna/value/rule"

	"github.com/go-kanna/value/example/model"
)

func main() {
	// A constant is known to be valid, so Must builds it.
	owner := model.MustEmail("kanna@example.com")
	fmt.Fprintf(os.Stdout, "owner: %s, at %s\n", owner, owner.Domain())

	// Input may not be, so New says why it is rejected.
	if _, err := model.NewEmail("Kanna <kanna@example.com>"); err != nil {
		fmt.Fprintf(os.Stdout, "rejected: %v\n", err)
	}

	// An enum lists its constants, as a form offering them would.
	fmt.Fprintf(os.Stdout, "priorities: %v\n\n", model.PriorityValues())

	bodies := []struct {
		title string
		body  string
	}{
		{
			title: "A valid request",
			body: `{"title":"Write docs","owner":"kanna@example.com","priority":"high","tags":["docs"],` +
				`"completed_at":"2026-10-29T09:00:00Z"}`,
		},
		{
			title: "Too many tags",
			body:  `{"title":"Ship it","owner":"kanna@example.com","priority":"low","tags":["a","b","c","d"]}`,
		},
		{
			title: "Completed before the service began",
			body:  `{"title":"Ship it","owner":"kanna@example.com","priority":"low","completed_at":"1999-12-31T00:00:00Z"}`,
		},
		{
			title: "A priority that is none of the constants",
			body:  `{"title":"Ship it","owner":"kanna@example.com","priority":"urgent"}`,
		},
	}
	for _, b := range bodies {
		fmt.Fprintf(os.Stdout, "=== %s ===\n", b.title)

		var task model.Task
		err := json.Unmarshal([]byte(b.body), &task)
		if rerr, ok := errors.AsType[*rule.Error](err); ok {
			// The code and params stay apart from the text, to key a translated
			// message or an API response.
			fmt.Fprintf(os.Stdout, "rejected: %v; code %q, params %v\n\n", err, rerr.Code(), rerr.Params())
			continue
		}
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(os.Stdout, "%q by %s, %s priority, tagged %v, completed at %v\n\n",
			task.Title, task.Owner, task.Priority, task.Tags, task.CompletedAt)
	}

	// Tags holds a copy, so changing the slice it was built from changes
	// nothing.
	in := []string{"go"}
	tags := model.MustTags(in)
	in[0] = strings.Repeat("x", 30)
	fmt.Fprintf(os.Stdout, "tags: %v\n", tags)
}
