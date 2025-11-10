package grpcpool_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/openkcm/common-sdk/pkg/grpcpool"
)

func TestNew(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(3),
		grpcpool.WithIdleTimeout(time.Hour))
	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}

	if a := p.Available(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

	if a := p.Capacity(); a != 3 {
		t.Errorf("The pool capacity was %d but should be 3", a)
	}

	// Get a client
	client, err := p.Get(t.Context())
	if err != nil {
		t.Errorf("Get returned an error: %s", err.Error())
	}

	if client == nil {
		t.Error("client was nil")
	}

	if a := p.Available(); a != 2 {
		t.Errorf("The pool available was %d but should be 2", a)
	}

	if a := p.Capacity(); a != 3 {
		t.Errorf("The pool capacity was %d but should be 3", a)
	}

	// Return the client
	err = client.Close()
	if err != nil {
		t.Errorf("Close returned an error: %s", err.Error())
	}

	if a := p.Available(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

	if a := p.Capacity(); a != 3 {
		t.Errorf("The pool capacity was %d but should be 3", a)
	}

	// Attempt to return the client again
	err = client.Close()
	if !errors.Is(err, grpcpool.ErrAlreadyClosed) {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			grpcpool.ErrAlreadyClosed.Error(), err.Error())
	}

	// Take 3 clients
	cl1, err1 := p.Get(t.Context())
	cl2, err2 := p.Get(t.Context())
	cl3, err3 := p.Get(t.Context())

	if err1 != nil {
		t.Errorf("Err1 was not nil: %s", err1.Error())
	}

	if err2 != nil {
		t.Errorf("Err2 was not nil: %s", err2.Error())
	}

	if err3 != nil {
		t.Errorf("Err3 was not nil: %s", err3.Error())
	}

	if a := p.Available(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	if a := p.Capacity(); a != 3 {
		t.Errorf("The pool capacity was %d but should be 3", a)
	}

	// Returning all of them
	err1 = cl1.Close()
	if err1 != nil {
		t.Errorf("Close returned an error: %s", err1.Error())
	}

	err2 = cl2.Close()
	if err2 != nil {
		t.Errorf("Close returned an error: %s", err2.Error())
	}

	err3 = cl3.Close()
	if err3 != nil {
		t.Errorf("Close returned an error: %s", err3.Error())
	}
}

func TestFactoryError(t *testing.T) {
	_, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return nil, errors.New("error")
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(3),
	)
	if err == nil {
		t.Error("The pool should have returned an error")
	}
}

func TestCapacity(t *testing.T) {
	// create the test cases
	tests := []struct {
		name        string
		initCap     int
		maxCap      int
		wantError   bool
		wantInitCap int
		wantMaxCap  int
	}{
		{
			name:        "valid capacity",
			initCap:     1,
			maxCap:      1,
			wantInitCap: 1,
			wantMaxCap:  1,
		}, {
			name:      "init cap too low",
			initCap:   0,
			maxCap:    1,
			wantError: true,
		}, {
			name:      "max cap too low",
			initCap:   1,
			maxCap:    0,
			wantError: true,
		}, {
			name:      "max cap smaller than init cap",
			initCap:   10,
			maxCap:    5,
			wantError: true,
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
				return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
			},
				grpcpool.WithInitialCapacity(tc.initCap),
				grpcpool.WithMaxCapacity(tc.maxCap),
			)

			// Assert
			if tc.wantError {
				if err == nil {
					t.Error("expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				} else {
					if p.Available() != tc.wantInitCap {
						t.Errorf("expected init cap %d, but got %d", tc.wantInitCap, p.Available())
					}

					if p.Capacity() != tc.wantMaxCap {
						t.Errorf("expected max cap %d, but got %d", tc.wantMaxCap, p.Capacity())
					}
				}
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
	)
	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}

	_, err = p.Get(t.Context())
	if err != nil {
		t.Errorf("Get returned an error: %s", err.Error())
	}

	if a := p.Available(); a != 0 {
		t.Errorf("The pool available was %d but expected 0", a)
	}

	// We want to fetch a second one, with a timeout. If the timeout was
	// omitted, the pool would wait indefinitely as it'd wait for another
	// client to get back into the queue
	ctx, canceled := context.WithDeadline(t.Context(), time.Now().Add(10*time.Millisecond))
	defer canceled()

	_, err2 := p.Get(ctx)
	if !errors.Is(err2, grpcpool.ErrTimeout) {
		t.Errorf("Expected error \"%s\" but got \"%s\"", grpcpool.ErrTimeout, err2.Error())
	}
}

