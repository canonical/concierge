package presets

import "embed"

// FS contains the embedded preset YAML files.
//
//go:embed *.yaml
var FS embed.FS
