package vet_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kanna/internal/cli"

	"github.com/go-kanna/value/internal/vet"
)

func TestCommand(t *testing.T) {
	t.Parallel()

	dir := module(t, model, map[string]string{
		"clean/clean.go": "package clean\n",
		"usecase/usecase.go": `package usecase

import "example.com/m/model"

func Closed(s model.Status) bool { return s == "clsoed" }
`,
		"tagged/tagged.go": `//go:build extra

package tagged

import "example.com/m/model"

var _ model.Status = "opne"
`,
	})
	const usecaseFinding = `error: "clsoed" is none of the constants of model.Status
  did you mean model.StatusClosed?
`
	for _, tt := range []struct {
		dir    string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{filepath.Join(dir, "clean"), nil, cli.ExitOK, "", ""},
		{dir, []string{"./usecase"}, cli.ExitFailure, "", "usecase/usecase.go:5:48: " + usecaseFinding},
		{filepath.Join(dir, "usecase"), nil, cli.ExitFailure, "", "./usecase.go:5:48: " + usecaseFinding},
		{dir, []string{"./clean", "./tagged"}, cli.ExitFailure, "", ""},
		{dir, []string{"-tags", "extra", "./tagged"}, cli.ExitFailure, "",
			"tagged/tagged.go:7:22: error: \"opne\" is none of the constants of model.Status\n" +
				"  did you mean model.StatusOpen?\n"},
		{dir, []string{"./does-not-exist"}, cli.ExitFailure, "", ""},
		{dir, []string{"-h"}, cli.ExitOK, "kanna-value vet: find the constants given to an enum", ""},
	} {
		var out, errOut bytes.Buffer
		code := vet.Command().Exec(cli.Env{Out: &out, Err: &errOut, Dir: tt.dir}, tt.args)
		if code != tt.code ||
			!strings.HasPrefix(out.String(), tt.stdout) ||
			(tt.stderr != "" && !strings.HasPrefix(errOut.String(), tt.stderr)) {
			t.Errorf("vet %v in %s = %d, want %d\nstdout:\n%s\nstderr:\n%s",
				tt.args, tt.dir, code, tt.code, out.String(), errOut.String())
		}
	}
}
