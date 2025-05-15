package grpcpool

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"
)

var (
	// ErrClosed is the error when the client pool is closed
	ErrClosed = errors.New("grpc pool: client pool is closed")
	// ErrTimeout is the error when the client pool timed out
	ErrTimeout = errors.New("grpc pool: client pool timed out")
	// ErrAlreadyClosed is the error when the client conn was already closed
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	// ErrFullPool is the error when the pool is already full
	ErrFullPool = errors.New("grpc pool: closing a ClientConn into a full pool")
)

// ClientFactory is a function type creating a grpc client
type ClientFactory func() (*grpc.ClientConn, error)

// Pool is the grpc client pool
type Pool struct {
	clientWrappers  chan PooledClientConn
	initCap, maxCap int
	factory         ClientFactory
	idleTimeout     time.Duration
	maxLifeDuration time.Duration
	mu              sync.RWMutex
}

// Option is used to configure a pool.
type Option func(*Pool) error

// WithInitialCapacity configures the initial capacity of the pool.
func WithInitialCapacity(initCap int) Option {
	return func(p *Pool) error {
		if initCap < 1 {
			return errors.New("grpc pool: initial capacity must be greater than 0")
		}
		p.initCap = initCap
		return nil
	}
}

// WithMaxCapacity configures the max capacity of the pool.
func WithMaxCapacity(maxCap int) Option {
	return func(p *Pool) error {
		if maxCap < 1 {
			return errors.New("grpc pool: max capacity must be greater than 0")
		}
		p.maxCap = maxCap
		return nil
	}
}

// WithMaxLifeDuration configures the max life time of a gRPC connection.
func WithMaxLifeDuration(maxLifeDuration time.Duration) Option {
	return func(p *Pool) error {
		p.maxLifeDuration = maxLifeDuration
		return nil
	}
}

// WithIdleTimeout configures the idle timeout for a gRPC connection.
func WithIdleTimeout(idleTimeout time.Duration) Option {
	return func(p *Pool) error {
		p.idleTimeout = idleTimeout
		return nil
	}
}

// New creates a new client pool using the given options and returns an error
// if the initial clients could not be created.
func New(factory ClientFactory, opts ...Option) (*Pool, error) {
	// Create the pool with default values
	p := &Pool{
		initCap:     1,
		maxCap:      10,
		factory:     factory,
		idleTimeout: 3 * time.Minute,
	}

	// Apply the options
	for _, opt := range opts {
		if opt != nil {
			if err := opt(p); err != nil {
				return nil, err
			}
		}
	}

	// Adjust if needed
	if p.initCap > p.maxCap {
		return nil, errors.New("grpc pool: initial capacity cannot be greater than max capacity")
	}

	// Create the channel for the pooled client connections
	p.clientWrappers = make(chan PooledClientConn, p.maxCap)

	// Populate the pool with initial clients
	for range p.initCap {
		c, err := factory()
		if err != nil {
			return nil, err
		}

		p.clientWrappers <- PooledClientConn{
			ClientConn:    c,
			pool:          p,
			timeLastUsed:  time.Now(),
			timeInitiated: time.Now(),
		}
	}

	// Fill the rest of the pool with empty clients
	for range p.maxCap - p.initCap {
		p.clientWrappers <- PooledClientConn{
			pool: p,
		}
	}

	return p, nil
}

func (p *Pool) getClientWrappers() chan PooledClientConn {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.clientWrappers
}

// Close empties the pool calling Close on all its clients.
// You can call Close while there are outstanding clients.
// It waits for all clients to be returned (Close).
// The pool channel is then closed, and Get will not be allowed anymore
func (p *Pool) Close() error {
	p.mu.Lock()
	clientWrappers := p.clientWrappers
	p.clientWrappers = nil
	p.mu.Unlock()

	if clientWrappers == nil {
		return nil
	}

	// Close the channel
	close(clientWrappers)

	for clientWrapper := range clientWrappers {
		if clientWrapper.ClientConn == nil {
			continue
		}
		if err := clientWrapper.closeGRPCConn(); err != nil {
			return err
		}
	}
	return nil
}

