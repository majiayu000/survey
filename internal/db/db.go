package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
)

// Config holds database connection configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// ConfigFromEnv creates a Config from environment variables with sensible defaults
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Host:     getEnvOrDefault("DATABASE_HOST", "localhost"),
		User:     getEnvOrDefault("DATABASE_USER", "postgres"),
		Password: os.Getenv("DATABASE_PASSWORD"),
		Database: getEnvOrDefault("DATABASE_NAME", "survey"),
		SSLMode:  getEnvOrDefault("DATABASE_SSLMODE", "disable"),
	}

	// Parse port with default
	portStr := getEnvOrDefault("DATABASE_PORT", "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid DATABASE_PORT: %w", err)
	}
	cfg.Port = port

	// Validate the config
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate checks that required configuration fields are set
func (c Config) Validate() error {
	if c.Password == "" {
		return fmt.Errorf("DATABASE_PASSWORD is required")
	}
	if c.Port <= 0 {
		return fmt.Errorf("port must be positive, got %d", c.Port)
	}
	return nil
}

// ConnectionString returns a PostgreSQL connection string
func (c Config) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// Connect establishes a database connection with OpenTelemetry instrumentation
func Connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	// Validate config before attempting connection
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Register the pgx driver with OpenTelemetry instrumentation
	driverName, err := otelsql.Register(
		"pgx",
		otelsql.WithAttributes(
			semconv.DBSystemPostgreSQL,
			attribute.String("db.name", cfg.Database),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register otelsql driver: %w", err)
	}

	// Open database connection
	db, err := sql.Open(driverName, cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	// SetConnMaxIdleTime sets the maximum amount of time a connection may be idle
	// db.SetConnMaxIdleTime(5 * time.Minute)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused
	// db.SetConnMaxLifetime(30 * time.Minute)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
