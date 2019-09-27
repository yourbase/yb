package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/yourbase/yb/plumbing/log"

	_ "net/http/pprof"
)

func DaemonKickoff() (err error) {
	go func() {
		// TODO add Prometheus
		listenPort := "6060"
		listenIp := "0.0.0.0"

		listenHostPort := fmt.Sprintf("%s:%s", listenIp, listenPort)

		srv := &http.Server{
			Handler: http.DefaultServeMux,
			Addr:    listenHostPort,
			// Good practice: enforce timeouts for servers you create!
			WriteTimeout: 60 * time.Second,
			ReadTimeout:  60 * time.Second,
		}

		// TODO Maybe use other logging facility
		err = srv.ListenAndServe()
		log.Fatal(err)
	}()
	return
}
