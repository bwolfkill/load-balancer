package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You are now connected to Server 2.")
	})

	fmt.Printf("Starting Server 2 at port 8082\n")
	if err := http.ListenAndServe("localhost:8082", nil); err != nil {
		log.Fatal(err)
	}
}
