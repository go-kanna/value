package gen

import (
	"flag"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-kanna/internal/cli"
	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/output"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
)

// config holds what kanna-value's own flags set.
type config struct {
	tags string
}

// Command returns kanna-value.
func Command() cli.Command[config] {
	return cli.Command[config]{
		Name:    generator,
		Summary: "generate value types from structs marked //kanna:" + directiveKey,
		Args:    "[packages]",
		Help: "The packages default to the one in the current directory, so that\n" +
			"//go:generate go tool " + generator + " needs no argument.",
		Flags: func(fs *flag.FlagSet, cfg *config) {
			fs.StringVar(&cfg.tags, "tags", "", "comma-separated build tags")
		},
		Run: run,
	}
}

func run(dir string, cfg config, patterns []string) ([]output.Change, []diag.Diag) {
	set, err := packages.Load(patterns, packages.Config{Dir: dir, Tags: cfg.tags, Regenerated: []string{outputFile}})
	if err != nil {
		return nil, []diag.Diag{diag.Errorf(token.Position{}, "%v", err)}
	}
	// Code calling a constructor or a method of a value type does not compile
	// before the run writes it, nor does code calling it as the run is about to
	// change it. So a type error of a package loaded does not stop the run
	// here: packages.Recheck tells, once the output is in place, whether it
	// remains.
	blocking, deferred := set.Split()
	if diag.HasErrors(blocking) {
		return nil, append(blocking, set.SetAsideWarnings()...)
	}

	changes, diags := generate(set)
	if diag.HasErrors(diags) {
		return nil, packages.Stopped(deferred, diags, set.Pending(written(set)))
	}
	if rds := packages.Recheck(set, changes); len(rds) > 0 {
		return nil, slices.Concat(diags, rds, set.SetAsideWarnings())
	}
	if !slices.ContainsFunc(changes, func(c output.Change) bool { return !c.Remove }) {
		// Nothing to stop for: a go:generate line left behind does no harm. The
		// packages are named by path, as go generate ./... runs every one of
		// them in a "." of its own.
		var in []string
		for _, pkg := range set.Packages {
			in = append(in, pkg.PkgPath)
		}
		if len(in) == 0 {
			in = patterns // which matched no package
		}
		diags = append(diags, diag.Warningf(token.Position{},
			"no struct is marked //kanna:%s in %s", directiveKey, strings.Join(in, ", ")))
	}
	return changes, diags
}

// written returns what the run writes for the value types of set, by full
// name: their constructors, and their methods.
func written(set packages.Set) map[string]bool {
	found, _ := targets(set.Fset, scan.Structs(set.Packages))
	names := make(map[string]bool)
	for _, t := range found {
		names[t.PkgPath+"."+t.newFunc], names[t.PkgPath+"."+t.mustFunc] = true, true
		for _, m := range append(generatedMethods(t.field.Type), t.accessor) {
			names[t.PkgPath+"."+t.Name+"."+m] = true
		}
	}
	return names
}

// generate works out the changes for the packages of set: the output of each
// package with targets, and the removal of what an earlier run left in a
// package without any.
func generate(set packages.Set) ([]output.Change, []diag.Diag) {
	found, diags := targets(set.Fset, scan.Structs(set.Packages))
	if diag.HasErrors(diags) {
		return nil, diags
	}

	byPkg := make(map[string][]target)
	for _, t := range found {
		byPkg[t.PkgPath] = append(byPkg[t.PkgPath], t)
	}

	var changes []output.Change
	for _, pkg := range set.Packages {
		if len(pkg.GoFiles) == 0 {
			continue
		}
		dir := filepath.Dir(pkg.GoFiles[0])
		path := filepath.Join(dir, outputFile)

		pkgTargets := byPkg[pkg.PkgPath]
		if len(pkgTargets) == 0 {
			// A package whose structs have all lost the directive keeps the
			// types an earlier run generated unless the file goes as well.
			if packages.GeneratedFiles(pkg)[path] {
				changes = append(changes, output.Change{Path: path, Remove: true})
			}
			continue
		}

		declared := slices.Sorted(maps.Keys(output.DeclaredNames(dir, outputFile, pkg.Name)))
		f := emit(pkg.PkgPath, pkg.Name, pkgTargets, declared)
		src, err := f.Bytes()
		if err != nil {
			return nil, append(diags, diag.Errorf(token.Position{Filename: path}, "generated code does not parse: %v", err))
		}
		changes = append(changes, output.Change{Path: path, Data: src})
		diags = append(diags, output.MissingRequires(path, f.ModuleImports())...)
	}
	return changes, diags
}
