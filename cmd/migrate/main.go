package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/zhirschtritt/eventing/internal/migrations"
)

type Config struct {
	DBConnString string `env:"DB_CONN_STRING" envDefault:"postgres://postgres:postgres@localhost:5444/eventing_wal?sslmode=disable"`
}

func main() {
	var (
		up    = flag.Bool("up", false, "Run all pending migrations")
		down  = flag.Bool("down", false, "Rollback all migrations")
		steps = flag.Int("steps", 0, "Run specific number of migrations (positive for up, negative for down)")
	)
	flag.Parse()

	// Parse configuration from environment variables
	config := &Config{}
	if err := env.Parse(config); err != nil {
		slog.Error("failed to parse configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	migrator, err := migrations.NewMigrator(config.DBConnString, logger)
	if err != nil {
		logger.Error("failed to create migrator", "error", err)
		os.Exit(1)
	}
	defer migrator.Close()

	switch {
	case *up:
		if err := migrator.Up(); err != nil {
			logger.Error("failed to run migrations up", "error", err)
			os.Exit(1)
		}
		fmt.Println("Migrations completed successfully")
	case *down:
		if err := migrator.Down(); err != nil {
			logger.Error("failed to run migrations down", "error", err)
			os.Exit(1)
		}
		fmt.Println("Migrations rolled back successfully")
	case *steps != 0:
		if err := migrator.Steps(*steps); err != nil {
			logger.Error("failed to run migrations steps", "error", err, "steps", *steps)
			os.Exit(1)
		}
		fmt.Printf("Migrations completed successfully (%d steps)\n", *steps)
	default:
		version, dirty, err := migrator.Version()
		if err != nil {
			logger.Error("failed to get migration version", "error", err)
			os.Exit(1)
		}
		fmt.Printf("Current migration version: %d, dirty: %t\n", version, dirty)
	}
}
