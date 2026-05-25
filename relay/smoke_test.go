package main

import (
	"context"
	"testing"

	relay_configs "github.com/nambuitechx/nam-tunnel/relay/configs"
	relay_migrations "github.com/nambuitechx/nam-tunnel/relay/migrations"
	relay_models "github.com/nambuitechx/nam-tunnel/relay/models"
)

func TestSQLiteUserCRUD(t *testing.T) {
	path := t.TempDir() + "/relay.db"
	t.Setenv("DATABASE_PATH", path)

	cfg := relay_configs.NewEnvConfig()
	if err := relay_migrations.Up(cfg.DatabaseMigrateURL()); err != nil {
		t.Fatal(err)
	}

	db, err := cfg.ConnectDatabase(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := relay_models.NewUserRepository(db)
	user, err := repo.Create(context.Background(), "alice", "secret")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("username: got %q", user.Username)
	}

	got, err := repo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID {
		t.Fatalf("id mismatch")
	}

	auth, err := repo.Authenticate(context.Background(), "alice", "secret")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if auth.ID != user.ID {
		t.Fatal("auth id mismatch")
	}

}
