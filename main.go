package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/statusquonjc46/chirpy-http/internal/database"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// HANDLERS FOR ENDPOINTS
// Server Health Function
func healthHandler(w http.ResponseWriter, r *http.Request) {
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
	platform := cfg.platform
	if platform != "dev" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache control", "no-cache")
		w.WriteHeader(403)
		w.Write([]byte("403 Forbidden"))
		return
	}
	cfg.database.DeleteUsers(r.Context())
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) addUserHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}
	type Errors struct {
		Error string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rtn := &Errors{Error: "something went wrong"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error decoding json %s\n", err)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		fmt.Printf("Error decoding parameters: %s\n", err)
		return
	}

	email := sql.NullString{String: params.Email, Valid: true}
	fmt.Println(email)
	user, err := cfg.database.CreateUser(r.Context(), email)
	fmt.Println(user)
	if err != nil {
		rtn := &Errors{Error: "failed to add user to db"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(dat)
		fmt.Printf("Error adding user to DB: %s\n", err)
		return
	}
	ret := &User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email.String,
	}

	dat, err := json.Marshal(ret)
	if err != nil {
		fmt.Printf("Error marshalling json: %s\n", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(dat)
}

// validates chirp char lengths
func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type returnErr struct {
		Error string `json:"error"`
	}
	type returnClean struct {
		CleanedBody string `json:"cleaned_body"`
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
	strBody := params.Body
	chirpLen := len(strBody)
	if chirpLen < 140 {
		bannedWords := []string{"kerfuffle", "sharbert", "fornax"}
		censor := "****"
		rtnStr := strBody
		for _, sub := range bannedWords {
			uppedSub := strings.ToUpper(string(sub[0])) + sub[1:]
			fmt.Println(uppedSub)
			if strings.Contains(rtnStr, sub) {
				rtnStr = strings.Replace(rtnStr, sub, censor, -1)
			} else if strings.Contains(rtnStr, strings.ToUpper(sub)) {
				rtnStr = strings.Replace(rtnStr, sub, censor, -1)
			} else if strings.Contains(rtnStr, uppedSub) {
				rtnStr = strings.Replace(rtnStr, uppedSub, censor, -1)
				fmt.Println("uppedSub triggered")
			}
		}
		fmt.Println(rtnStr)
		rtn := &returnClean{CleanedBody: rtnStr}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(dat)
		fmt.Printf("Chirp: %s | Length: %d\n", strBody, chirpLen)
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
	database       *database.Queries
	platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func main() {
	//var instantiation
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux() //instantiate the server mux
	server := &http.Server{   //create the http server
		Addr:    ":8080",
		Handler: mux,
	}

	cfg := &apiConfig{} //instantiate an instance of apiConfig struct
	dbURL := os.Getenv("DB_URL")
	cfg.platform = os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)
	cfg.database = dbQueries

	fmt.Printf("Attempting to serve at: %s\n", server.Addr)

	//connection handlers/rputers
	mux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", healthHandler)
	mux.HandleFunc("GET /admin/metrics", cfg.metricHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	mux.HandleFunc("POST /api/users", cfg.addUserHandler)

	//Serve content on connection
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed at ListenAndServe: %s", err)
	}
}
