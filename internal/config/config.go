package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func init() {
	viper.SetConfigType("yaml")
	viper.SetConfigName("concierge")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CONCIERGE")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

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
	// If the user specified a path to the config file manually, load that file
	if len(configFile) > 0 {
		b, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read specified config file: %w", err)
		}

		err = viper.ReadConfig(bytes.NewBuffer(b))
		if err != nil {
			return nil, fmt.Errorf("error parsing config file %s: %w", configFile, err)
		}

		slog.Info("Configuration file found", "path", configFile)
	} else {
		// Otherwise check in the default locations
		err := viper.ReadInConfig()
		if err != nil {
			var configFileNotFoundError viper.ConfigFileNotFoundError
			if errors.As(err, &configFileNotFoundError) {
				slog.Info("No config file found, falling back to 'dev' preset")

				conf, err := Preset("dev")
				if err != nil {
					return nil, fmt.Errorf("failed to load configuration preset: %w", err)
				}

				return conf, nil
			}

			return nil, fmt.Errorf("error reading config file: %w", err)
		}

		slog.Info("Configuration file found", "path", "concierge.yaml")
	}

	conf := &Config{}
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

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
	if v := viper.GetBool(key); v {
		value = v
	}
	return value
}

// envOrFlagString returns a string config value set from env var or flag, priority on env var.
func envOrFlagString(flags *pflag.FlagSet, key string) string {
	value, _ := flags.GetString(key)
	if v := viper.GetString(key); v != "" {
		value = v
	}
	return value
}

// envOrFlagSlice returns a slice config value set from env var or flag, priority on env var.
func envOrFlagSlice(flags *pflag.FlagSet, key string) []string {
	value, _ := flags.GetStringSlice(key)

	if v := viper.GetString(key); v != "" {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			extraValue := p
			value = append(value, extraValue)
		}
	}

	return value
}

// bindFlags ensures that for each flag defined, the equivalent env var is also check for a value.
func bindFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent keys with underscores
		if strings.Contains(f.Name, "-") {
			viper.BindEnv(f.Name, flagToEnvVar(f.Name))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && viper.IsSet(f.Name) {
			val := viper.Get(f.Name)
			slog.Debug("Override detected in environment", "override", f.Name, "value", fmt.Sprintf("%v", val), "env_var", flagToEnvVar(f.Name))
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

// flagToEnvVar converts command flag name to equivalent environment variable name
func flagToEnvVar(flag string) string {
	envVarSuffix := strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
	return fmt.Sprintf("%s_%s", viper.GetEnvPrefix(), envVarSuffix)
}
