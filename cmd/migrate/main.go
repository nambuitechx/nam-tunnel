package main

import (
	"flag"
	"log"
	"os"

	"github.com/nambuitechx/nam-tunnel/relay/configs"
	relay_migrations "github.com/nambuitechx/nam-tunnel/relay/migrations"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	flag.Parse()

	envConfig := relay_configs.NewEnvConfig()
	dsn := envConfig.DatabaseMigrateURL()

	var err error
	switch *direction {
	case "up":
		err = relay_migrations.Up(dsn)
	case "down":
		err = relay_migrations.Down(dsn)
	default:
		log.Fatalf("unknown direction %q (use up or down)", *direction)
	}

	if err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
