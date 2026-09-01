package config

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/canonical/concierge/presets"
	"gopkg.in/yaml.v3"
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

// unmarshalYAMLConfig parses YAML config data into a Config.
func unmarshalYAMLConfig(data []byte) (*Config, error) {
	conf := &Config{}
	if err := yaml.Unmarshal(data, conf); err != nil {
		return nil, err
	}
	return conf, nil
}

// loadPreset parses YAML preset data into a Config.
func loadPreset(data []byte) (*Config, error) {
	conf, err := unmarshalYAMLConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse preset: %w", err)
	}
	return conf, nil
}
