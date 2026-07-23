package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/TeluTrix/seahorse/internal/api"
	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/config"
	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/scanner"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/TeluTrix/seahorse/internal/transcode"
	"github.com/TeluTrix/seahorse/internal/web"
	"github.com/gorilla/mux"
)

func main() {
	cfg := config.Load()

	if err := db.OpenConnection(cfg.DBPath); err != nil {
		log.Fatalf("could not open database: %v", err)
	}

	authenticator := auth.New(cfg.JWTSecret, cfg.JWTTTL)
	tmdbClient := tmdb.New(cfg.TMDBAPIKey, cfg.TMDBTimeout, cfg.CastLimit)
	transcodeOpts := transcode.Options{
		ProbeTimeout: cfg.AudioProbeTimeout,
		RemuxTimeout: cfg.AudioRemuxTimeout,
		AudioBitrate: cfg.AudioBitrate,
	}
	libraryScanner := scanner.New(tmdbClient, cfg.RemuxConcurrency, transcodeOpts)
	clientConfig := api.ClientConfig{
		DefaultPageSize:               cfg.DefaultPageSize,
		PlayerSeekSeconds:             cfg.PlayerSeekSeconds,
		ResumeThresholdSeconds:        cfg.ResumeThresholdSeconds,
		ProgressReportIntervalSeconds: cfg.ProgressReportIntervalSeconds,
	}
	handlers := api.NewHandlers(authenticator, libraryScanner, cfg.LibraryPath, cfg.MaxPageSize, cfg.DisableRegistration, clientConfig)

	r := mux.NewRouter()

	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/auth/register", handlers.Register).Methods("POST")
	apiRouter.HandleFunc("/auth/login", handlers.Login).Methods("POST")
	apiRouter.HandleFunc("/config", handlers.GetConfig).Methods("GET")

	apiRouter.Handle("/user/me", authenticator.RequireAuth(http.HandlerFunc(handlers.Me))).Methods("GET")

	apiRouter.Handle("/movies", authenticator.RequireAuth(http.HandlerFunc(handlers.ListMovies))).Methods("GET")
	apiRouter.Handle("/movies/{id}", authenticator.RequireAuth(http.HandlerFunc(handlers.GetMovie))).Methods("GET")
	apiRouter.Handle("/tvshows", authenticator.RequireAuth(http.HandlerFunc(handlers.ListTVShows))).Methods("GET")
	apiRouter.Handle("/tvshows/{id}", authenticator.RequireAuth(http.HandlerFunc(handlers.GetTVShow))).Methods("GET")

	apiRouter.Handle("/search", authenticator.RequireAuth(http.HandlerFunc(handlers.Search))).Methods("GET")
	apiRouter.Handle("/genres", authenticator.RequireAuth(http.HandlerFunc(handlers.ListGenres))).Methods("GET")

	apiRouter.Handle("/stream/movies/{id}", authenticator.RequireAuth(http.HandlerFunc(handlers.StreamMovie))).Methods("GET")
	apiRouter.Handle("/stream/episodes/{id}", authenticator.RequireAuth(http.HandlerFunc(handlers.StreamEpisode))).Methods("GET")

	apiRouter.Handle("/images/movies/{id}/cover", authenticator.RequireAuth(http.HandlerFunc(handlers.MovieCover))).Methods("GET")
	apiRouter.Handle("/images/tvshows/{id}/cover", authenticator.RequireAuth(http.HandlerFunc(handlers.TVShowCover))).Methods("GET")

	apiRouter.Handle("/subtitles/movies/{id}", authenticator.RequireAuth(http.HandlerFunc(handlers.MovieSubtitles))).Methods("GET")
	apiRouter.Handle("/subtitles/movies/{id}/vtt", authenticator.RequireAuth(http.HandlerFunc(handlers.MovieSubtitleVTT))).Methods("GET")
	apiRouter.Handle("/subtitles/episodes/{id}", authenticator.RequireAuth(http.HandlerFunc(handlers.EpisodeSubtitles))).Methods("GET")
	apiRouter.Handle("/subtitles/episodes/{id}/vtt", authenticator.RequireAuth(http.HandlerFunc(handlers.EpisodeSubtitleVTT))).Methods("GET")

	apiRouter.Handle("/progress", authenticator.RequireAuth(http.HandlerFunc(handlers.SaveProgress))).Methods("PUT")
	apiRouter.Handle("/progress/{mediaType}/{mediaId}", authenticator.RequireAuth(http.HandlerFunc(handlers.GetProgress))).Methods("GET")

	apiRouter.Handle("/admin/scan", authenticator.RequireAdmin(http.HandlerFunc(handlers.ScanLibrary))).Methods("POST")
	apiRouter.Handle("/admin/scan/status", authenticator.RequireAdmin(http.HandlerFunc(handlers.ScanStatus))).Methods("GET")
	apiRouter.Handle("/admin/scan/events", authenticator.RequireAdmin(http.HandlerFunc(handlers.ScanEvents))).Methods("GET")
	apiRouter.Handle("/admin/users", authenticator.RequireAdmin(http.HandlerFunc(handlers.ListUsers))).Methods("GET")
	apiRouter.Handle("/admin/users", authenticator.RequireAdmin(http.HandlerFunc(handlers.CreateUser))).Methods("POST")
	apiRouter.Handle("/admin/users/{id}/password", authenticator.RequireAdmin(http.HandlerFunc(handlers.SetUserPassword))).Methods("PUT")

	r.PathPrefix("/").Handler(web.Handler())

	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf("%s:%s", cfg.ListenOn, cfg.Port),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 0, // streaming responses can run long; rely on client/proxy timeouts instead
		ReadTimeout:  15 * time.Second,
	}

	slog.Info("seahorse listening", "addr", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
