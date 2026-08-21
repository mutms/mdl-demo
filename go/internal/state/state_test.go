package state

import "testing"

func TestIdentityDefaults(t *testing.T) {
	s := &State{}
	if got := s.Port(); got != 8081 {
		t.Fatalf("Port() = %d, want 8081", got)
	}
	if got := s.SitePort(); got != 8082 {
		t.Fatalf("SitePort() = %d, want 8082", got)
	}
	if got := s.ID(); got != "mdl-demo-8081" {
		t.Fatalf("ID() = %q", got)
	}
	if got := s.Title(); got != "mdl-demo-8081" {
		t.Fatalf("Title() = %q", got)
	}
	if got := s.SiteURLFor("localhost"); got != "http://localhost:8082" {
		t.Fatalf("SiteURLFor = %q", got)
	}
}

func TestIdentityCustom(t *testing.T) {
	s := &State{ConsolePort: 7777, Name: "Fancy test demo"}
	if got := s.ID(); got != "mdl-demo-7777" {
		t.Fatalf("ID() = %q", got)
	}
	if got := s.Title(); got != "mdl-demo-7777 · Fancy test demo" {
		t.Fatalf("Title() = %q", got)
	}
	if got := s.ConsoleURLFor("10.0.0.5"); got != "http://10.0.0.5:7777" {
		t.Fatalf("ConsoleURLFor = %q", got)
	}
	if got := s.SiteURLFor("10.0.0.5"); got != "http://10.0.0.5:7778" {
		t.Fatalf("SiteURLFor = %q", got)
	}
}

func TestURLOverrides(t *testing.T) {
	s := &State{ConsolePort: 6381, ConsoleURL: "https://mdl-demo.201.mpd.test", SiteURL: "https://site.mdl-demo.201.mpd.test"}
	if got := s.ConsoleURLFor("localhost"); got != "https://mdl-demo.201.mpd.test" {
		t.Fatalf("ConsoleURLFor = %q", got)
	}
	if got := s.SiteURLFor("localhost"); got != "https://site.mdl-demo.201.mpd.test" {
		t.Fatalf("SiteURLFor = %q", got)
	}
}

func TestParsePort(t *testing.T) {
	var warnings []string
	warn := func(l string) { warnings = append(warnings, l) }
	cases := []struct {
		in   string
		want int
		warn bool
	}{
		{"", 8081, false},
		{" 7777 ", 7777, false},
		{"1", 1, false},
		{"65534", 65534, false},
		{"65535", 8081, true}, // no room for the site port
		{"0", 8081, true},
		{"abc", 8081, true},
		{"80abc", 8081, true},
	}
	for _, c := range cases {
		warnings = nil
		if got := ParsePort(c.in, warn); got != c.want {
			t.Errorf("ParsePort(%q) = %d, want %d", c.in, got, c.want)
		}
		if (len(warnings) > 0) != c.warn {
			t.Errorf("ParsePort(%q) warnings = %v, want warning=%v", c.in, warnings, c.warn)
		}
	}
}

func TestAdoptIdentityOnce(t *testing.T) {
	s := &State{}
	if !s.AdoptIdentity("7777", "  Keynote  ", nil) {
		t.Fatal("first adoption should report a change")
	}
	if s.ConsolePort != 7777 || s.Name != "Keynote" {
		t.Fatalf("adopted %d/%q", s.ConsolePort, s.Name)
	}
	// A second start with a different environment must not rewrite identity.
	if s.AdoptIdentity("9999", "Other", nil) {
		t.Fatal("second adoption should be a no-op")
	}
	if s.ConsolePort != 7777 || s.Name != "Keynote" {
		t.Fatalf("identity rewritten to %d/%q", s.ConsolePort, s.Name)
	}
	// No env at all: port falls back to the default and is still recorded.
	fresh := &State{}
	if !fresh.AdoptIdentity("", "", nil) || fresh.ConsolePort != 8081 || fresh.Name != "" {
		t.Fatalf("default adoption: %+v", fresh)
	}
}
