package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// useTempDir points the package at a throwaway store dir for one test.
func useTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RENFE_CONFIG_DIR", dir)
	return dir
}

func TestDirHonoursEnv(t *testing.T) {
	d := useTempDir(t)
	if dir() != d {
		t.Errorf("dir() = %q, want %q", dir(), d)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := useTempDir(t)
	type sess struct {
		Cookie string `json:"cookie"`
	}
	if err := Save("session.json", sess{Cookie: "JSESSIONID=abc"}); err != nil {
		t.Fatal(err)
	}
	var got sess
	if err := Load("session.json", &got); err != nil {
		t.Fatal(err)
	}
	if got.Cookie != "JSESSIONID=abc" {
		t.Errorf("round trip lost the value: %+v", got)
	}

	// The file holds a session cookie: it must not be world-readable, and the
	// temp file Save renames into place must not be left behind. Windows has no
	// mode to check — access there is an ACL and Stat reports 0666 whatever was
	// asked for.
	info, err := os.Stat(filepath.Join(dir, "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		t.Errorf("session.json mode = %o, want 600", perm)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("Save left temp files behind: %v", entries)
	}
}

func TestSaveOverwritesInPlace(t *testing.T) {
	useTempDir(t)
	type sess struct {
		Cookie string `json:"cookie"`
	}
	for _, want := range []string{"first", "second"} {
		if err := Save("session.json", sess{Cookie: want}); err != nil {
			t.Fatal(err)
		}
		var got sess
		if err := Load("session.json", &got); err != nil {
			t.Fatal(err)
		}
		if got.Cookie != want {
			t.Errorf("after save %q, loaded %q", want, got.Cookie)
		}
	}
}

func TestLoadMissingFileIsAnError(t *testing.T) {
	useTempDir(t)
	var v map[string]string
	if err := Load("nope.json", &v); err == nil {
		t.Error("Load of a missing file should fail so callers fall back deliberately")
	}
}
