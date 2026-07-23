package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenOn    string
	Port        string
	DBPath      string
	LibraryPath string
	TMDBAPIKey  string
	JWTSecret   string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on process environment")
	}

	cfg := Config{
		ListenOn:    os.Getenv("SEAHORSE_LISTEN_ON"),
		Port:        os.Getenv("SEAHORSE_PORT"),
		DBPath:      os.Getenv("SEAHORSE_DB"),
		LibraryPath: os.Getenv("SEAHORSE_LIBRARY_PATH"),
		TMDBAPIKey:  os.Getenv("SEAHORSE_TMDB_API_KEY"),
		JWTSecret:   os.Getenv("SEAHORSE_JWT_SECRET"),
	}

	if cfg.Port == "" {
		cfg.Port = "8585"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "sqlite.db"
	}
	if cfg.JWTSecret == "" {
		log.Fatal("SEAHORSE_JWT_SECRET must be set")
	}
	if cfg.TMDBAPIKey == "" {
		log.Println("warning: SEAHORSE_TMDB_API_KEY is not set, library scanning will fail until it is")
	}
	if cfg.LibraryPath == "" {
		log.Fatal("SEAHORSE_LIBRARY_PATH must be set")
	}

	return cfg
}
