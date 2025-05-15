package prof

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
)

func TestBToMb(t *testing.T) {
	// create the test cases
	tests := []struct {
		name string
		b    uint64
		want uint64
	}{
		{"1KB", 1 * 1024, 0},
		{"1MB", 1 * 1024 * 1024, 1},
		{"2GB", 2 * 1024 * 1024 * 1024, 2048},
		{"3TB", 3 * 1024 * 1024 * 1024 * 1024, 3145728},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := bToMb(tc.b)

			// Assert
			if got != tc.want {
				t.Errorf("expected: %+v, got: %+v", tc.want, got)
			}
		})
	}
}

func TestRegisterPProfHandlers(t *testing.T) {
	// Arrange
	mux := http.NewServeMux()
	// Act
	RegisterPProfHandlers(mux)

	// create the test cases
	tests := []struct {
		name        string
		expectEmpty bool
	}{
		{name: "/adfv/", expectEmpty: true},
		{name: "/debug/pprof/"},
		{name: "/debug/pprof/"},
		{name: "/debug/pprof/cmdline"},
		{name: "/debug/pprof/profile"},
		{name: "/debug/pprof/symbol"},
		{name: "/debug/pprof/trace"},
		{name: "/debug/pprof/mem"},
		{name: "/debug/pprof/block"},
		{name: "/debug/pprof/goroutine"},
		{name: "/debug/pprof/heap"},
		{name: "/debug/pprof/threadcreate"},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Assert
			_, got := mux.Handler(&http.Request{URL: &url.URL{Path: tc.name}})
			if tc.expectEmpty {
				if got != "" {
					t.Errorf("expected: empty, got: %+v", got)
				}
			} else {
				if got != tc.name {
					t.Errorf("expected: %+v, got: %+v", tc.name, got)
				}
			}
		})
	}
}

func TestWriteMemUsage(t *testing.T) {
	// Arrange
	re := regexp.MustCompile(`^Alloc = [0-9]+ MiB\|TotalAlloc = [0-9]+ MiB\|Sys = [0-9]+ MiB\|NumGC = [0-9]+\n$`)
	r := &http.Request{}
	rr := httptest.NewRecorder()

	// Act
	writeMemUsage(rr, r)
	body := rr.Body.String()

	// Assert
	if rr.Code != http.StatusOK {
		t.Errorf("expected: %+v, got: %+v", http.StatusOK, rr.Code)
	}

	if !re.MatchString(body) {
		t.Errorf("expected: %+v, got: %+v", re.String(), body)
	}
}
