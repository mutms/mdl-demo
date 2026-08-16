// Package state persists the little mdl-demo needs to remember across
// restarts: the web UI password hash and what is currently installed.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Path is the state file location. moodle-cron.service conditions on its
// existence, so it must only exist once a site is installed OR a password
// is set — both fine: cron itself checks for an installed site.
const Path = "/etc/mdl-demo/state.json"

type State struct {
	// PasswordHash is "pbkdf2-sha256:<iterations>:<salt-b64>:<hash-b64>",
	// empty until a web UI password is set.
	PasswordHash string    `json:"password_hash,omitempty"`
	Recipe       string    `json:"recipe,omitempty"`
	Wwwroot      string    `json:"wwwroot,omitempty"`
	InstalledAt  time.Time `json:"installed_at,omitzero"`
}

func (s *State) Installed() bool { return s.Recipe != "" }

// Load returns the saved state, or a zero state if none exists yet.
func Load() (*State, error) {
	data, err := os.ReadFile(Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := &State{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Save writes the state atomically; 0600 because the password hash is in it.
func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := Path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(Path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, Path)
}
