package prof

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
)

const (
	KB = 1024
)

// RegisterPProfHandlers registers the pprof handlers to the provided ServeMux.
// This is used for the status server.
func RegisterPProfHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/debug/pprof/mem", writeMemUsage)

	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
}

// writeMemUsage writes the memory usage to the provided ResponseWriter.
func writeMemUsage(w http.ResponseWriter, _ *http.Request) {
	var m runtime.MemStats

	runtime.ReadMemStats(&m)
	s := fmt.Sprintf("Alloc = %v MiB|TotalAlloc = %v MiB|Sys = %v MiB|NumGC = %v\n",
		bToMb(m.Alloc),
		bToMb(m.TotalAlloc),
		bToMb(m.Sys), m.NumGC)

	_, err := w.Write([]byte(s))
	if err != nil {
		return
	}
}

// bToMb converts bytes to megabytes.
func bToMb(b uint64) uint64 {
	return b / (KB * KB)
}
