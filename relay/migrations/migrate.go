package relay_migrations

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

func Up(databaseURL string) error {
	source, err := iofs.New(files, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, pgxDSN(databaseURL))
	if err != nil {
		return fmt.Errorf("migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func Down(databaseURL string) error {
	source, err := iofs.New(files, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, pgxDSN(databaseURL))
	if err != nil {
		return fmt.Errorf("migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func pgxDSN(databaseURL string) string {
	return strings.Replace(databaseURL, "postgres://", "pgx5://", 1)
}
