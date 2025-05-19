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

	err := server.ListenAndServe()

	if err != nil {
		fmt.Printf("Failed at ListenAndServe: %s", err)
	}
}