func TestMaxLifeDuration(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
		grpcpool.WithMaxLifeDuration(1),
	)
	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}

	c, err := p.Get(t.Context())
	if err != nil {
		t.Errorf("Get returned an error: %s", err.Error())
		t.Fail()
	}

	// The max life of the connection was very low (1ns), so when we close
	// the connection it should get marked as unhealthy
	err = c.Close()
	if err != nil {
		t.Errorf("Close returned an error: %s", err.Error())
	}

	if c.IsHealthy() {
		t.Errorf("the connection should've been marked as unhealthy")
	}

	// Let's also make sure we don't prematurely close the connection
	count := 0

	p, err = grpcpool.New(func() (*grpc.ClientConn, error) {
		count++
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
		grpcpool.WithMaxLifeDuration(time.Minute),
	)
	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}

	for range 3 {
		c, err = p.Get(t.Context())
		if err != nil {
			t.Errorf("Get returned an error: %s", err.Error())
		}

		// The max life of the connection is high, so when we close
		// the connection it shouldn't be marked as unhealthy
		err := c.Close()
		if err != nil {
			t.Errorf("Close returned an error: %s", err.Error())
		}

		if c.IsHealthy() {
			t.Errorf("the connection shouldn't have been marked as unhealthy")
		}
	}

	// Count should have been 1 as dial function should only have been called once
	if count > 1 {
		t.Errorf("Dial function has been called multiple times")
	}
}

func TestClose(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
	)
	if err != nil {
		t.Fatalf("The pool returned an error: %s", err.Error())
	}

	err = p.Close()
	if err != nil {
		t.Fatalf("Close returned an error: %s", err.Error())
	}

	p = &grpcpool.Pool{}

	err = p.Close()
	if err != nil {
		t.Fatalf("Close returned an error: %s", err.Error())
	}
}

func TestIsUnhealthyDueToMaxLife(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
		grpcpool.WithMaxLifeDuration(1),
	)
	if err != nil {
		t.Fatalf("The pool returned an error: %s", err.Error())
	}

	cw, err := p.Get(t.Context())
	if err != nil {
		t.Fatalf("Get returned an error: %s", err.Error())
	}

	if cw.IsHealthy() {
		t.Fatalf("the connection should've been marked as unhealthy")
	}
}
func TestIsUnhealthyDueToIdleTimeout(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
		grpcpool.WithIdleTimeout(1),
	)
	if err != nil {
		t.Fatalf("The pool returned an error: %s", err.Error())
	}

	cw, err := p.Get(t.Context())
	if err != nil {
		t.Fatalf("Get returned an error: %s", err.Error())
	}

	if cw.IsHealthy() {
		t.Fatalf("the connection should've been marked as unhealthy")
	}
}

func TestMarkUnhealthy(t *testing.T) {
	p, err := grpcpool.New(func() (*grpc.ClientConn, error) {
		return grpc.NewClient("example.com", grpc.WithTransportCredentials(insecure.NewCredentials()))
	},
		grpcpool.WithInitialCapacity(1),
		grpcpool.WithMaxCapacity(1),
	)
	if err != nil {
		t.Fatalf("The pool returned an error: %s", err.Error())
	}

	cw, err := p.Get(t.Context())
	if err != nil {
		t.Fatalf("Get returned an error: %s", err.Error())
	}

	cw.MarkUnhealthy()

	if cw.IsHealthy() {
		t.Fatalf("the connection should've been marked as unhealthy")
	}
}
