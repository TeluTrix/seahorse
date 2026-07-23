package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenOn    string
	Port        string
	DBPath      string
	LibraryPath string
	TMDBAPIKey  string
	JWTSecret   string

	RemuxConcurrency    int
	DisableRegistration bool

	JWTTTL            time.Duration
	TMDBTimeout       time.Duration
	AudioProbeTimeout time.Duration
	AudioRemuxTimeout time.Duration
	AudioBitrate      string
	CastLimit         int
	MaxPageSize       int

	// Values below are also served to the frontend via GET /api/config,
	// since the frontend is a prebuilt static bundle and can't read these
	// env vars directly.
	DefaultPageSize               int
	PlayerSeekSeconds             int
	ResumeThresholdSeconds        int
	ProgressReportIntervalSeconds int
}

// envInt reads key as a positive integer, falling back to def (with a
// warning) if it's unset, invalid, or not positive.
func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("warning: invalid %s %q, defaulting to %d", key, raw, def)
		return def
	}
	return n
}

func envString(key, def string) string {
	if raw := os.Getenv(key); raw != "" {
		return raw
	}
	return def
}

// envBool reads key as a boolean ("true"/"1"/"yes"/"on", case-insensitive,
// for true), falling back to def (with a warning) if it's unset or
// unrecognized.
func envBool(key string, def bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	switch strings.ToLower(raw) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		log.Printf("warning: invalid %s %q, defaulting to %v", key, raw, def)
		return def
	}
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

	cfg.RemuxConcurrency = envInt("SEAHORSE_REMUX_CONCURRENCY", 1)
	cfg.DisableRegistration = envBool("SEAHORSE_DISABLE_REGISTRATION", false)

	cfg.JWTTTL = time.Duration(envInt("SEAHORSE_JWT_TTL_HOURS", 24)) * time.Hour
	cfg.TMDBTimeout = time.Duration(envInt("SEAHORSE_TMDB_TIMEOUT_SECONDS", 10)) * time.Second
	cfg.AudioProbeTimeout = time.Duration(envInt("SEAHORSE_AUDIO_PROBE_TIMEOUT_SECONDS", 30)) * time.Second
	cfg.AudioRemuxTimeout = time.Duration(envInt("SEAHORSE_AUDIO_REMUX_TIMEOUT_MINUTES", 60)) * time.Minute
	cfg.AudioBitrate = envString("SEAHORSE_AUDIO_BITRATE", "192k")
	cfg.CastLimit = envInt("SEAHORSE_CAST_LIMIT", 15)
	cfg.MaxPageSize = envInt("SEAHORSE_MAX_PAGE_SIZE", 200)

	cfg.DefaultPageSize = envInt("SEAHORSE_DEFAULT_PAGE_SIZE", 48)
	cfg.PlayerSeekSeconds = envInt("SEAHORSE_PLAYER_SEEK_SECONDS", 15)
	cfg.ResumeThresholdSeconds = envInt("SEAHORSE_RESUME_THRESHOLD_SECONDS", 5)
	cfg.ProgressReportIntervalSeconds = envInt("SEAHORSE_PROGRESS_REPORT_INTERVAL_SECONDS", 10)

	return cfg
}
