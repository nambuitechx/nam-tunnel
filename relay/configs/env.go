package relay_configs

import "os"

type EnvConfig struct {
	Host         string
	Port         string
	DatabasePath string
}

func NewEnvConfig() *EnvConfig {
	host, ok := os.LookupEnv("HOST")
	if !ok {
		host = "localhost"
	}

	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8001"
	}

	databasePath, ok := os.LookupEnv("DATABASE_PATH")
	if !ok {
		databasePath = "relay.db"
	}

	return &EnvConfig{
		Host:         host,
		Port:         port,
		DatabasePath: databasePath,
	}
}
