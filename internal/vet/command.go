package vet

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/go-kanna/internal/cli"
	"github.com/go-kanna/internal/diag"
)

const name = "kanna-value vet"

// Main runs kanna-value vet with args, and returns its exit code.
func Main(args []string) int {
	return Exec(cli.Env{Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv}, args)
}

// Exec runs kanna-value vet in env, the way the commands of
// go-kanna/internal/cli run.
func Exec(env cli.Env, args []string) int {
	fset := flag.NewFlagSet(name, flag.ContinueOnError)
	fset.SetOutput(env.Err)
	fset.Usage = func() {}
	var (
		tags    string
		version bool
	)
	cli.Tags(fset, &tags)
	fset.BoolVar(&version, "version", false, "print the version and exit")

	help := len(args) > 0 && args[0] == "help"
	err := fset.Parse(args)
	switch {
	case help || errors.Is(err, flag.ErrHelp):
		usage(env.Out, fset)
		return cli.ExitOK
	case err != nil:
		usage(env.Err, fset)
		return cli.ExitUsage
	case version:
		fmt.Fprintln(env.Out, "kanna-value", buildVersion())
		return cli.ExitOK
	}

	dir, err := filepath.Abs(env.Dir)
	if err != nil {
		fmt.Fprintf(env.Err, "%s: %v\n", name, err)
		return cli.ExitFailure
	}
	patterns := fset.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	ds := Run(dir, tags, patterns)
	diag.Write(env.Err, dir, ds)
	if diag.HasErrors(ds) {
		return cli.ExitFailure
	}
	return cli.ExitOK
}

func usage(w io.Writer, fset *flag.FlagSet) {
	fmt.Fprintf(w, "%s: find the constants given to an enum that are none of its constants\n\n"+
		"Usage:\n  %s [flags] [packages]\n\n"+
		"The packages default to the one in the current directory. A package passing holds no\n"+
		"such constant that vet found; it does not prove there is none.\n\nFlags:\n", name, name)
	fset.SetOutput(w)
	fset.PrintDefaults()
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
