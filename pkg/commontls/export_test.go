package commontls

import (
	"crypto/x509"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// Test hooks, compiled only into the tests of this package.

// NewDynamicTLSConfigForTest is NewDynamicTLSConfig with the interval of the
// periodic check (none if zero) and the clock chosen by the test.
func NewDynamicTLSConfigForTest(throttle, recheck time.Duration, mtls *commoncfg.MTLS, now func() time.Time) (*DynamicTLSConfig, error) {
	return newDynamicTLSConfig(throttle, recheck, mtls, now)
}

// Refresh reads the credentials again, as a file change or the periodic
// check does, and returns once the decision is made.
func (d *DynamicTLSConfig) Refresh() {
	d.refresh()
}

// X509CertPool calls x509CertPool.
func (d *DynamicTLSConfig) X509CertPool() *x509.CertPool {
	return d.x509CertPool()
}
