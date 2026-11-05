package vet

import "github.com/go-kanna/internal/cli"

// Command returns kanna-value vet.
func Command() cli.Vet {
	return cli.Vet{
		Name:    "kanna-value",
		Summary: "find the constants given to an enum that are none of its constants",
		Run:     Run,
	}
}
