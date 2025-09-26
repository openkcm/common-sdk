/*
Package certs provides a TLS configuration watcher that automatically
loads TLS certificates, private keys, and CA certificates from a
directory. It leverages a file watcher to dynamically reload TLS
configuration when the underlying files change.

The TLSConfigWatcher is intended for applications that need to serve
TLS connections and want to automatically pick up updated certificates
without restarting the process.

Example usage:

	dir := "/etc/myapp/certs"

	// Create a watcher with default filenames
	watcher, err := certs.NewTLSConfigWatcher(dir)
	if err != nil {
		log.Fatal(err)
	}

	// Optionally customize filenames
	watcher, err = certs.NewTLSConfigWatcher(dir,
		certs.WithCertFileName("server.crt"),
		certs.WithKeyFileName("server.key"),
		certs.WithCAFileName("ca_bundle.crt"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Start the watcher
	err = watcher.StartWatching()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.StopWatching()

	// Retrieve TLS configuration for use in a server
	tlsConfig, err := watcher.Get()
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:      ":443",
		TLSConfig: tlsConfig,
	}
	server.ListenAndServeTLS("", "")
*/
package certs

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commonfs/loader"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"
)

// Default filenames for certificate, private key, and CA.
const (
	// DefaultCertFilename is the default filename for the TLS certificate.
	DefaultCertFilename = "tls.crt"

	// DefaultKeyFilename is the default filename for the TLS private key.
	DefaultKeyFilename = "tls.key"

	// DefaultCAFilename is the default filename for the CA bundle.
	DefaultCAFilename = "ca.crt"
)

// TLSConfigWatcher watches a directory for TLS certificate files and
// provides a tls.Config based on the latest loaded certificates.
//
// It supports dynamic reloading through a file watcher and stores all
// loaded data in a memory storage.
type TLSConfigWatcher struct {
	crtFileName string
	keyFileName string
	caFileName  string

	certsLoader *loader.Loader
	storage     keyvalue.ReadOnlyStringToBytesStorage
}

// Option is a function type that can modify TLSConfigWatcher during creation.
type Option func(*TLSConfigWatcher) error

// WithCertFileName sets a custom certificate filename.
func WithCertFileName(filename string) Option {
	return func(w *TLSConfigWatcher) error {
		w.crtFileName = filename
		return nil
	}
}

// WithKeyFileName sets a custom private key filename.
func WithKeyFileName(filename string) Option {
	return func(w *TLSConfigWatcher) error {
		w.keyFileName = filename
		return nil
	}
}

// WithCAFileName sets a custom CA bundle filename.
func WithCAFileName(filename string) Option {
	return func(w *TLSConfigWatcher) error {
		w.caFileName = filename
		return nil
	}
}

// NewTLSConfigWatcher creates a new TLSConfigWatcher for the given path.
// It accepts optional settings for certificate, key, and CA filenames.
//
// The watcher uses an in-memory storage and a loader to monitor the
// directory for changes.
func NewTLSConfigWatcher(path string, opts ...Option) (*TLSConfigWatcher, error) {
	memoryStorage := keyvalue.NewMemoryStorage[string, []byte]()

	certsLoader, err := loader.Create(
		path,
		loader.WithKeyIDType(loader.FileNameWithoutExtension),
		loader.WithStorage(memoryStorage),
	)
	if err != nil {
		return nil, err
	}

	watcher := &TLSConfigWatcher{
		crtFileName: DefaultCertFilename,
		keyFileName: DefaultKeyFilename,
		caFileName:  DefaultCAFilename,
		certsLoader: certsLoader,
		storage:     memoryStorage,
	}

	for _, opt := range opts {
		if opt != nil {
			err := opt(watcher)
			if err != nil {
				return nil, err
			}
		}
	}

	return watcher, nil
}

// StartWatching starts the underlying file watcher and loads all
// existing certificate files.
//
// Returns an error if starting the watcher or loading resources fails.
func (dl *TLSConfigWatcher) StartWatching() error {
	return dl.certsLoader.StartWatching()
}

// StopWatching stops the watcher and releases resources.
// It is safe to call multiple times.
func (dl *TLSConfigWatcher) StopWatching() error {
	return dl.certsLoader.StopWatching()
}

// Storage returns the internal read-only storage containing
// all loaded certificate data.
func (tw *TLSConfigWatcher) Storage() keyvalue.ReadOnlyStringToBytesStorage {
	return tw.storage
}

// Get returns a tls.Config built from the latest loaded certificate,
// private key, and optionally CA bundle.
//
// Returns an error if any required file is missing or cannot be parsed.
func (c *TLSConfigWatcher) Get() (*tls.Config, error) {
	certPEMBlock, ok := c.storage.Get(c.crtFileName)
	if !ok {
		return nil, fmt.Errorf("no value found for %s", c.crtFileName)
	}

	keyPEMBlock, ok := c.storage.Get(c.keyFileName)
	if !ok {
		return nil, fmt.Errorf("no value found for %s", c.keyFileName)
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if ca, ok := c.storage.Get(c.caFileName); ok {
		caCertPool.AppendCertsFromPEM(ca)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS13,
	}, nil
}
