// Package camp loads and queries the baked Camp registry catalogue — the
// community plugin index from camp-registry.org (the git repo camp-index),
// cloned into /srv/extra/camp at image build. It is the read side of the Camp
// browse/install feature: parse the ~6,400 plugin YAMLs and the security
// advisories once, then filter/paginate them for the console and look one up by
// component for the installed-plugins dialog.
//
// Unlike internal/recipes (which hand-scans a couple of flat scalars), the Camp
// schema is nested (releases[], metrics{}), so this package uses a real YAML
// parser. The data is ODbL 1.0; the console is a "produced work" (attribution
// only) — the credit lives on the Camp page and in VENDOR.md.
package camp

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// Dir is where the catalogue is baked (see container/Containerfile).
	Dir = "/srv/extra/camp"
	// SiteBase is the public browse site; a plugin's page is SiteBase/plugin/<component>.
	SiteBase = "https://camp-registry.org"
)

// Release is one published version of a plugin, with the Moodle versions it
// supports (dotted, e.g. "5.2"). Only camp-CI-published entries carry releases.
type Release struct {
	Version         string   `yaml:"version"`
	Tag             string   `yaml:"tag"`
	MoodleVersion   int      `yaml:"moodle-version"`
	SupportedMoodle []string `yaml:"supported-moodle"`
}

// Plugin is one catalogue entry. Type is derived from the directory, everything
// else from the YAML.
type Plugin struct {
	Component string
	Type      string
	Source    string
	Summary   string
	Status    string
	License   string
	Tier      int
	Stars     int
	Archived  bool
	Labels    []string
	Releases  []Release
}

// URL is the plugin's page on the public browse site (for templates).
func (p Plugin) URL() string { return SiteBase + "/plugin/" + p.Component }

// pluginFile mirrors the on-disk YAML shape; mapped into Plugin on load.
type pluginFile struct {
	Component string    `yaml:"component"`
	Source    string    `yaml:"source"`
	Summary   string    `yaml:"summary"`
	Status    string    `yaml:"status"`
	License   string    `yaml:"license"`
	Tier      int       `yaml:"tier"`
	Labels    []string  `yaml:"labels"`
	Releases  []Release `yaml:"releases"`
	Metrics   struct {
		Stars    int  `yaml:"stars"`
		Archived bool `yaml:"archived"`
	} `yaml:"metrics"`
}

// Advisory is one security advisory, keyed to a plugin by Component.
type Advisory struct {
	ID               string `yaml:"id"`
	Component        string `yaml:"component"`
	Title            string `yaml:"title"`
	Severity         string `yaml:"severity"`
	AffectedVersions string `yaml:"affected-versions"`
	FixedIn          string `yaml:"fixed-in"`
	Revoke           bool   `yaml:"revoke"`
}

// Catalog is the loaded, queryable index.
type Catalog struct {
	plugins     []Plugin
	byComponent map[string]*Plugin
	advisories  map[string][]Advisory
	types       []string
}

// Query selects and paginates a slice of the catalogue.
type Query struct {
	Text    string // case-insensitive substring of component or summary
	Type    string // plugin type (directory); "" = any
	Status  string // status; "" = any
	MinTier int    // minimum tier (0 = any; 1 = claimed+; 2 = verified)
	Page    int    // 1-based
	PerPage int
}

