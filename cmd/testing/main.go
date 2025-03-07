package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const validToken = "12345abcdef" // Define your valid test token here

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	// Check Bearer token format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	// Extract the token
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate the token
	if token != validToken {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// If valid, return success
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"message": "Token is valid"}`)
}

func main() {
	http.HandleFunc("/validate", tokenHandler)

	port := ":8090"
	log.Println("Server running on", port)

	server := &http.Server{
		Addr:         port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
