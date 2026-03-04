package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You are now connected to Server 3.")
	})

	fmt.Printf("Starting Server 3 at port 8083\n")
	if err := http.ListenAndServe("localhost:8083", nil); err != nil {
		log.Fatal(err)
	}
}
