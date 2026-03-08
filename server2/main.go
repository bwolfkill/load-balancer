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
		fmt.Fprintf(w, "You are now connected to Server 2.")
	})
	http.HandleFunc("/healthz", getHealthCheckHandler())

	fmt.Printf("Starting Server 2 at port 8082\n")
	if err := http.ListenAndServe("localhost:8082", nil); err != nil {
		log.Fatal(err)
	}
}
