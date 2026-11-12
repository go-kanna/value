package gen

import (
	"go/types"
	"maps"
	"path/filepath"
	"slices"

	"github.com/go-kanna/internal/deps"
	"github.com/go-kanna/internal/directive"
	"github.com/go-kanna/internal/enumtype"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
	"github.com/go-kanna/internal/valuetype"
)

// reportDeps tells what a run in dir reads and writes. It takes from the packages
// only the types written by hand, and what their fields hold, so what another
// generator writes there is none of its input. Where the run writes follows from
// the directives alone, so it is told even when a type does not resolve yet. So
// are the names the declarations fix; the methods that follow from the value
// held wait for its type.
func reportDeps(dir string, cfg config, patterns []string) (deps.Report, error) {
	set, err := packages.Load(patterns, packages.Config{Dir: dir, Tags: cfg.tags, Regenerated: []string{outputFile}})
	if err != nil {
		return deps.Report{}, err //nolint:wrapcheck // it says what could not be loaded
	}

	declares := written(set)
	r := deps.Report{Packages: set.Paths(), SourcesOnly: true}
	for _, s := range scan.Structs(set.Packages) {
		if d, _ := directive.Find(set.Fset, s.Doc, directiveKey); d.Found {
			r.Writes = append(r.Writes, filepath.Join(filepath.Dir(s.Pos.Filename), outputFile))
			newFunc, mustFunc := valuetype.Constructors(s.Name)
			declares[s.PkgPath+"."+newFunc], declares[s.PkgPath+"."+mustFunc] = true, true
			for _, m := range jsonMethods {
				declares[s.PkgPath+"."+s.Name+"."+m] = true
			}
		}
	}
	for _, t := range scan.Types(set.Packages) {
		switch t.Named.Underlying().(type) {
		case *types.Struct, *types.Interface:
			continue
		}
		if d, _ := directive.Find(set.Fset, t.Doc, enumKey); d.Found {
			r.Writes = append(r.Writes, filepath.Join(filepath.Dir(t.Pos.Filename), outputFile))
			newFunc, valuesFunc := enumtype.Funcs(t.Name)
			declares[t.PkgPath+"."+newFunc], declares[t.PkgPath+"."+valuesFunc] = true, true
			for _, m := range enumMethods {
				declares[t.PkgPath+"."+t.Name+"."+m] = true
			}
		}
	}
	r.Declares = slices.Collect(maps.Keys(declares))
	return r, nil
}
