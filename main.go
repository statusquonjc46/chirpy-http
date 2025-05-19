package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Printf("Attempting to serve at: %s", server.Addr)

	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.Handle("/assets", http.FileServer(http.Dir("/assets/logo.png")))
	err := server.ListenAndServe()

	if err != nil {
		fmt.Printf("Failed at ListenAndServe: %s", err)
	}
}
