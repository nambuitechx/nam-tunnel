package relay_configs

import "os"

type EnvConfig struct {
	Host string
	Port string
	DatabaseHost string
	DatabasePort string
	DatabaseUser string
	DatabasePassword string
	DatabaseName string
	DatabaseSSLMode string
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

	databaseHost, ok := os.LookupEnv("DATABASE_HOST")
	if !ok {
		databaseHost = "localhost"
	}

	databasePort, ok := os.LookupEnv("DATABASE_PORT")
	if !ok {
		databasePort = "5432"
	}

	databaseUser, ok := os.LookupEnv("DATABASE_USER")
	if !ok {
		databaseUser = "postgres"
	}

	databasePassword, ok := os.LookupEnv("DATABASE_PASSWORD")
	if !ok {
		databasePassword = "password"
	}

	databaseName, ok := os.LookupEnv("DATABASE_NAME")
	if !ok {
		databaseName = "postgres"
	}

	databaseSSLMode, ok := os.LookupEnv("DATABASE_SSL_MODE")
	if !ok {
		databaseSSLMode = "disable"
	}

	return &EnvConfig{
		Host: host,
		Port: port,
		DatabaseHost: databaseHost,
		DatabasePort: databasePort,
		DatabaseUser: databaseUser,
		DatabasePassword: databasePassword,
		DatabaseName: databaseName,
		DatabaseSSLMode: databaseSSLMode,
	}
}
