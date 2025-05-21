package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Server Health Function
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) metricHandler(w http.ResponseWriter, r *http.Request) {
	hits := fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(hits))

}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	//vars
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	cfg := &apiConfig{}

	fmt.Printf("Attempting to serve at: %s", server.Addr)

	//connection handlers
	mux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /healthz", HealthHandler)
	mux.HandleFunc("GET /metrics", cfg.metricHandler)
	mux.HandleFunc("POST /reset", cfg.resetHandler)
	//Serve content on connection
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed at ListenAndServe: %s", err)
	}
}
