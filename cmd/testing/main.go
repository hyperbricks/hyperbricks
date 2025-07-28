package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Query Parameter Echo Handler
func echoQueryHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	queryParams := r.URL.Query()

	// Convert query parameters to a map
	paramsMap := make(map[string]interface{})
	for key, values := range queryParams {
		if len(values) == 1 {
			paramsMap[key] = values[0] // Store as string if single value
		} else {
			paramsMap[key] = values // Store as array if multiple values
		}
	}

	// Prepare the response
	response := map[string]interface{}{
		"queryParams": paramsMap,
		"valid":       len(paramsMap) > 0,
		"message":     "",
	}

	if len(paramsMap) == 0 {
		response["message"] = "No query parameters provided"
	}

	// Convert response to JSON
	jsonResponse, _ := json.Marshal(response)

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

const jwtSecret = "a-string-secret-at-least-256-bits-long" // Hardcoded secret for validation

func echoTokenClaimAndValidationHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	// Check Bearer token format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"message": "Invalid token format"}`, http.StatusUnauthorized)
		return
	}

	// Extract the token
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Parse and validate the JWT
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Ensure signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	// Prepare the response
	response := map[string]interface{}{
		"token":  tokenString,
		"valid":  err == nil && token.Valid,
		"claims": claims,
		"error":  "",
	}

	if err != nil {
		response["error"] = err.Error()
	}

	// Convert response to JSON
	jsonResponse, _ := json.Marshal(response)

	// Set response header
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}
func echoTokenHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	// Check Bearer token format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"message": "Invalid token format"}`, http.StatusUnauthorized)
		return
	}

	// Extract the token
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Set response header
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Return JSON response
	response := fmt.Sprintf(`{"token": "%s"}`, token)
	w.Write([]byte(response))
}

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

func bodyHandler(w http.ResponseWriter, r *http.Request) {

	mergedData := make(map[string]interface{})

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		// If valid, return success
		w.WriteHeader(http.StatusNoContent)
		fmt.Fprintln(w, `{"message": "No body provided"}`)
		return
	}

	// Parse JSON body into a map
	var bodyData map[string]interface{}
	err = json.Unmarshal(bodyBytes, &bodyData)
	if err != nil {
		bodyData = make(map[string]interface{})
		fmt.Errorf("invalid or no JSON payload in body: %w", err)
	} else {

		// Merge body data with conflicts resolved
		for key, value := range bodyData {
			if _, exists := mergedData[key]; exists {
				mergedData["body_"+key] = value
			} else {
				mergedData[key] = value
			}
		}
	}
	if mergedData["password"] == "mysupersecretpassword" {
		// If valid, return success
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"message": "Token is valid"}`)
	} else {
		w.WriteHeader(400)
		fmt.Fprintln(w, `{"message": "Token is not valid"}`)
	}

}

func echoDataHandler(w http.ResponseWriter, r *http.Request) {
	// Initialize mergedData map
	mergedData := make(map[string]interface{})

	// Read request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil || len(bodyBytes) == 0 { // Check if body is empty
		http.Error(w, `{"message": "No body provided"}`, http.StatusNoContent)
		return
	}

	// Parse JSON body into a map
	var bodyData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &bodyData); err != nil {
		http.Error(w, `{"message": "Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	// Merge body data into mergedData with conflict resolution
	for key, value := range bodyData {
		if _, exists := mergedData[key]; exists {
			mergedData["body_"+key] = value
		} else {
			mergedData[key] = value
		}
	}

	// Convert mergedData to JSON
	jsonBytes, err := json.Marshal(mergedData)
	if err != nil {
		http.Error(w, `{"message": "Failed to encode JSON"}`, http.StatusInternalServerError)
		return
	}

	// Set content type and return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}

func main() {
	http.HandleFunc("/validate", tokenHandler)
	http.HandleFunc("/validate/body", bodyHandler)
	http.HandleFunc("/echo/query", echoQueryHandler)
	http.HandleFunc("/echo/data", echoDataHandler)
	http.HandleFunc("/echo/token", echoTokenHandler)
	http.HandleFunc("/echo/token/validate", echoTokenClaimAndValidationHandler)
	port := ":8090"
	log.Println("Server running on", port)

	server := &http.Server{
		Addr:         port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
