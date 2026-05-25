package relay_migrations

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

func Up(databaseURL string) error {
	if err := ensureDatabaseDir(databaseURL); err != nil {
		return err
	}

	m, err := newMigrate(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func Down(databaseURL string) error {
	m, err := newMigrate(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func newMigrate(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(files, ".")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("migration instance: %w", err)
	}
	return m, nil
}

func ensureDatabaseDir(databaseURL string) error {
	const prefix = "sqlite://"
	if len(databaseURL) <= len(prefix) || databaseURL[:len(prefix)] != prefix {
		return nil
	}

	path := databaseURL[len(prefix):]
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	return nil
}
