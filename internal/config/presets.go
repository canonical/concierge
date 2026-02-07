package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/canonical/concierge/presets"
	"github.com/spf13/viper"
)

// ValidPresets returns the sorted list of available preset names.
func ValidPresets() []string {
	entries, err := presets.FS.ReadDir(".")
	if err != nil {
		return nil
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}

// Preset returns a configuration preset by name.
func Preset(preset string) (*Config, error) {
	filename := preset + ".yaml"
	data, err := presets.FS.ReadFile(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("unknown preset '%s'", preset)
		}
		return nil, fmt.Errorf("failed to read preset '%s': %w", preset, err)
	}
	return loadPreset(data)
}

// loadPreset parses YAML data into a Config using a fresh Viper instance.
func loadPreset(data []byte) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("failed to parse preset: %w", err)
	}

	conf := &Config{}
	if err := v.Unmarshal(conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal preset: %w", err)
	}
	return conf, nil
}
