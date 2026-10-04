package webui

import (
	"os"
	"reflect"
	"testing"
)

func TestParseChangelog(t *testing.T) {
	got := parseChangelog("# Changelog\n\n- preamble bullet\n\n## [Unreleased]\n\n### Changes\n\n" +
		"- Docs point to [the app](https://example.org/app) and `mdl-demo url`\n\n" +
		"## [0.1.0] - 2026-08-18\n\n### Added\n\n- First preview release\n")
	want := []release{
		{Version: "Unreleased", Groups: []changeGroup{{Title: "Changes", Items: [][]changeSpan{{
			{Text: "Docs point to "},
			{Text: "the app", URL: "https://example.org/app"},
			{Text: " and "},
			{Text: "mdl-demo url", Code: true},
		}}}}},
		{Version: "0.1.0", Date: "2026-08-18", Groups: []changeGroup{{Title: "Added", Items: [][]changeSpan{{
			{Text: "First preview release"},
		}}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseChangelog:\n got %+v\nwant %+v", got, want)
	}
}

// The repo's own CHANGELOG.md must stay within the subset the console renders.
func TestRepoChangelogParses(t *testing.T) {
	b, err := os.ReadFile("../../../CHANGELOG.md")
	if err != nil {
		t.Skip(err)
	}
	rs := parseChangelog(string(b))
	if len(rs) == 0 {
		t.Fatal("no releases parsed")
	}
	for _, r := range rs {
		if len(r.Groups) == 0 {
			t.Errorf("release %s has no changes", r.Version)
		}
		for _, g := range r.Groups {
			if g.Title == "" || len(g.Items) == 0 {
				t.Errorf("release %s: empty group %q", r.Version, g.Title)
			}
		}
	}
}
