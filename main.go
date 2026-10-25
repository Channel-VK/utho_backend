package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"utho_backend/internal/api"
	"utho_backend/internal/logger"
)

// corsMiddleware wraps an http.Handler with permissive CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("=================================================================")
	fmt.Println("🚀 SMARTCLOUD OPTIMIZER - UTHO CLOUD FINOPS BACKEND API")
	fmt.Println("=================================================================")
	fmt.Println("⚙️  Initializing In-Memory Store & Deterministic Seed (Seed 42)...")

	store := api.NewStore()

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"smartcloud-optimizer-backend"}`))
	})

	// Register all FinOps API routes
	store.RegisterRoutes(mux)

	handler := corsMiddleware(logger.HTTPLogger(mux))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run context
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("✨ SmartCloud Optimizer Backend listening on http://localhost:%s\n", port)
		log.Printf("📡 Full Permissive CORS Enabled for React Frontend (:5173)\n")
		log.Printf("🤖 Python AI Bridge configured for %s\n", os.Getenv("PYTHON_AI_URL"))
		if os.Getenv("PYTHON_AI_URL") == "" {
			log.Println("ℹ️  PYTHON_AI_URL not set; defaulting to http://localhost:8001 with automatic rule-engine fallback.")
		}
		serverErrors <- server.ListenAndServe()
	}()

	// Graceful shutdown handling
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received signal %v, initiating graceful shutdown...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			_ = server.Close()
		}
		log.Println("Server gracefully stopped.")
	}
}
