// Package common defines the necessary types to configure the application.
// This minimal configuration is tailored for logging.
package commoncfg

import (
	"encoding/json"
	"errors"
	"runtime/debug"
	"time"
)

// LoggerFormat is used to specify the logging format.
type LoggerFormat string

// LoggerTimeType is used to specify the type of time formatting.
type LoggerTimeType string

// SourceValueType represents the source type for retrieving configuration values.
type SourceValueType string

// FileFormat represents the format of a file.
type FileFormat string

// SecretType defines the type of secret used for authentication.
type SecretType string

// Protocol represents the communication protocol.
type Protocol string

const (
	// Logger format types.
	JSONLoggerFormat LoggerFormat = "json"
	TextLoggerFormat LoggerFormat = "text"

	// Logger time types.
	UnixTimeLogger    LoggerTimeType = "unix"
	PatternTimeLogger LoggerTimeType = "pattern"

	GRPCProtocol Protocol = "grpc"
	HTTPProtocol Protocol = "http"

	InsecureSecretType SecretType = "insecure"
	MTLSSecretType     SecretType = "mtls"
	ApiTokenSecretType SecretType = "api-token"
	BasicSecretType    SecretType = "basic"
	OAuth2SecretType   SecretType = "oauth2"

	EmbeddedSourceValue SourceValueType = "embedded"
	EnvSourceValue      SourceValueType = "env"
	FileSourceValue     SourceValueType = "file"

	JSONFileFormat   FileFormat = "json"
	YAMLFileFormat   FileFormat = "yaml"
	BinaryFileFormat FileFormat = "binary"
)

var ErrFeatureNotFound = errors.New("feature not found")

type BaseConfig struct {
	Application  Application  `yaml:"application" json:"application"`
	FeatureGates FeatureGates `yaml:"featureGates" json:"featureGates"`
	Status       Status       `yaml:"status" json:"status"`
	Logger       Logger       `yaml:"logger" json:"logger"`
	Telemetry    Telemetry    `yaml:"telemetry" json:"telemetry"`
	Audit        Audit        `yaml:"audit" json:"audit"`
}

// FeatureGates are a set of key=value pairs that describe service features.
type FeatureGates map[string]bool

func (fg FeatureGates) IsFeatureEnabled(feature string) bool {
	v, ok := fg[feature]
	return ok && v
}

func (fg FeatureGates) Feature(feature string) (bool, error) {
	if v, ok := fg[feature]; !ok {
		return false, ErrFeatureNotFound
	} else {
		return v, nil
	}
}

// Application holds minimal application configuration.
type Application struct {
	Name             string            `yaml:"name" json:"name"`
	Environment      string            `yaml:"environment" json:"environment"`
	Labels           map[string]string `yaml:"labels" json:"labels"`
	BuildInfo        BuildInfo
	RuntimeBuildInfo *debug.BuildInfo
}

type Status struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Status.Address is the address to listen on for status reporting
	Address string `yaml:"address" json:"address" default:":8888"`
	// Status.Profiling enables profiling on the status server
	Profiling bool `yaml:"profiling" json:"profiling"`
}

// Logger holds the configuration for logging.
type Logger struct {
	Source    bool            `yaml:"source" json:"source"`
	Format    LoggerFormat    `yaml:"format" json:"format" default:"json"`
	Level     string          `yaml:"level" json:"level" default:"info"`
	Formatter LoggerFormatter `yaml:"formatter" json:"formatter"`
}

// LoggerTime holds configuration for the time formatting in logs.
type LoggerTime struct {
	Type      LoggerTimeType `yaml:"type" json:"type" default:"unix"`
	Pattern   string         `yaml:"pattern" json:"pattern" default:"Mon Jan 02 15:04:05 -0700 2006"`
	Precision string         `yaml:"precision" json:"precision" default:"1us"`
}

// LoggerFormatter holds the logger formatter configuration.
type LoggerFormatter struct {
	Time   LoggerTime   `yaml:"time" json:"time"`
	Fields LoggerFields `yaml:"fields" json:"fields"`
}

// LoggerOtel holds configuration for the OpenTelemetry fields.
type LoggerOTel struct {
	TraceID string `yaml:"traceId" json:"traceId" default:"traceId"`
	SpanID  string `yaml:"spanId" json:"spanId" default:"spanId"`
}

// LoggerFields holds the mapping of log attributes.
type LoggerFields struct {
	Time    string              `yaml:"time" json:"time" default:"time"`
	Error   string              `yaml:"error" json:"error" default:"error"`
	Level   string              `yaml:"level" json:"level" default:"info"`
	Message string              `yaml:"message" json:"message" default:"msg"`
	OTel    LoggerOTel          `yaml:"otel" json:"otel"`
	Masking LoggerFieldsMasking `yaml:"masking" json:"masking"`
}

