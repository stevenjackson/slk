package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Load reads ~/.slk/config into the environment. Missing file is not an error.
// Existing env vars take precedence (godotenv does not overwrite).
func Load() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	godotenv.Load(filepath.Join(home, ".slk", "config"))
}
