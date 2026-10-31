package gen

import (
	"maps"
	"path/filepath"
	"slices"

	"github.com/go-kanna/internal/deps"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/scan"
)

// reportDeps tells what a run in dir reads and writes. It takes from the packages
// only the types written by hand, and what their fields hold, so what another
// generator writes there is none of its input.
func reportDeps(dir string, cfg config, patterns []string) (deps.Report, error) {
	set, err := packages.Load(patterns, packages.Config{Dir: dir, Tags: cfg.tags, Regenerated: []string{outputFile}})
	if err != nil {
		return deps.Report{}, err //nolint:wrapcheck // it says what could not be loaded
	}

	r := deps.Report{
		Packages:    set.Paths(),
		SourcesOnly: true,
		Declares:    slices.Collect(maps.Keys(written(set))),
	}
	found, _ := targets(set.Fset, scan.Structs(set.Packages))
	for _, t := range found {
		r.Writes = append(r.Writes, filepath.Join(filepath.Dir(t.Pos.Filename), outputFile))
	}
	marked, _ := enums(set.Fset, scan.Types(set.Packages))
	for _, e := range marked {
		r.Writes = append(r.Writes, filepath.Join(filepath.Dir(e.Pos.Filename), outputFile))
	}
	return r, nil
}
