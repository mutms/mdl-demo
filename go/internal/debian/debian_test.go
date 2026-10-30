package debian

import "testing"

func TestIsNoChange(t *testing.T) {
	for line, want := range map[string]bool{
		"0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.": true,
		"3 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.": false,
		"0 upgraded, 1 newly installed, 0 to remove and 0 not upgraded.": false,
		"Reading package lists...":                                       false,
	} {
		if got := isNoChange(line); got != want {
			t.Errorf("isNoChange(%q) = %v, want %v", line, got, want)
		}
	}
}
