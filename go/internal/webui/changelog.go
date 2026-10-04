package webui

import (
	"os"
	"regexp"
	"strings"
)

// changelogPath is the repo's CHANGELOG.md, COPY'd into the image (the file
// lives outside the Go module, so it cannot be embedded). Read on each Settings
// page load — it is small, and a fork's own changelog just replaces the file.
const changelogPath = "/usr/share/mdl-demo/CHANGELOG.md"

// release is one "## [version] - date" block of the changelog.
type release struct {
	Version string
	Date    string
	Groups  []changeGroup
}

// changeGroup is one "### Added"/"### Fixed"/… list within a release.
type changeGroup struct {
	Title string
	Items [][]changeSpan
}

// changeSpan is a run of one list item: plain text, `code`, or a [link](url).
// Kept as data so html/template escapes every piece — nothing is injected raw.
type changeSpan struct {
	Text string
	Code bool
	URL  string
}

// loadChangelog returns the baked changelog's releases, newest first; nil when
// the file is missing (a native dev build) — the card then stays hidden.
func loadChangelog() []release {
	b, err := os.ReadFile(changelogPath)
	if err != nil {
		return nil
	}
	return parseChangelog(string(b))
}

var (
	releaseRe = regexp.MustCompile(`^## \[([^\]]+)\](?:\s*-\s*(.+))?$`)
	spanRe    = regexp.MustCompile("`([^`]+)`|\\[([^\\]]+)\\]\\((https?://[^)\\s]+)\\)")
)

// parseChangelog reads the Keep a Changelog subset CHANGELOG.md is written in:
// release headings, group headings and one-line bullets. Everything else (the
// preamble, blank lines) is skipped.
func parseChangelog(md string) []release {
	var out []release
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if m := releaseRe.FindStringSubmatch(line); m != nil {
			out = append(out, release{Version: m[1], Date: strings.TrimSpace(m[2])})
			continue
		}
		if len(out) == 0 {
			continue
		}
		r := &out[len(out)-1]
		if title, ok := strings.CutPrefix(line, "### "); ok {
			r.Groups = append(r.Groups, changeGroup{Title: strings.TrimSpace(title)})
			continue
		}
		if item, ok := strings.CutPrefix(line, "- "); ok {
			if len(r.Groups) == 0 {
				r.Groups = append(r.Groups, changeGroup{})
			}
			g := &r.Groups[len(r.Groups)-1]
			g.Items = append(g.Items, changeSpans(item))
		}
	}
	return out
}

func changeSpans(s string) []changeSpan {
	var out []changeSpan
	pos := 0
	for _, m := range spanRe.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > pos {
			out = append(out, changeSpan{Text: s[pos:m[0]]})
		}
		if m[2] >= 0 {
			out = append(out, changeSpan{Text: s[m[2]:m[3]], Code: true})
		} else {
			out = append(out, changeSpan{Text: s[m[4]:m[5]], URL: s[m[6]:m[7]]})
		}
		pos = m[1]
	}
	if pos < len(s) {
		out = append(out, changeSpan{Text: s[pos:]})
	}
	return out
}
