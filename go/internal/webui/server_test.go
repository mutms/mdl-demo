package webui

import "testing"

func TestSeries(t *testing.T) {
	cases := map[string]string{
		"5.2.2":    "5.2",
		"5.2.1":    "5.2",
		"5.2.2.01": "5.2", // a MuTMS build folds with its Moodle branch
		"5.2.1.01": "5.2",
		"5.2":      "5.2", // a dev stream is its own branch
		"4.5":      "4.5",
	}
	for v, want := range cases {
		if got := series(v); got != want {
			t.Errorf("series(%q) = %q, want %q", v, got, want)
		}
	}
}
