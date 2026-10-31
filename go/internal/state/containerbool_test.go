package state

import "testing"

func TestParseBool(t *testing.T) {
	for v, want := range map[string]bool{
		"1": true, "true": true, "YES": true, " on ": true,
		"": false, "0": false, "false": false, "no": false, "off": false, "2": false,
	} {
		if got := parseBool(v); got != want {
			t.Errorf("parseBool(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestAdoptEditableCode(t *testing.T) {
	var s State
	if s.AdoptEditableCode("") || s.EditableCode {
		t.Error("an unset option must not switch it on")
	}
	if !s.AdoptEditableCode("1") || !s.EditableCode {
		t.Error("the option must switch it on and report a change")
	}
	if s.AdoptEditableCode("1") {
		t.Error("adopting again must not report a change")
	}
	if s.AdoptEditableCode("") || !s.EditableCode {
		t.Error("once recorded it stays, whatever the environment says")
	}
}