// LoggerFieldsMasking holds configuration for masking log fields.
type LoggerFieldsMasking struct {
	PII   []string          `yaml:"pii" json:"pii"`
	Other map[string]string `yaml:"other" json:"other"`
}

// Telemetry defines the configuration for telemetry components.
type Telemetry struct {
	DynatraceOneAgent bool   `yaml:"dynatraceOneAgent" json:"dynatraceOneAgent"`
	Traces            Trace  `yaml:"traces" json:"traces"`
	Metrics           Metric `yaml:"metrics" json:"metrics"`
	Logs              Log    `yaml:"logs" json:"logs"`
}

// Trace defines settings for distributed tracing.
type Trace struct {
	Enabled   bool      `yaml:"enabled" json:"enabled"`
	Protocol  Protocol  `yaml:"protocol" json:"protocol"`
	Host      SourceRef `yaml:"host" json:"host"`
	URL       string    `yaml:"url" json:"url"`
	SecretRef SecretRef `yaml:"secretRef" json:"secretRef"`
}

// Log defines settings for structured logging export.
type Log struct {
	Enabled   bool      `yaml:"enabled" json:"enabled"`
	Protocol  Protocol  `yaml:"protocol" json:"protocol"`
	Host      SourceRef `yaml:"host" json:"host"`
	URL       string    `yaml:"url" json:"url"`
	SecretRef SecretRef `yaml:"secretRef" json:"secretRef"`
}

// Metric defines settings for metrics export and Prometheus.
type Metric struct {
	Enabled    bool       `yaml:"enabled" json:"enabled"`
	Protocol   Protocol   `yaml:"protocol" json:"protocol"`
	Host       SourceRef  `yaml:"host" json:"host"`
	URL        string     `yaml:"url" json:"url"`
	SecretRef  SecretRef  `yaml:"secretRef" json:"secretRef"`
	Prometheus Prometheus `yaml:"prometheus" json:"prometheus"`
}

// SecretRef defines how credentials or certificates are provided.
type SecretRef struct {
	Type     SecretType `yaml:"type" json:"type"`
	MTLS     MTLS       `yaml:"mtls" json:"mtls"`
	APIToken SourceRef  `yaml:"apiToken" json:"apiToken"`
	OAuth2   OAuth2     `yaml:"oauth2" json:"oauth2"`
	Basic    BasicAuth  `yaml:"basic" json:"basic"`
}

// MTLS holds mTLS configuration for audit library.
type MTLS struct {
	Cert     SourceRef `yaml:"cert" json:"cert"`
	CertKey  SourceRef `yaml:"certKey" json:"certKey"`
	ServerCA SourceRef `yaml:"serverCa" json:"serverCa"`
}

// Audit holds the audit log library configuration.
type Audit struct {
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	// Potential mTLS for the endpoint.
	MTLS *MTLS `yaml:"mtls" json:"mtls"`
	// Potential BasicAuth for the endpoint.
	BasicAuth *BasicAuth `yaml:"basicAuth" json:"basicAuth"`
	// Optional set of additional properties to be added to OTLP log object. Must be added as a literal string to maintain casing.
	AdditionalProperties string `yaml:"additionalProperties" json:"additionalProperties"`
}

// BasicAuth holds basic auth configuration for audit library.
type BasicAuth struct {
	Username SourceRef `yaml:"username" json:"username"`
	Password SourceRef `yaml:"password" json:"password"`
}

// OAuth2 holds client id and secret auth configuration
type OAuth2 struct {
	URL         SourceRef         `yaml:"url" json:"url"`
	Credentials OAuth2Credentials `yaml:"credentials" json:"credentials"`
	MTLS        *MTLS             `yaml:"mtls" json:"mtls"`
}

type OAuth2Credentials struct {
	ClientID     SourceRef  `yaml:"clientID" json:"clientID"`
	ClientSecret *SourceRef `yaml:"clientSecret" json:"clientSecret"`
}

// SourceRef defines a reference to a source for retrieving a value.
type SourceRef struct {
	Source SourceValueType `yaml:"source" json:"source"`
	Env    string          `yaml:"env" json:"env"`
	File   CredentialFile  `yaml:"file" json:"file"`
	Value  string          `yaml:"value" json:"value"`
}

// CredentialFile describes a file-based credential.
type CredentialFile struct {
	Path     string     `yaml:"path" json:"path"`
	Format   FileFormat `yaml:"format" json:"format"`
	JSONPath string     `yaml:"jsonPath" json:"jsonPath"`
}

