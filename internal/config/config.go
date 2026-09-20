// Package config is ~/.renfe/config.toml: the settings a user writes by hand —
// the session cookie for the authenticated commands, and the defaults that fill
// in what a command line leaves out. The machine-managed files next to it
// belong to internal/store.
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/seifreed/renfecli/internal/store"
)

const configFile = "config.toml"

// Path is where config.toml lives. It is exported because the file may hold a
// session cookie, and the CLI warns when its mode lets other users read it.
func Path() string { return store.Path(configFile) }

// Config mirrors ~/.renfe/config.toml.
type Config struct {
	Auth struct {
		Cookie string `toml:"cookie"` // browser session for the authenticated leg
	} `toml:"auth"`
	Defaults struct {
		Origin      string `toml:"origin"`      // station name or code used when the origin is omitted
		Destination string `toml:"destination"` // likewise for the destination
		Adults      int    `toml:"adults"`      // passengers, when not given per search
	} `toml:"defaults"`
}

// Load reads config.toml. A missing file is not an error (empty config). Any
// other stat error (permission, broken path) IS returned, so a malformed or
// unreadable config fails loudly rather than silently reverting to defaults.
func Load() (Config, error) {
	var c Config
	p := Path()
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, fmt.Errorf("stat config %s: %w", p, err)
	}
	_, err := toml.DecodeFile(p, &c)
	return c, err
}
