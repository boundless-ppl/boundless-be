package debug

import (
	"log"
	"net/http"
	"net/http/pprof"
	"net/netip"
)

func StartPprofServer(addr string) {
	if addr == "" {
		return
	}

	mux := http.NewServeMux()
	registerPprofRoutes(mux)

	go func() {
		log.Printf("pprof listening on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("pprof stopped: %v", err)
		}
	}()
}

func registerPprofRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", protectDebugEndpoint(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", protectDebugEndpoint(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", protectDebugEndpoint(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", protectDebugEndpoint(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", protectDebugEndpoint(pprof.Trace))
	mux.Handle("/debug/pprof/allocs", protectDebugEndpoint(pprof.Handler("allocs").ServeHTTP))
	mux.Handle("/debug/pprof/block", protectDebugEndpoint(pprof.Handler("block").ServeHTTP))
	mux.Handle("/debug/pprof/goroutine", protectDebugEndpoint(pprof.Handler("goroutine").ServeHTTP))
	mux.Handle("/debug/pprof/heap", protectDebugEndpoint(pprof.Handler("heap").ServeHTTP))
	mux.Handle("/debug/pprof/mutex", protectDebugEndpoint(pprof.Handler("mutex").ServeHTTP))
	mux.Handle("/debug/pprof/threadcreate", protectDebugEndpoint(pprof.Handler("threadcreate").ServeHTTP))
}

func protectDebugEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAllowedDebugClient(r.RemoteAddr) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func isAllowedDebugClient(remoteAddr string) bool {
	addr, err := netip.ParseAddrPort(remoteAddr)
	if err == nil {
		return isAllowedDebugIP(addr.Addr())
	}

	host, err := netip.ParseAddr(remoteAddr)
	if err != nil {
		return false
	}

	return isAllowedDebugIP(host)
}

func isAllowedDebugIP(addr netip.Addr) bool {
	return addr.IsLoopback() || addr.IsPrivate()
}
