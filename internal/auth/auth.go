package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("Failed to hash password: %s.", err)
		return "", err
	}

	return string(hashed), nil
}

func CheckPasswordHash(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		fmt.Printf("Password is incorrect: %s\n", err)
		return err
	}
	fmt.Printf("Password is correct.\n")
	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string) (string, error) {
	mySigningKey := []byte(tokenSecret)

	issuedUTC := time.Now().UTC()
	expiresUTC := time.Now().UTC().Add(time.Duration(3600) * time.Second)
	// Create the Claims
	registeredClaims := &jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(issuedUTC),
		ExpiresAt: jwt.NewNumericDate(expiresUTC),
		Issuer:    "chirpy",
		Subject:   userID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, registeredClaims)
	ss, err := token.SignedString(mySigningKey)
	if err != nil {
		return "", err
	}
	fmt.Println(ss, err)
	return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claimStruct := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claimStruct, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	} else if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	} else {
		return uuid.Nil, errors.New("Unknown Error in JWT Validation")
	}
}

func GetBearerToken(headers http.Header) (string, error) {
	bearer := headers.Get("Authorization")
	if bearer == "" {
		return "", errors.New("Bearer Token does not exist.")
	} else if !strings.Contains(bearer, "Bearer ") {
		return "", errors.New("Incorrectly formatted Bearer token.")
	}

	token := strings.TrimPrefix(bearer, "Bearer ")
	return token, nil
}

func MakeRefreshToken() (string, error) {
	const randBytes int = 32
	rawToken := make([]byte, randBytes)
	rand.Read(rawToken)
	encodedToken := hex.EncodeToString(rawToken)
	return encodedToken, nil
}
