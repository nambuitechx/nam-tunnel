package relay_configs

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func (c *EnvConfig) DatabaseMigrateURL() string {
	return "sqlite://" + c.DatabasePath
}

func (c *EnvConfig) ConnectDatabase(ctx context.Context) (*sql.DB, error) {
	if dir := filepath.Dir(c.DatabasePath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", sqliteDSN(c.DatabasePath))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func sqliteDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
}
