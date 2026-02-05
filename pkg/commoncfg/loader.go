package commoncfg

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/creasty/defaults"
	"github.com/davidhoo/jsonpath"
	"github.com/go-viper/mapstructure/v2"
	"github.com/samber/oops"
	"github.com/spf13/viper"
)

const (
	DefaultFileName   = "config"
	DefaultFileFormat = YAMLFileFormat
)

var (
	ErrMTLSIsNil           = errors.New("missing mTLS configuration: value is nil")
	ErrCertificateIsNil    = errors.New("missing certificate configuration: value is nil")
	ErrKeyCertificateIsNil = errors.New("missing key certificate configuration: value is nil")
)

// Loader is used to load configuration from a `config.yaml` file.
// It supports loading from multiple paths and can override values with environment variables.
// It is instantiated by using the NewLoader function that supports multiple options.
type Loader struct {
	cfg        any
	defaults   map[string]any
	paths      []string
	envPrefix  string
	useEnv     bool
	fileName   string
	fileFormat FileFormat
}

type Option func(*Loader)

// NewLoader creates a new config loader
// Uses config.yaml as default config file
func NewLoader(cfg any, options ...Option) *Loader {
	loader := &Loader{cfg: cfg}

	loader.fileName = DefaultFileName
	loader.fileFormat = DefaultFileFormat

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

// WithFile sets the file name and type of the config file
func WithFile(name string, extension FileFormat) Option {
	return func(l *Loader) {
		if strings.TrimSpace(name) == "" {
			l.fileName = DefaultFileName
		} else {
			l.fileName = name
		}

		l.fileFormat = extension
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

	v.SetConfigName(l.fileName)
	v.SetConfigType(string(l.fileFormat))

	for _, path := range l.paths {
		v.AddConfigPath(path)
	}

	if l.useEnv {
		v.SetEnvPrefix(l.envPrefix)
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()
	}

	err := v.ReadInConfig()
	if err != nil {
		return oops.
			In("Config Loader").
			Wrapf(err, "Failed reading config file")
	}

	err = v.Unmarshal(l.cfg,
		func(c *mapstructure.DecoderConfig) {
			c.ErrorUnused = true // error if there are unknown keys in the config
		},
	)
	if err != nil {
		return oops.
			In("Config Loader").
			Wrapf(err, "Unable to unmarshall configuration")
	}

	// Loading the configuration fields tag defaults
	// Usage
	//     type ExampleBasic struct {
	//         Foo bool   `default:"true"`
	//         Bar string `default:"33"`
	//         Qux int8
	//         Dur time.Duration `default:"2m3s"`
	//     }
	//
	//      foo := &ExampleBasic{}
	//      Set(foo)
	return defaults.Set(l.cfg)
}

func ExtractValueFromSourceRef(cred *SourceRef) ([]byte, error) {
	if cred == nil {
		return nil, errors.New("given credential is nil")
	}

	switch cred.Source {
	case EmbeddedSourceValue:
		return []byte(cred.Value), nil
	case EnvSourceValue:
		result := env(cred.Value, cred.Env)
		if result == "" {
			return nil, errors.New("environment variable not set")
		}

		return []byte(result), nil
	case FileSourceValue:
		data, err := os.ReadFile(cred.File.Path)
		if err != nil {
			return nil, err
		}

		return parseFile(data, cred.File)
	}

	return nil, fmt.Errorf("no credential found, based on given credentials source: %s", cred.Source)
}

func LoadValueFromSourceRef(cred SourceRef) ([]byte, error) {
	return ExtractValueFromSourceRef(&cred)
}

func LoadMTLSClientCertificate(cfg *MTLS) (*tls.Certificate, error) {
	if cfg == nil {
		return nil, ErrMTLSIsNil
	}

	certPEMBlock, err := ExtractValueFromSourceRef(&cfg.Cert)
	if err != nil {
		return nil, err
	}

	keyPEMBlock, err := ExtractValueFromSourceRef(&cfg.CertKey)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

func LoadMTLSCACertPool(cfg *MTLS) (*x509.CertPool, error) {
	if cfg == nil {
		return nil, ErrMTLSIsNil
	}

	cas := make([]SourceRef, 0)
	if cfg.ServerCA != nil {
		cas = append(cas, *cfg.ServerCA)
	}

	if len(cfg.RootCAs) > 0 {
		cas = append(cas, cfg.RootCAs...)
	}

	certPool, err := LoadCAsCertPool(cas)
	if err != nil {
		return nil, err
	}

	return certPool, err
}

func LoadCACertPool(certRef *SourceRef) (*x509.CertPool, error) {
	if certRef == nil {
		// Returns nil instead of NewCertPool if no CA is provided, which means using the system CA pool
		return nil, nil //nolint:nilnil
	}

	caCertPool := x509.NewCertPool()

	caCert, err := ExtractValueFromSourceRef(certRef)
	if err != nil {
		return nil, err
	}

	caCertPool.AppendCertsFromPEM(caCert)

	return caCertPool, nil
}

func LoadCAsCertPool(certRefs []SourceRef) (*x509.CertPool, error) {
	if len(certRefs) == 0 {
		// Returns nil instead of NewCertPool if no CA is provided, which means using the system CA pool
		return nil, nil //nolint:nilnil
	}

	caCertPool := x509.NewCertPool()

	for _, cert := range certRefs {
		caCert, err := ExtractValueFromSourceRef(&cert)
		if err != nil {
			return nil, err
		}

		caCertPool.AppendCertsFromPEM(caCert)
	}

	return caCertPool, nil
}

func LoadClientCertificate(certRef, keyRef *SourceRef) (*tls.Certificate, error) {
	if certRef == nil {
		return nil, ErrCertificateIsNil
	}

	if keyRef == nil {
		return nil, ErrKeyCertificateIsNil
	}

	certPEMBlock, err := ExtractValueFromSourceRef(certRef)
	if err != nil {
		return nil, err
	}

	keyPEMBlock, err := ExtractValueFromSourceRef(keyRef)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	return &cert, nil
}

func LoadMTLSConfig(cfg *MTLS) (*tls.Config, error) {
	if cfg == nil {
		return nil, ErrMTLSIsNil
	}

	cert, err := LoadMTLSClientCertificate(cfg)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	if cfg.Attributes != nil {
		tlsConfig.InsecureSkipVerify = cfg.Attributes.InsecureSkipVerify
		tlsConfig.ServerName = cfg.Attributes.ServerName
		tlsConfig.SessionTicketsDisabled = cfg.Attributes.SessionTicketsDisabled
		tlsConfig.SessionTicketsDisabled = cfg.Attributes.SessionTicketsDisabled
	}

	caCertPool, err := LoadMTLSCACertPool(cfg)
	if err != nil {
		return nil, err
	}

	tlsConfig.RootCAs = caCertPool

	return tlsConfig, nil
}

func env(names ...string) string {
	for _, name := range names {
		val := os.Getenv(strings.TrimSpace(name))
		if val != "" {
			return val
		}
	}

	return ""
}

func parseFile(data []byte, file CredentialFile) ([]byte, error) {
	switch file.Format {
	case JSONFileFormat:
		if strings.TrimSpace(file.JSONPath) == "" {
			return data, nil
		}

		return jsonQuery(string(data), file.JSONPath)
	case YAMLFileFormat, BinaryFileFormat:
		return data, nil
	default:
		return data, nil
	}
}

func jsonQuery(data, path string) ([]byte, error) {
	result, err := jsonpath.Query(data, path)
	if err != nil {
		return nil, err
	}

	r, ok := result.(string)
	if !ok {
		return nil, errors.New("invalid credential format, expect string value")
	}

	return []byte(r), nil
}
