package commoncfg

import (
	"encoding/json"
	"errors"
	"os"
	"runtime/debug"
	"strings"

	"github.com/davidhoo/jsonpath"
	"github.com/go-viper/mapstructure/v2"
	"github.com/samber/oops"
	"github.com/spf13/viper"
)

// Loader is used to load configuration from a `config.yaml` file.
// It supports loading from multiple paths and can override values with environment variables.
// It is instantiated by using the NewLoader function that supports multiple options.
type Loader struct {
	cfg       any
	defaults  map[string]any
	paths     []string
	envPrefix string
	useEnv    bool
}

type Option func(*Loader)

// NewLoader creates a new config loader
func NewLoader(cfg any, options ...Option) *Loader {
	loader := &Loader{cfg: cfg}

	for _, o := range options {
		if o != nil {
			o(loader)
		}
	}
	return loader
}

// WithDefaults sets the default values for the config loader
func WithDefaults(defaults map[string]any) Option {
	return func(l *Loader) {
		l.defaults = defaults
	}
}

// WithPaths sets the paths where the Loader looks for the config file
func WithPaths(paths ...string) Option {
	return func(l *Loader) {
		l.paths = paths
	}
}

// WithEnvOverride activates the environment variable override option with an optional prefix
// To override a config value, the environment variable should be named as:
// <ENVPREFIX>_<KEY1>_<KEY2>_<KEY3>_... (all uppercase)
// If the prefix is not set, the environment variable should be named as:
// <KEY1>_<KEY2>_<KEY3>_... (all uppercase)
func WithEnvOverride(prefix string) Option {
	return func(l *Loader) {
		l.envPrefix = prefix
		l.useEnv = true
	}
}

// LoadConfig is a convenience function to load the config file from the specified paths
func LoadConfig[T any | BaseConfig](cfg T, defaults map[string]any, paths ...string) error {
	loader := NewLoader(cfg, WithDefaults(defaults), WithPaths(paths...))
	return loader.LoadConfig()
}

// LoadConfig loads the config from the specified paths and environment variables
func (l *Loader) LoadConfig() error {
	v := viper.New()
	for key, val := range l.defaults {
		v.SetDefault(key, val)
	}
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	for _, path := range l.paths {
		v.AddConfigPath(path)
	}

	if l.useEnv {
		v.SetEnvPrefix(l.envPrefix)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()
	}

	if err := v.ReadInConfig(); err != nil {
		return oops.
			In("Config Loader").
			Wrapf(err, "Failed reading config file")
	}
	err := v.Unmarshal(l.cfg,
		func(c *mapstructure.DecoderConfig) {
			c.ErrorUnused = true // error if there are unknown keys in the config
		},
	)
	if err != nil {
		return oops.
			In("Config Loader").
			Wrapf(err, "Unable to unmarshall configuration")
	}
	return nil
}

func UpdateConfigVersion(cfg *BaseConfig, buildInfo string) error {
	cfg.Application.BuildInfo = BuildInfo{
		rawJSON: []byte(buildInfo),
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		cfg.Application.RuntimeBuildInfo = bi
	}

	err := json.Unmarshal([]byte(buildInfo), &cfg.Application.BuildInfo)
	if err != nil {
		return err
	}

	return nil
}

func LoadValueFromSourceRef(cred SourceRef) ([]byte, error) {
	switch cred.Source {
	case EmbeddedSourceValue:
		return []byte(cred.Value), nil
	case EnvSourceValue:
		envSourceVal := os.Getenv(strings.TrimSpace(cred.Env))
		if envSourceVal == "" {
			return nil, errors.New("environment variable not set")
		}
		return []byte(envSourceVal), nil
	case FileSourceValue:
		data, err := os.ReadFile(cred.File.Path)
		if err != nil {
			return nil, err
		}

		switch cred.File.Format {
		case JSONFileFormat:
			result, err := jsonpath.Query(string(data), cred.File.JSONPath)
			if err != nil {
				return nil, err
			}

			r, ok := result.(string)
			if !ok {
				return nil, errors.New("invalid credential format, expect string value")
			}

			return []byte(r), nil
		case BinaryFileFormat:
			return data, nil
		}
	}

	return nil, errors.New("no credential found, based on given credentials source")
}
