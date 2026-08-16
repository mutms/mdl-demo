// Package recipes lists the site recipes shipped in the image's catalogue.
//
// Recipes are mudev's YAML files at <Dir>/<vendor>/<stream>/<version>.yaml.
// mdl-demo only needs each recipe's identifier, name and description for the
// picker; mudev itself does all real recipe parsing at clone time. The
// top-level name:/description: keys are flat scalars in every catalogue
// recipe, so a line scan is enough and keeps mdl-demo dependency-free.
package recipes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Dir is mudev's default recipe catalogue location.
const Dir = "/srv/extra/mdl-recipes"

type Recipe struct {
	ID          string // "mutms/release/5.2.2.01" — what mudev clone takes
	Vendor      string
	Stream      string
	Version     string
	Name        string
	Description string
}

// List returns all catalogue recipes, sorted by vendor, stream, then
// version descending (newest first within a stream).
func List() ([]Recipe, error) {
	paths, err := filepath.Glob(filepath.Join(Dir, "*", "*", "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []Recipe
	for _, p := range paths {
		rel, err := filepath.Rel(Dir, p)
		if err != nil {
			continue
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 3 {
			continue
		}
		r := Recipe{
			Vendor:  parts[0],
			Stream:  parts[1],
			Version: strings.TrimSuffix(parts[2], ".yaml"),
		}
		r.ID = r.Vendor + "/" + r.Stream + "/" + r.Version
		r.Name, r.Description = scanHeader(p)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out, nil
}

// Get returns the recipe with the given identifier.
func Get(id string) (Recipe, error) {
	all, err := List()
	if err != nil {
		return Recipe{}, err
	}
	for _, r := range all {
		if r.ID == id {
			return r, nil
		}
	}
	return Recipe{}, fmt.Errorf("unknown recipe %q (see `mdl-demo recipes`)", id)
}

// scanHeader pulls the first top-level name: and description: values.
func scanHeader(path string) (name, description string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if name == "" {
			if v, ok := strings.CutPrefix(line, "name:"); ok {
				name = cleanScalar(v)
			}
		}
		if description == "" {
			if v, ok := strings.CutPrefix(line, "description:"); ok {
				description = cleanScalar(v)
			}
		}
		if name != "" && description != "" {
			break
		}
	}
	return name, description
}

func cleanScalar(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "   #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	v = strings.Trim(v, `"'`)
	return v
}

// less orders vendor asc, stream asc, version desc (numeric-aware).
func less(a, b Recipe) bool {
	if a.Vendor != b.Vendor {
		return a.Vendor < b.Vendor
	}
	if a.Stream != b.Stream {
		return a.Stream < b.Stream
	}
	return compareVersions(a.Version, b.Version) > 0
}

// compareVersions compares dotted versions numerically where possible.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		an, aerr := strconv.Atoi(av)
		bn, berr := strconv.Atoi(bv)
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		default:
			if av != bv {
				if av < bv {
					return -1
				}
				return 1
			}
		}
	}
	return 0
}
