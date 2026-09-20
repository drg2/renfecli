package config

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempDir points the store at a throwaway config dir for one test.
func useTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RENFE_CONFIG_DIR", dir)
	return dir
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	useTempDir(t)
	c, err := Load()
	if err != nil {
		t.Fatalf("a missing config.toml should read as empty, got %v", err)
	}
	if c.Auth.Cookie != "" || c.Defaults.Origin != "" {
		t.Errorf("empty config is not empty: %+v", c)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := useTempDir(t)
	toml := `
[auth]
cookie = "JSESSIONID=abc; SSOInfo=xyz"

[defaults]
origin = "Madrid"
destination = "Sevilla"
adults = 2
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Auth.Cookie != "JSESSIONID=abc; SSOInfo=xyz" {
		t.Errorf("cookie = %q", c.Auth.Cookie)
	}
	if c.Defaults.Origin != "Madrid" || c.Defaults.Destination != "Sevilla" ||
		c.Defaults.Adults != 2 {
		t.Errorf("defaults = %+v", c.Defaults)
	}
}

func TestLoadConfigRejectsMalformedTOML(t *testing.T) {
	dir := useTempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[auth\ncookie ="), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Error("a malformed config should fail loudly, not silently read as empty")
	}
}
