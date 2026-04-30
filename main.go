package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fulcrum-test/config"
	"fulcrum-test/handlers"
	"fulcrum-test/middlewares"
	"fulcrum-test/stores"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	// Initialize stores
	stores, err := stores.NewStores(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		for _, s := range stores {
			s.Shutdown()
		}
	}()

	// Initialize metrics
	log.Println("Registering metrics...")
	metrics := handlers.NewMetrics(nil)
	log.Println("Metrics initialization complete")

	// Initialize handlers
	ingest := handlers.NewIngestHandler(stores, metrics)

	// Setup gin router
	r := gin.New()

	// Routes without auth
	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes with middleware
	api := r.Group("")
	api.Use(middlewares.APIKeyAuth(cfg.Middlewares.APIKey))
	api.Use(middlewares.RateLimit(cfg.Middlewares.RateLimit))
	api.Use(middlewares.NewCircuitBreaker(cfg.Middlewares.CircuitBreaker.FailureThreshold, cfg.Middlewares.CircuitBreaker.Timeout).Handler())

	// Routes
	api.POST("/v1/events", gin.WrapF(ingest.HandleSingle))
	api.POST("/v1/events/batch", gin.WrapF(ingest.HandleBatch))

	// Start server
	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Starting server on :%s", cfg.Server.Port)
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

	log.Println("Server exited")
}
