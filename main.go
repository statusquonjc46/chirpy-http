package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/statusquonjc46/chirpy-http/internal/auth"
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

// Takes a POST request to create a user, adds to the users table, then returns the users row from the DB
func (cfg *apiConfig) addUserHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type returnErrors struct {
		Error string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rtn := &returnErrors{Error: "something went wrong"}
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

	//Check to see if email or password are empty. Then get Email and Password from POST request, hash password
	if params.Email == "" {
		rtn := &returnErrors{Error: "Email is empty."}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		return
	}

	if params.Password == "" {
		rtn := &returnErrors{Error: "Password is empty."}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		return
	}

	email := sql.NullString{String: params.Email, Valid: true}
	password := params.Password

	hash, err := auth.HashPassword(password)
	if err != nil {
		rtn := &returnErrors{Error: "Failed to hash password."}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal password hash error: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(dat)
		fmt.Printf("Error hashing password: %s\n", err)
		return
	}

	userParams := database.CreateUserParams{
		Email:          email,
		HashedPassword: hash,
	}

	user, err := cfg.database.CreateUser(r.Context(), userParams)
	if err != nil {
		rtn := &returnErrors{Error: "failed to add user to db"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json for adding user to DB: %s\n", err)
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
		fmt.Printf("Error marshalling json for user Struct: %s\n", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(dat)
	fmt.Printf("%+v", ret)
}

// Perform User Authentication/Login
func (cfg *apiConfig) userLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type returnErrors struct {
		Error string `json:"error"`
	}

	//decode POST request
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rtn := &returnErrors{Error: "Unable to decode json POST request."}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal user login error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(dat)
		return
	}

	if params.Email == "" {
		rtn := &returnErrors{Error: "Incorrect email or password"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal userLogin email check: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		w.Write(dat)
		return
	}

	if params.Password == "" {
		rtn := &returnErrors{Error: "Incorrect email or password"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal userLogin password check: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		w.Write(dat)
		return
	}

	email := sql.NullString{String: params.Email, Valid: true}

	getUser, err := cfg.database.UserandHashLookup(r.Context(), email)
	if err != nil {
		rtn := &returnErrors{Error: "Incorrect email or password"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal DB lookup error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		w.Write(dat)
		return
	}

	err = auth.CheckPasswordHash(getUser.HashedPassword, params.Password)
	if err != nil {
		rtn := &returnErrors{Error: "Incorrect email or password"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal password hash check error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		w.Write(dat)
		return
	}

	authedUser := &User{
		ID:        getUser.ID,
		CreatedAt: getUser.CreatedAt,
		UpdatedAt: getUser.UpdatedAt,
		Email:     getUser.Email.String,
	}

	dat, err := json.Marshal(authedUser)
	if err != nil {
		rtn := &returnErrors{Error: "Error marshaling authed user struct"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal, the failed marshal of authed user: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(dat)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(dat)
	fmt.Printf("%+v", authedUser)

}

// validates chirp char lengths, censors banned words, then puts the full chirp in the chirp DB, and returns the full chirp
func (cfg *apiConfig) addChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string `json:"body"`
		UserID string `json:"user_id"`
	}
	type returnErr struct {
		Error string `json:"error"`
	}
	type chirpParams struct {
		Body   string
		UserID uuid.UUID
	}

	//Decode POST data
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		rtn := &returnErr{Error: "something went wrong"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json for POST data %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(dat)
		fmt.Printf("Error decoding parameters: %s\n", err)
		return
	}

	//check for banned words, then return the cleaned string
	strBody := params.Body
	userID, err := uuid.Parse(params.UserID) //parse string to UUID
	if err != nil {
		rtn := &returnErr{Error: "unable to parse uuid from json POST"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Error marshalling json for chirp parameters %s\n", err)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
		fmt.Printf("UserID was not able to be parses from string to UUID: %s\n", err)
		return
	}

	//valid chirp logic
	chirpLen := len(strBody) //get length of body to check if 140 chars
	if chirpLen <= 140 {     //if less than or equal to 140, check for banned words, create a cleaned body
		bannedWords := []string{"kerfuffle", "sharbert", "fornax"}
		censor := "****"
		cleanBody := strBody
		for _, sub := range bannedWords {
			uppedSub := strings.ToUpper(string(sub[0])) + sub[1:]
			if strings.Contains(cleanBody, sub) {
				cleanBody = strings.Replace(cleanBody, sub, censor, -1)
			} else if strings.Contains(cleanBody, strings.ToUpper(sub)) {
				cleanBody = strings.Replace(cleanBody, sub, censor, -1)
			} else if strings.Contains(cleanBody, uppedSub) {
				cleanBody = strings.Replace(cleanBody, uppedSub, censor, -1)
			}
		}
		fmt.Println(cleanBody)

		//insert chirp to DB, save chirp to Chirp struct, r.context for ID, CreatedAt, UpdatedAt, cleanBody for cleanedbody
		addChirpParams := database.AddChirpParams{Body: cleanBody, UserID: uuid.NullUUID{UUID: userID, Valid: true}}
		createChirp, err := cfg.database.AddChirp(r.Context(), addChirpParams)
		if err != nil {
			rtn := &returnErr{Error: "Failed to Add Chirp to DB"}
			dat, err := json.Marshal(rtn)
			if err != nil {
				fmt.Printf("Error marshalling chirp DB failure: %s\n", err)
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			w.Write(dat)
			return
		}
		//get user_id from DB, check if Valid is true before setting user_id in chirp struct
		var userID uuid.UUID
		if createChirp.UserID.Valid {
			userID = createChirp.UserID.UUID
		} else {
			userID = uuid.Nil
			fmt.Printf("Error: user id is nil: %s", userID)
		}

		//create chirp instance with chirp data
		chirp := &Chirp{
			ID:        createChirp.ID,
			CreatedAt: createChirp.CreatedAt,
			UpdatedAt: createChirp.UpdatedAt,
			Body:      createChirp.Body,
			UserID:    userID,
		}
		//marshal chirp, return the chirp, or return error
		dat, err := json.Marshal(chirp)

		if err != nil {
			fmt.Printf("Error marshalling json for chirp: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(dat)
		fmt.Printf("Chirp added to DB successfully\nChirp: %s | Length: %d \n", strBody, chirpLen)
	} else { //chirp length is too logn error response
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

// Get all Chirps from chirps table, return the array of chirps
func (cfg *apiConfig) getAllChirps(w http.ResponseWriter, r *http.Request) {
	type returnErrors struct {
		Error string `json:"error"`
	}

	allChirps, err := cfg.database.GetAllChirps(r.Context())
	if err != nil {
		rtn := &returnErrors{Error: "Failed to query DB for all chirps"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal all chirp query error: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(dat)
		return
	}

	var jsonFormattedChirps []Chirp
	for _, row := range allChirps {

		var userID uuid.UUID
		if row.UserID.Valid {
			userID = row.UserID.UUID
		} else {
			fmt.Printf("Error: user id is nil: %s", userID)
			continue
		}

		ch := Chirp{
			ID:        row.ID,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
			Body:      row.Body,
			UserID:    userID,
		}

		jsonFormattedChirps = append(jsonFormattedChirps, ch)
	}

	returnChirp, err := json.Marshal(jsonFormattedChirps)
	if err != nil {
		rtn := &returnErrors{Error: "Failed to marshal Array of Chirps"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal Array of Chirps error: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(dat)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(returnChirp)
}

// Get a single ID specifc Chirp if it exists
func (cfg *apiConfig) getSpecificChirp(w http.ResponseWriter, r *http.Request) {
	type returnErrors struct {
		Error string `json:"error"`
	}

	chirpID, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		rtn := &returnErrors{Error: "Failed to convert string Id from PathValue to uuid.UUID"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal string ID to uuid.UUID error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(dat)
		return
	}

	chirpAtID, err := cfg.database.GetSpecificChirp(r.Context(), chirpID)
	if err != nil {
		rtn := &returnErrors{Error: "Failed to query DB for chirpID"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal error from getting specific chirp from DB: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(dat)
		return
	}

	var userID uuid.UUID
	if chirpAtID.UserID.Valid {
		userID = chirpAtID.UserID.UUID
	} else {
		fmt.Printf("Error: user id is nil: %s\n", userID)
	}
	ch := Chirp{
		ID:        chirpAtID.ID,
		CreatedAt: chirpAtID.CreatedAt,
		UpdatedAt: chirpAtID.UpdatedAt,
		Body:      chirpAtID.Body,
		UserID:    userID,
	}

	returnChirp, err := json.Marshal(ch)
	if err != nil {
		rtn := &returnErrors{Error: "Failed to return Chirp"}
		dat, err := json.Marshal(rtn)
		if err != nil {
			fmt.Printf("Failed to marshal Returning Chirp Error: %s\n", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(dat)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(returnChirp)
	fmt.Printf("%+v", ch)
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
	ID             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"hashed_password"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
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
	mux.HandleFunc("POST /api/chirps", cfg.addChirp)
	mux.HandleFunc("POST /api/users", cfg.addUserHandler)
	mux.HandleFunc("GET /api/chirps", cfg.getAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.getSpecificChirp)
	mux.HandleFunc("POST /api/login", cfg.userLogin)

	//Serve content on connection
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed at ListenAndServe: %s", err)
	}
}
