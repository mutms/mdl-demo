// Package sso mints the single-use login tokens behind the console's
// "Log in…" buttons and QR codes. A token is 32 random bytes; what lands on
// disk — <dataroot>/mdldemo-sso/<sha256(token)>.json — is only its hash plus
// {username, expires}, so reading the dataroot never yields a usable login
// URL. The mdldemo-login.php handler (internal/moodle) consumes the file on
// first use; install/reset wipe the dataroot and the tokens with it.
package sso

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/mutms/mdl-demo/go/internal/execx"
	"github.com/mutms/mdl-demo/go/internal/moodle"
)

// TTL is how long an unclaimed token stays valid.
const TTL = 10 * time.Minute

func dir() string { return filepath.Join(moodle.Dataroot, "mdldemo-sso") }

type payload struct {
	Username string `json:"username"`
	Expires  int64  `json:"expires"`
}

// Mint creates a single-use login token for username, returning the token
// (goes into the login URL) and its id (the hash, used for status polling).
func Mint(username string) (token, id string, err error) {
	if err := ensureDir(); err != nil {
		return "", "", err
	}
	prune()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	id = hashOf(token)
	data, err := json.Marshal(payload{Username: username, Expires: time.Now().Add(TTL).Unix()})
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Join(dir(), id+".json"), data, 0644); err != nil {
		return "", "", err
	}
	return token, id, nil
}

// Pending reports whether the token behind id is still unclaimed and fresh.
// The caller has validated id's shape (64 hex chars).
func Pending(id string) bool {
	data, err := os.ReadFile(filepath.Join(dir(), id+".json"))
	if err != nil {
		return false
	}
	var p payload
	if json.Unmarshal(data, &p) != nil {
		return false
	}
	return p.Expires >= time.Now().Unix()
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ensureDir creates the token dir owned by www-data: PHP must read tokens
// and unlink them on use (single-use consumes via the directory write bit).
func ensureDir() error {
	d := dir()
	if _, err := os.Stat(d); err == nil {
		return nil
	}
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	return execx.Run(func(string) {}, "", "chown", "www-data:www-data", d)
}

// prune drops expired token files so abandoned codes do not pile up.
func prune() {
	entries, err := os.ReadDir(dir())
	if err != nil {
		return
	}
	now := time.Now().Unix()
	for _, e := range entries {
		path := filepath.Join(dir(), e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var p payload
		if json.Unmarshal(data, &p) != nil || p.Expires < now {
			_ = os.Remove(path)
		}
	}
}
