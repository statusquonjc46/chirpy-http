package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

// HANDLERS FOR ENDPOINTS
// Server Health Function
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Handles the endpoint to count site visits until server reboots, serves html to the page
func (cfg *apiConfig) metricHandler(w http.ResponseWriter, r *http.Request) {
	hits := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(hits))

}

// Resets the count on /metrics instead of neededing to restart server
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// validates chirp char lengths
func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type returnErr struct {
		Error string `json:"error"`
	}
	type returnValid struct {
		Valid bool `json:"valid"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rtn := &returnErr{Error: "something went wrong"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(dat)
		fmt.Printf("Error decoding parameters: %s\n", err)
		return
	}

	chirpLen := len(params.Body)
	if chirpLen < 140 {
		rtn := &returnValid{Valid: true}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(dat)
		fmt.Printf("Chirp: %s | Length: %d\n", params.Body, chirpLen)
	} else {
		overage := chirpLen - 140
		rtn := &returnErr{Error: "chirp is too long"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache control", "no-cache")
		w.WriteHeader(400)
		w.Write(dat)
		fmt.Printf("Error: %d is greater than 140 characters by %d\n", chirpLen, overage)
		return
	}
}

// MIDDLEWARE
// middleware to do the actual counting of site visits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// struct for api site hits
type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	//var instantiation
	mux := http.NewServeMux() //instantiate the server mux
	server := &http.Server{   //create the http server
		Addr:    ":8080",
		Handler: mux,
	}
	cfg := &apiConfig{} //instantiate an instance of apiConfig struct

	fmt.Printf("Attempting to serve at: %s\n", server.Addr)

	//connection handlers/rputers
	mux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", HealthHandler)
	mux.HandleFunc("GET /admin/metrics", cfg.metricHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)

	//Serve content on connection
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed at ListenAndServe: %s", err)
	}
}
