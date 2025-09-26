package commoncfg

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/fsnotify/fsnotify"

	fs "github.com/openkcm/common-sdk/pkg/commonfs/watcher"
)

const (
	DefaultAttempts = 3
	DefaultDelay    = 100 * time.Millisecond
	DefaultMaxDelay = 5 * time.Second
)

// ReloadCallback is a function type for certificate reload callbacks
type ReloadCallback func(*tls.Config, error)

// CertWatcher monitors certificate files for rotation using fs.NotifyWrapper
type CertWatcher struct {
	mu           sync.RWMutex
	mtlsConfig   MTLS
	tlsConfig    *tls.Config
	logger       *slog.Logger
	retryOptions []retry.Option
	watcher      *fs.NotifyWrapper
	callbacks    []ReloadCallback
}

// DefaultRetryOptions returns sensible default retry configuration
func DefaultRetryOptions() []retry.Option {
	return []retry.Option{
		retry.Attempts(DefaultAttempts),
		retry.Delay(DefaultDelay),
		retry.MaxDelay(DefaultMaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	}
}

// NewCertWatcher creates a new certificate watcher for the given MTLS configuration
func NewCertWatcher(mtlsConfig MTLS, logger *slog.Logger, retryOptions []retry.Option) (*CertWatcher, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if len(retryOptions) == 0 {
		retryOptions = DefaultRetryOptions()
	}

	cw := &CertWatcher{
		mtlsConfig:   mtlsConfig,
		logger:       logger,
		retryOptions: retryOptions,
		callbacks:    make([]ReloadCallback, 0),
	}

	// Load initial TLS configuration
	err := cw.loadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load initial TLS config: %w", err)
	}

	// Create and configure the filesystem watcher
	err = cw.createWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Start watching
	err = cw.watcher.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start watching: %w", err)
	}

	return cw, nil
}

// RegisterCallback adds a callback function that will be called when certificates are reloaded
func (cw *CertWatcher) RegisterCallback(callback ReloadCallback) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.callbacks = append(cw.callbacks, callback)
	cw.logger.Info("Certificate reload callback registered")
}

// executeCallbacks executes all registered callbacks with the current TLS config and any error
func (cw *CertWatcher) executeCallbacks(tlsConfig *tls.Config, err error) {
	cw.mu.RLock()
	callbacks := make([]ReloadCallback, len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mu.RUnlock()

	for i, callback := range callbacks {
		func(idx int, cb ReloadCallback) {
			defer func() {
				if r := recover(); r != nil {
					cw.logger.Error("Certificate reload callback panic",
						"callback_index", idx,
						"panic", r)
				}
			}()

			cb(tlsConfig, err)
		}(i, callback)
	}
}

// createWatcher creates and configures the fs.NotifyWrapper
func (cw *CertWatcher) createWatcher() error {
	certPaths := cw.getCertificatePaths()
	if len(certPaths) == 0 {
		return errors.New("no certificate paths found")
	}

	// Create watcher with certificate paths and handlers
	watcher, err := fs.NewFSWatcher(
		fs.OnPaths(certPaths...),
		fs.WithEventHandler(cw.handleCertEvent),
		fs.WithErrorEventHandler(cw.handleError),
	)
	if err != nil {
		return fmt.Errorf("failed to create fs watcher: %w", err)
	}

	cw.watcher = watcher

	cw.logger.Info("Certificate watcher configured", "paths", certPaths)

	return nil
}

// handleCertEvent is the event handler for certificate file changes
func (cw *CertWatcher) handleCertEvent(event fsnotify.Event) {
	// Handle both Write and Create events for directory watching
	// Create events occur when files are atomically replaced (common in k8s)
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}

	cw.logger.Debug("Certificate file changed, reloading...",
		"file", event.Name,
		"operation", event.Op.String())

	err := cw.loadTLSConfig()
	if err != nil {
		cw.logger.Error("Failed to reload certificate",
			"file", event.Name,
			"error", err.Error())
	}
}

// handleError is the error handler for filesystem watcher errors
func (cw *CertWatcher) handleError(err error) {
	cw.logger.Error("Certificate watcher error", "error", err.Error())
}

// loadTLSConfig loads or reloads the TLS configuration
func (cw *CertWatcher) loadTLSConfig() error {
	var tlsConfig *tls.Config

	err := retry.Do(
		func() error {
			var err error

			tlsConfig, err = LoadMTLSConfig(&cw.mtlsConfig)
			if err != nil {
				return err
			}

			return nil
		},
		cw.retryOptions...,
	)
	if err != nil {
		// Execute callbacks with error
		cw.executeCallbacks(nil, fmt.Errorf("failed to load TLS config: %w", err))
		return fmt.Errorf("failed to load TLS config: %w", err)
	}

	cw.mu.Lock()
	cw.tlsConfig = tlsConfig
	cw.mu.Unlock()

	cw.logger.Info("Certificate loaded successfully")

	// Execute callbacks with successful config
	cw.executeCallbacks(tlsConfig, nil)

	return nil
}

// getCertificatePaths extracts file paths from MTLS configuration
func (cw *CertWatcher) getCertificatePaths() []string {
	var paths []string

	pathSet := make(map[string]bool) // To avoid duplicate directories

	if cw.mtlsConfig.Cert.Source == FileSourceValue {
		dir := filepath.Dir(cw.mtlsConfig.Cert.File.Path)
		if !pathSet[dir] {
			paths = append(paths, dir)
			pathSet[dir] = true
		}
	}

	if cw.mtlsConfig.CertKey.Source == FileSourceValue {
		dir := filepath.Dir(cw.mtlsConfig.CertKey.File.Path)
		if !pathSet[dir] {
			paths = append(paths, dir)
			pathSet[dir] = true
		}
	}

	if cw.mtlsConfig.ServerCA.Source == FileSourceValue {
		dir := filepath.Dir(cw.mtlsConfig.ServerCA.File.Path)
		if !pathSet[dir] {
			paths = append(paths, dir)
			pathSet[dir] = true
		}
	}

	return paths
}

// GetTLSConfig returns the current TLS configuration
func (cw *CertWatcher) GetTLSConfig() *tls.Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	return cw.tlsConfig
}

// Reload triggers an immediate certificate reload
func (cw *CertWatcher) Reload() error {
	cw.logger.Info("Manual certificate reload triggered")
	return cw.loadTLSConfig()
}

// Close stops the certificate watcher
func (cw *CertWatcher) Close() error {
	if cw.watcher != nil {
		err := cw.watcher.Close()
		if err != nil {
			cw.logger.Error("Error closing watcher", "error", err.Error())
		}
	}

	cw.logger.Info("Certificate watcher stopped")

	return nil
}
