package migrations

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Migrator struct {
	migrate *migrate.Migrate
	logger  *slog.Logger
}

func NewMigrator(dbConnString string, logger *slog.Logger) (*Migrator, error) {
	// Get the current working directory to find migrations
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	migrationsPath := filepath.Join(wd, "migrations")
	migrationsURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(migrationsURL, dbConnString)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{
		migrate: m,
		logger:  logger,
	}, nil
}

func (m *Migrator) Up() error {
	m.logger.Info("running database migrations")

	if err := m.migrate.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	m.logger.Info("database migrations completed successfully")
	return nil
}

func (m *Migrator) Down() error {
	m.logger.Info("rolling back database migrations")

	if err := m.migrate.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	m.logger.Info("database migrations rolled back successfully")
	return nil
}

func (m *Migrator) Steps(n int) error {
	m.logger.Info("running database migrations", "steps", n)

	if err := m.migrate.Steps(n); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	m.logger.Info("database migrations completed successfully")
	return nil
}

func (m *Migrator) Version() (uint, bool, error) {
	return m.migrate.Version()
}

func (m *Migrator) Close() error {
	srcErr, dbErr := m.migrate.Close()
	if srcErr != nil {
		return srcErr
	}
	if dbErr != nil {
		return dbErr
	}
	return nil
}
