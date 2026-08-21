package site

import "testing"

func TestNormalizeURL(t *testing.T) {
	good := map[string]string{
		"http://localhost:8082":                  "http://localhost:8082",
		"  https://site.mdl-demo.201.mpd.test/ ": "https://site.mdl-demo.201.mpd.test",
		"https://abc.trycloudflare.com/moodle/":  "https://abc.trycloudflare.com/moodle",
	}
	for in, want := range good {
		got, err := NormalizeURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "localhost:8082", "ftp://x", "http://", "not a url"} {
		if got, err := NormalizeURL(in); err == nil {
			t.Errorf("NormalizeURL(%q) = %q, want error", in, got)
		}
	}
}
