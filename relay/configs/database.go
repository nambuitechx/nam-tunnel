package relay_configs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func (c *EnvConfig) DatabaseDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DatabaseUser,
		c.DatabasePassword,
		c.DatabaseHost,
		c.DatabasePort,
		c.DatabaseName,
		c.DatabaseSSLMode,
	)
}

func (c *EnvConfig) ConnectDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, c.DatabaseDSN())
}