// Load reads the whole catalogue from dir. Plugin files live at
// plugins/<type>/<component>.yml; advisories at advisories/*.yml.
func Load(dir string) (*Catalog, error) {
	c := &Catalog{byComponent: map[string]*Plugin{}, advisories: map[string][]Advisory{}}

	files, err := filepath.Glob(filepath.Join(dir, "plugins", "*", "*.yml"))
	if err != nil {
		return nil, err
	}
	typeSet := map[string]struct{}{}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			continue // a single unreadable entry must not sink the catalogue
		}
		var pf pluginFile
		if yaml.Unmarshal(b, &pf) != nil || pf.Component == "" {
			continue
		}
		c.plugins = append(c.plugins, Plugin{
			Component: pf.Component,
			Type:      filepath.Base(filepath.Dir(path)),
			Source:    pf.Source,
			Summary:   pf.Summary,
			Status:    pf.Status,
			License:   pf.License,
			Tier:      pf.Tier,
			Stars:     pf.Metrics.Stars,
			Archived:  pf.Metrics.Archived,
			Labels:    pf.Labels,
			Releases:  pf.Releases,
		})
		typeSet[filepath.Base(filepath.Dir(path))] = struct{}{}
	}
	// Default order: most-starred first, then component — a sensible unfiltered
	// landing view.
	sort.Slice(c.plugins, func(i, j int) bool {
		if c.plugins[i].Stars != c.plugins[j].Stars {
			return c.plugins[i].Stars > c.plugins[j].Stars
		}
		return c.plugins[i].Component < c.plugins[j].Component
	})
	for i := range c.plugins {
		c.byComponent[c.plugins[i].Component] = &c.plugins[i]
	}
	for t := range typeSet {
		c.types = append(c.types, t)
	}
	sort.Strings(c.types)

	adv, _ := filepath.Glob(filepath.Join(dir, "advisories", "*.yml"))
	for _, path := range adv {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var a Advisory
		if yaml.Unmarshal(b, &a) != nil || a.Component == "" {
			continue
		}
		c.advisories[a.Component] = append(c.advisories[a.Component], a)
	}
	return c, nil
}

// Filter returns one page of matching plugins and the total match count.
func (c *Catalog) Filter(q Query) (page []Plugin, total int) {
	text := strings.ToLower(strings.TrimSpace(q.Text))
	var matches []Plugin
	for _, p := range c.plugins {
		if q.Type != "" && p.Type != q.Type {
			continue
		}
		if q.Status != "" && p.Status != q.Status {
			continue
		}
		if q.MinTier > 0 && p.Tier < q.MinTier {
			continue
		}
		if text != "" &&
			!strings.Contains(strings.ToLower(p.Component), text) &&
			!strings.Contains(strings.ToLower(p.Summary), text) {
			continue
		}
		matches = append(matches, p)
	}
	total = len(matches)
	if q.PerPage <= 0 {
		return matches, total
	}
	start := (q.Page - 1) * q.PerPage
	if start < 0 || start >= total {
		return nil, total
	}
	end := start + q.PerPage
	if end > total {
		end = total
	}
	return matches[start:end], total
}

// Get returns the plugin with the given component.
func (c *Catalog) Get(component string) (*Plugin, bool) {
	p, ok := c.byComponent[component]
	return p, ok
}

// AdvisoriesFor returns the live (non-revoked) advisories for a component.
func (c *Catalog) AdvisoriesFor(component string) []Advisory {
	var out []Advisory
	for _, a := range c.advisories[component] {
		if !a.Revoke {
			out = append(out, a)
		}
	}
	return out
}

// Types is the sorted set of plugin types present in the catalogue.
func (c *Catalog) Types() []string { return c.types }

// PluginURL is the plugin's page on the public browse site.
func (c *Catalog) PluginURL(component string) string {
	return SiteBase + "/plugin/" + component
}

// MatchingRef proposes a git ref for installing this plugin onto a site whose
// Moodle branch is siteBranch (e.g. "502"): the tag of the newest release
// whose supported-moodle includes the site version. Returns "" when no release
// matches — the caller then falls back to the ls-remote branch proposal.
func (p *Plugin) MatchingRef(siteBranch string) string {
	want := dottedMoodle(siteBranch)
	if want == "" {
		return ""
	}
	best, bestVer := "", -1
	for _, r := range p.Releases {
		for _, m := range r.SupportedMoodle {
			if m == want && r.MoodleVersion > bestVer {
				best, bestVer = r.Tag, r.MoodleVersion
			}
		}
	}
	return best
}

// dottedMoodle turns a numeric branch ("502", "405") into the dotted form Camp
// uses in supported-moodle ("5.2", "4.5").
func dottedMoodle(branch string) string {
	branch = strings.TrimSpace(branch)
	if len(branch) < 2 {
		return ""
	}
	minor, err := strconv.Atoi(branch[1:])
	if err != nil {
		return ""
	}
	return branch[:1] + "." + strconv.Itoa(minor)
}
