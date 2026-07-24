package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// envPrefix is prepended to flag names to form the environment variable
// consulted for config overrides (for example, flag "juju-channel" -> env var
// "CONCIERGE_JUJU_CHANNEL").
const envPrefix = "CONCIERGE"

// defaultConfigFileName is the config file name looked for in the current
// working directory when no --config flag is given.
const defaultConfigFileName = "concierge.yaml"

func NewConfig(cmd *cobra.Command, flags *pflag.FlagSet) (*Config, error) {
	var conf *Config
	var err error

	bindFlags(cmd)

	// Grab the relevant command line flags
	configFile, _ := flags.GetString("config")
	preset, _ := flags.GetString("preset")
	verbose, _ := flags.GetBool("verbose")
	trace, _ := flags.GetBool("trace")

	if len(preset) > 0 {
		conf, err = Preset(preset)
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration preset: %w", err)
		}
		slog.Info("Preset selected", "preset", preset)
	} else {
		// Load and validate the configuration file
		conf, err = parseConfig(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse configuration: %w", err)
		}
	}

	dryRun, _ := flags.GetBool("dry-run")

	conf.Overrides = getOverrides(flags)
	conf.Verbose = verbose
	conf.Trace = trace
	conf.DryRun = dryRun

	return conf, nil
}

// parseConfig locates and parses the concierge configuration.
func parseConfig(configFile string) (*Config, error) {
	var data []byte

	if len(configFile) > 0 {
		// If the user specified a path to the config file manually, load that file
		b, err := os.ReadFile(configFile) //nolint:gosec // Config file path is provided by the user via CLI flag
		if err != nil {
			return nil, fmt.Errorf("unable to read specified config file: %w", err)
		}
		data = b

		slog.Info("Configuration file found", "path", configFile)
	} else {
		// Otherwise check in the default location
		b, err := os.ReadFile(defaultConfigFileName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				slog.Info("No config file found, falling back to 'dev' preset")

				conf, err := Preset("dev")
				if err != nil {
					return nil, fmt.Errorf("failed to load configuration preset: %w", err)
				}

				return conf, nil
			}

			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		data = b

		slog.Info("Configuration file found", "path", defaultConfigFileName)
	}

	conf, err := unmarshalYAMLConfig(data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Expand environment variables in config values
	expandConfigEnvVars(conf)

	return conf, nil
}

// getOverrides parses the cli flags related to config overrides and returns a constructed
// ConfigOverrides struct.
func getOverrides(flags *pflag.FlagSet) ConfigOverrides {
	return ConfigOverrides{
		DisableJuju:       envOrFlagBool(flags, "disable-juju"),
		JujuChannel:       envOrFlagString(flags, "juju-channel"),
		K8sChannel:        envOrFlagString(flags, "k8s-channel"),
		MicroK8sChannel:   envOrFlagString(flags, "microk8s-channel"),
		LXDChannel:        envOrFlagString(flags, "lxd-channel"),
		CharmcraftChannel: envOrFlagString(flags, "charmcraft-channel"),
		SnapcraftChannel:  envOrFlagString(flags, "snapcraft-channel"),
		RockcraftChannel:  envOrFlagString(flags, "rockcraft-channel"),

		GoogleCredentialFile: envOrFlagString(flags, "google-credential-file"),

		ExtraSnaps: envOrFlagSlice(flags, "extra-snaps"),
		ExtraDebs:  envOrFlagSlice(flags, "extra-debs"),
	}
}

// envOrFlagBool returns a boolean config value set from env var or flag, priority on env var.
func envOrFlagBool(flags *pflag.FlagSet, key string) bool {
	value, _ := flags.GetBool(key)
	if v, ok := os.LookupEnv(flagToEnvVar(key)); ok {
		if b, err := strconv.ParseBool(v); err == nil && b {
			value = b
		}
	}
	return value
}

// envOrFlagString returns a string config value set from env var or flag, priority on env var.
func envOrFlagString(flags *pflag.FlagSet, key string) string {
	value, _ := flags.GetString(key)
	if v, ok := os.LookupEnv(flagToEnvVar(key)); ok && v != "" {
		value = v
	}
	return value
}

// envOrFlagSlice returns a slice config value set from env var or flag, priority on env var.
func envOrFlagSlice(flags *pflag.FlagSet, key string) []string {
	value, _ := flags.GetStringSlice(key)

	if v, ok := os.LookupEnv(flagToEnvVar(key)); ok && v != "" {
		parts := strings.SplitSeq(v, ",")
		for p := range parts {
			extraValue := p
			value = append(value, extraValue)
		}
	}

	return value
}

// bindFlags ensures that for each flag defined, the equivalent env var is also check for a value.
func bindFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			return
		}

		// Apply the environment variable value to the flag when the flag is not set
		// and the equivalent env var has a value.
		envVar := flagToEnvVar(f.Name)
		val, ok := os.LookupEnv(envVar)
		if !ok {
			return
		}

		slog.Debug("Override detected in environment", "override", f.Name, "value", val, "env_var", envVar)
		_ = cmd.Flags().Set(f.Name, val) // Flag is known to exist; Set only fails on unknown flags
	})
}

// flagToEnvVar converts command flag name to equivalent environment variable name
func flagToEnvVar(flag string) string {
	envVarSuffix := strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
	return fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)
}

// envVarPattern matches both $VAR and ${VAR} patterns
var envVarPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// expandEnvVars expands environment variable references in a string.
// Supports both $VAR and ${VAR} syntax.
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from either ${VAR} or $VAR format
		var varName string
		if strings.HasPrefix(match, "${") {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}
		return os.Getenv(varName)
	})
}

// expandConfigEnvVars expands environment variables in relevant string fields of the config.
func expandConfigEnvVars(conf *Config) {
	// Expand in MicroK8s image registry config
	conf.Providers.MicroK8s.ImageRegistry.URL = expandEnvVars(conf.Providers.MicroK8s.ImageRegistry.URL)
	conf.Providers.MicroK8s.ImageRegistry.Username = expandEnvVars(conf.Providers.MicroK8s.ImageRegistry.Username)
	conf.Providers.MicroK8s.ImageRegistry.Password = expandEnvVars(conf.Providers.MicroK8s.ImageRegistry.Password)

	// Expand in K8s image registry config
	conf.Providers.K8s.ImageRegistry.URL = expandEnvVars(conf.Providers.K8s.ImageRegistry.URL)
	conf.Providers.K8s.ImageRegistry.Username = expandEnvVars(conf.Providers.K8s.ImageRegistry.Username)
	conf.Providers.K8s.ImageRegistry.Password = expandEnvVars(conf.Providers.K8s.ImageRegistry.Password)
}