// Prometheus defines configuration for Prometheus integration.
type Prometheus struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// GRPCServer specifies the gRPC server configuration e.g. used by the
// business gRPC server if any.
type GRPCServer struct {
	Address string `yaml:"address" json:"address" default:":9092"`
	Flags   Flags  `yaml:"flags" json:"flags"`
	// MaxSendMsgSize returns a ServerOption to set the max message size in bytes the server can send.
	// If this is not set, gRPC uses the default `2147483647`.
	MaxSendMsgSize int `yaml:"maxSendMsgSize" json:"maxSendMsgSize" default:"2147483647"`
	// MaxRecvMsgSize returns a ServerOption to set the max message size in bytes the server can receive.
	// If this is not set, gRPC uses the default 4MB.
	MaxRecvMsgSize int `yaml:"maxRecvMsgSize" json:"maxRecvMsgSize" default:"125829120"`
	// MinTime is the minimum amount of time a client should wait before sending
	// a keepalive ping.
	EfPolMinTime time.Duration `yaml:"efPolMinTime" json:"efPolMinTime" default:"180s"` // The current default value is 5 minutes.
	// If true, server allows keepalive pings even when there are no active
	// streams(RPCs). If false, and client sends ping when there are no active
	// streams, server will send GOAWAY and close the connection.
	EfPolPermitWithoutStream bool                 `yaml:"efPolPermitWithoutStream" json:"efPolPermitWithoutStream"` // false by default.
	Attributes               GRPCServerAttributes `yaml:"attributes" json:"attributes"`
}

type Flags struct {
	// Reflection is a protocol that gRPC servers can use to declare the protobuf-defined APIs.
	// Reflection is used by debugging tools like grpcurl or grpcui.
	// See https://grpc.io/docs/guides/reflection/.
	Reflection bool `yaml:"reflection" json:"reflection"`
	Health     bool `yaml:"health" json:"health"`
}

type GRPCServerAttributes struct {
	// MaxConnectionIdle is a duration for the amount of time after which an
	// idle connection would be closed by sending a GoAway. Idleness duration is
	// defined since the most recent time the number of outstanding RPCs became
	// zero or the connection establishment.
	MaxConnectionIdle time.Duration `yaml:"maxConnectionIdle" json:"maxConnectionIdle" default:"1800s"` // The current default value is infinity.
	// MaxConnectionAge is a duration for the maximum amount of time a
	// connection may exist before it will be closed by sending a GoAway. A
	// random jitter of +/-10% will be added to MaxConnectionAge to spread out
	// connection storms.
	MaxConnectionAge time.Duration `yaml:"maxConnectionAge" json:"maxConnectionAge" default:"1800s"` // The current default value is infinity.
	// MaxConnectionAgeGrace is an additive period after MaxConnectionAge after
	// which the connection will be forcibly closed.
	MaxConnectionAgeGrace time.Duration `yaml:"maxConnectionAgeGrace" json:"maxConnectionAgeGrace" default:"300s"` // The current default value is infinity.
	// After a duration of this time if the server doesn't see any activity it
	// pings the client to see if the transport is still alive.
	// If set below 1s, a minimum value of 1s will be used instead.
	Time time.Duration `yaml:"time" json:"time" default:"120m"` // The current default value is 2 hours.
	// After having pinged for keepalive check, the server waits for a duration
	// of Timeout and if no activity is seen even after that the connection is
	// closed.
	Timeout time.Duration `yaml:"timeout" json:"timeout" default:"20s"` // The current default value is 20 seconds.
}

// GRPCClient specifies the gRPC client configuration e.g. used by the
// gRPC health check client.
type GRPCClient struct {
	Address    string               `yaml:"address" json:"address"`
	Attributes GRPCClientAttributes `yaml:"attributes" json:"attributes"`
	Pool       GRPCPool             `yaml:"pool" json:"pool"`
}

type GRPCPool struct {
	InitialCapacity int           `yaml:"initialCapacity" json:"initialCapacity" default:"1"`
	MaxCapacity     int           `yaml:"maxCapacity" json:"maxCapacity" default:"1"`
	IdleTimeout     time.Duration `yaml:"idleTimeout" json:"idleTimeout" default:"5s"`
	MaxLifeDuration time.Duration `yaml:"maxLifeDuration" json:"maxLifeDuration" default:"60s"`
}

type GRPCClientAttributes struct {
	//  GRPC KeepaliveTime option
	KeepaliveTime time.Duration `yaml:"keepaliveTime" json:"keepaliveTime" default:"80s"`
	//  GRPC KeepaliveTimeout option
	KeepaliveTimeout time.Duration `yaml:"keepaliveTimeout" json:"keepaliveTimeout" default:"40s"`
}

// BuildInfo holds metadata about the build
type BuildInfo struct {
	rawJSON json.RawMessage

	Branch    string `json:"branch"`
	Org       string `json:"org"`
	Product   string `json:"product"`
	Repo      string `json:"repo"`
	SHA       string `json:"sha"`
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
}

func (bi *BuildInfo) String() string {
	return string(bi.rawJSON)
}
