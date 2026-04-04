package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	Version      = "1.0.0"
	CatboxAPI    = "https://catbox.moe/user/api.php"
	LitterboxAPI = "https://litterbox.catbox.moe/resources/internals/api.php"
	MaxFileSize  = 200 * 1024 * 1024      // 200 MB for catbox
	MaxLBSize    = 1024 * 1024 * 1024      // 1 GB for litterbox
)

type Config struct {
	Userhash string `json:"userhash,omitempty"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "meow")
}

func Path() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() Config {
	var cfg Config
	data, err := os.ReadFile(Path())
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	return cfg
}

func Save(cfg Config) error {
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(Path(), data, 0644)
}

// MigrateOld copies config from ~/.config/catbox/ if meow config doesn't exist yet.
func MigrateOld() {
	if _, err := os.Stat(Path()); err == nil {
		return // meow config already exists
	}
	home, _ := os.UserHomeDir()
	oldPath := filepath.Join(home, ".config", "catbox", "config.json")
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return // no old config
	}
	os.MkdirAll(Dir(), 0755)
	os.WriteFile(Path(), data, 0644)
}
