package common

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const configHeader = `# dwc-server-advanced configuration.
# Written by the setup wizard and the panel's server settings; you can also
# edit it by hand. config_example.yml documents every option.

`

// ConfigFileExists reports whether a config file can be loaded.
func ConfigFileExists() bool {
	_, _, err := readConfigFile()
	return err == nil
}

// ConfigFilePath is the file SaveConfig writes: WWFC_CONFIG if set, else
// config.yml (a legacy config.xml is migrated to YAML on the first save).
func ConfigFilePath() string {
	if path := strings.TrimSpace(os.Getenv("WWFC_CONFIG")); path != "" && strings.ToLower(filepath.Ext(path)) != ".xml" {
		return path
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return "config.yml"
}

// DefaultConfig is the config used when a key is missing from the file.
func DefaultConfig() Config {
	cfg, err := parseConfig([]byte("{}"), "config.yml")
	if err != nil {
		panic(err)
	}
	return cfg
}

// SaveConfig writes cfg to the config file atomically and makes it the
// loaded config. Settings read at startup still need a restart.
func SaveConfig(cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	path := ConfigFilePath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append([]byte(configHeader), data...), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	// Keep the old XML file out of the way so it is not loaded again.
	if path == "config.yml" {
		if _, err := os.Stat("config.xml"); err == nil {
			_ = os.Rename("config.xml", "config.xml.bak")
		}
	}

	return ReloadConfig()
}

// ReloadConfig reads the config file again.
func ReloadConfig() error {
	data, path, err := readConfigFile()
	if err != nil {
		return err
	}
	parsed, err := parseConfig(data, path)
	if err != nil {
		return err
	}

	cmutex.Lock()
	config = parsed
	configLoaded = true
	cmutex.Unlock()
	return nil
}
