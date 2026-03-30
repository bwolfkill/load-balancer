// Package main is a minimal HTTP backend used for local development.
// It is not part of the load balancer itself — run all three testservers
// alongside the load balancer to simulate a real backend pool locally.
package main

import (
	"fmt"
	"log"
	"net/http"
)

func getHealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You are now connected to Server 3.")
	})
	http.HandleFunc("/healthz", getHealthCheckHandler())

	log.Printf("Starting Server 3 at port 8083\n")
	if err := http.ListenAndServe("localhost:8083", nil); err != nil {
		log.Fatal(err)
	}
}
