package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Initialize database
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "user"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "password"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "fulcrum"
	}
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create tables if not exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS events (
		event_id TEXT PRIMARY KEY,
		tenant_id TEXT,
		user_id TEXT,
		session_id TEXT,
		event_type TEXT,
		properties JSONB,
		occurred_at TIMESTAMP
	)`)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize metrics
	metrics := NewMetrics(prometheus.DefaultRegisterer)
	if err := metrics.Register(prometheus.DefaultRegisterer); err != nil {
		log.Fatal(err)
	}

	// Initialize store
	store := NewStore(db, metrics)

	// Initialize ingest handler
	ingest := NewIngest(store, metrics)

	// Setup mux
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/events", ingest.HandleSingle)
	mux.HandleFunc("/v1/events/batch", ingest.HandleBatch)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// Start server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	// Drain in-flight and flush
	store.Shutdown(ctx)

	log.Println("Server exited")
}
