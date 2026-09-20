// Package store is the ~/.renfe directory: the small JSON files the CLI keeps
// between runs — the browser session `login` lifts, the cached station
// catalogue. RENFE_CONFIG_DIR overrides the location.
//
// It knows nothing about what those files mean; the CLI decides that. What it
// owns is that the directory exists, that its contents stay private (it holds a
// session cookie), and that a write either lands whole or not at all.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// dir is ~/.renfe (or $RENFE_CONFIG_DIR). It falls back to a relative path when
// the home directory cannot be determined, so the CLI still runs.
func dir() string {
	if d := os.Getenv("RENFE_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".renfe"
	}
	return filepath.Join(home, ".renfe")
}

// Path is the full path of a file in the store.
func Path(name string) string { return filepath.Join(dir(), name) }

// Load reads a JSON file from the store into v.
func Load(name string, v any) error {
	b, err := os.ReadFile(Path(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Save writes v as pretty JSON to name, 0600 (the store holds secrets). The
// write is atomic — a temp file in the same dir, then os.Rename — so a crash or
// a full disk mid-write cannot truncate the credential store and leave the user
// with a corrupt, half-written session.
func Save(name string, v any) error {
	if err := os.MkdirAll(dir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	d := dir()
	tmp, err := os.CreateTemp(d, name+".tmp-*") // CreateTemp opens with 0600
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // cleanup on error; no-op once the rename succeeds
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(d, name))
}
