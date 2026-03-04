package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You are now connected to Server 1.")
	})

	fmt.Printf("Starting Server 1 at port 8081\n")
	if err := http.ListenAndServe("localhost:8081", nil); err != nil {
		log.Fatal(err)
	}
}
