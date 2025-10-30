package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type HTTPClientConfig struct {
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
}

func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:             30 * time.Second,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     0, // unlimited
		IdleConnTimeout:     90 * time.Second,
	}
}

func NewHTTPClient(config HTTPClientConfig) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
	}

	return &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}
}

func NewDefaultHTTPClient() *http.Client {
	return NewHTTPClient(DefaultHTTPClientConfig())
}

func NewHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	config := DefaultHTTPClientConfig()
	config.Timeout = timeout
	return NewHTTPClient(config)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("← %s %s [%v]", r.Method, r.URL.Path, time.Since(start))
	})
}

func JSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func JSONError(w http.ResponseWriter, message string, statusCode int) {
	JSONResponse(w, map[string]string{"error": message}, statusCode)
}

type HealthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, HealthResponse{
		Status: "ok",
		Time:   time.Now().Format(time.RFC3339),
	}, http.StatusOK)
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				JSONError(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}