// IsClosed returns true if the client pool is closed.
func (p *Pool) IsClosed() bool {
	return p == nil || p.getClientWrappers() == nil
}

// Get will return the next available client. If capacity
// has not been reached, it will create a new one using the factory. Otherwise,
// it will wait till the next client becomes available or the context was canceled.
func (p *Pool) Get(ctx context.Context) (*PooledClientConn, error) {
	clientWrappers := p.getClientWrappers()
	if clientWrappers == nil {
		return nil, ErrClosed
	}

	pcc := PooledClientConn{
		pool: p,
	}

	select {
	case pcc = <-clientWrappers:
		// All good
	case <-ctx.Done():
		return nil, ErrTimeout
	}

	// Return it, if it is healthy
	if pcc.isHealthy() {
		return &pcc, nil
	}

	// Otherwise create a new one
	// But first make sure we close the underlying gRPC connection
	if err := pcc.closeGRPCConn(); err != nil {
		return nil, err
	}

	// Then use the factory to create the new gRPC connection
	var err error
	pcc.ClientConn, err = p.factory()
	if err != nil {
		// Pass back an empty one
		clientWrappers <- PooledClientConn{pool: p}
		return nil, err
	}

	// Initialize and return it
	pcc.timeInitiated = time.Now()
	pcc.timeLastUsed = time.Now()
	return &pcc, err
}

// PooledClientConn is a wrapper for a grpc client conn
type PooledClientConn struct {
	*grpc.ClientConn
	pool          *Pool
	timeLastUsed  time.Time
	timeInitiated time.Time
	unhealthy     bool
}

// isHealthy verifies if the connection is healthy
func (pcc *PooledClientConn) isHealthy() bool {
	got := !pcc.isUnhealthy()
	return got
}

// isUnhealthy verifies if the connection is unhealthy
func (pcc *PooledClientConn) isUnhealthy() bool {
	if pcc.unhealthy {
		return true
	}
	if pcc.ClientConn == nil {
		pcc.unhealthy = true
		return true
	}
	// If the pooled client connection has been idle for too long, we want to recycle it.
	idleTimeout := pcc.pool.idleTimeout
	if idleTimeout > 0 && pcc.timeLastUsed.Add(idleTimeout).Before(time.Now()) {
		pcc.unhealthy = true
		return true
	}
	// If the pooled client connection has become too old, we want to recycle it.
	maxLifeDuration := pcc.pool.maxLifeDuration
	if maxLifeDuration > 0 && pcc.timeInitiated.Add(maxLifeDuration).Before(time.Now()) {
		pcc.unhealthy = true
		return true
	}
	return false
}

// MarkUnhealthy marks the client conn as unhealthy, so that the connection
// gets reset when closed
func (pcc *PooledClientConn) MarkUnhealthy() {
	pcc.unhealthy = true
}

// Close returns a ClientConn to the pool. It is safe to call multiple time,
// but will return an error after first time
func (pcc *PooledClientConn) Close() error {
	if pcc == nil {
		return nil
	}

	if pcc.ClientConn == nil {
		return ErrAlreadyClosed
	}

	if pcc.pool.IsClosed() {
		if err := pcc.closeGRPCConn(); err != nil {
			return err
		}
		return ErrClosed
	}

	// Create the new pooled client connection handing over our gRPC connection
	newpcc := PooledClientConn{
		pool:          pcc.pool,
		ClientConn:    pcc.ClientConn,
		timeInitiated: pcc.timeInitiated,
		timeLastUsed:  time.Now(),
	}
	pcc.ClientConn = nil

	// Pass it back to the pool
	select {
	case pcc.pool.clientWrappers <- newpcc:
		// Successfully returned into the pool
	default:
		if err := pcc.closeGRPCConn(); err != nil {
			return err
		}
		return ErrFullPool
	}

	return nil
}

func (pcc *PooledClientConn) closeGRPCConn() error {
	if pcc.ClientConn == nil {
		return nil
	}
	currConn := pcc.ClientConn
	pcc.ClientConn = nil
	return currConn.Close()
}
