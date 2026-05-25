package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/nambuitechx/nam-tunnel/relay/configs"
	relay_handlers "github.com/nambuitechx/nam-tunnel/relay/handlers"
	relay_migrations "github.com/nambuitechx/nam-tunnel/relay/migrations"
	relay_models "github.com/nambuitechx/nam-tunnel/relay/models"
)

func main() {
	ctx := context.Background()
	envConfig := relay_configs.NewEnvConfig()

	if err := relay_migrations.Up(envConfig.DatabaseDSN()); err != nil {
		log.Fatal("migrate:", err)
	}

	db, err := envConfig.ConnectDatabase(ctx)
	if err != nil {
		log.Fatal("connect database:", err)
	}
	defer db.Close()

	userRepo := relay_models.NewUserRepository(db)

	tunnelHandler := relay_handlers.NewTunnelHandler()
	userHandler := relay_handlers.NewUserHandler(userRepo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /connect", tunnelHandler.Connect)
	// {path...} captures everything after the tunnel id (api/v1/users, etc.)
	mux.HandleFunc("GET /{id}/{path...}", tunnelHandler.Proxy)
	mux.HandleFunc("GET /{id}", tunnelHandler.Proxy)

	mux.HandleFunc("POST /api/v1/users", userHandler.Create)
	mux.HandleFunc("GET /api/v1/users", userHandler.List)
	mux.HandleFunc("GET /api/v1/users/{id}", userHandler.Get)
	mux.HandleFunc("PUT /api/v1/users/{id}", userHandler.Update)
	mux.HandleFunc("DELETE /api/v1/users/{id}", userHandler.Delete)
	mux.HandleFunc("POST /api/v1/auth/login", userHandler.Login)

	srv := &http.Server{Addr: ":" + envConfig.Port, Handler: mux}

	go func() {
		log.Println("relay server is listening on :" + envConfig.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()
	log.Println("relay server is shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
	log.Println("relay server is gracefully shutted down")
}
