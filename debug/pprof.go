package debug

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

func StartPprofServer(addr string) {
	if addr == "" {
		return
	}

	go func() {
		log.Printf("pprof listening on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("pprof stopped: %v", err)
		}
	}()
}
