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

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/openmeet-team/survey/internal/api"
	"github.com/openmeet-team/survey/internal/db"
	"github.com/openmeet-team/survey/internal/telemetry"
)

func main() {
	// Register Prometheus metrics
	telemetry.RegisterMetrics()

	// Get database configuration from environment
	dbConfig, err := db.ConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load database config: %v", err)
	}

	// Connect to database
	ctx := context.Background()
	database, err := db.Connect(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close(database)

	log.Println("Connected to database successfully")

	// Create database queries instance
	queries := db.NewQueries(database)

	// Create Echo instance
	e := echo.New()

	// Basic middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Create handlers
	handlers := api.NewHandlers(queries)
	healthHandlers := api.NewHealthHandlers(database)

	// Setup routes (includes metrics and request ID middleware)
	api.SetupRoutes(e, handlers, healthHandlers)

	// Start server with graceful shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server in a goroutine
	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("Starting server on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}

	log.Println("Server shutdown complete")
}
