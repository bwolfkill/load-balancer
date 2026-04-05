// Package main is a minimal HTTP backend used for local development.
// It is not part of the load balancer itself — run all three testservers
// alongside the load balancer to simulate a real backend pool locally.
package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

var healthy atomic.Bool

func getHealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !healthy.Load() {
			http.Error(w, "Unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "OK")
	}
}

func main() {
	healthy.Store(true)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "You are now connected to Server 2.")
	})
	http.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(25 * time.Second)
		_, _ = fmt.Fprintf(w, "Slow response from Server 2.")
	})
	http.HandleFunc("/sethealthy", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("healthy") == "false" {
			healthy.Store(false)
			_, _ = fmt.Fprintf(w, "Server 2 set to unhealthy")
		} else {
			healthy.Store(true)
			_, _ = fmt.Fprintf(w, "Server 2 set to healthy")
		}
	})
	http.HandleFunc("/healthz", getHealthCheckHandler())

	log.Printf("Starting Server 2 at port 8082\n")
	if err := http.ListenAndServe(":8082", nil); err != nil {
		log.Fatal(err)
	}
}
