package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/progress"
	"github.com/TeluTrix/seahorse/internal/scanner"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// RefreshMovie re-fetches this movie's metadata from TMDB by its already-known
// TMDB ID (never a fresh title search, so a refresh can't drift to a
// different match than the one already on file) and overwrites every stored
// field the detail page shows, including re-downloading the cover image so a
// changed poster on TMDB actually takes effect.
func (h *Handlers) RefreshMovie(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid movie id")
		return
	}

	var movie models.Movie
	if err := db.DB.First(&movie, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "movie not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load movie")
		return
	}

	details, err := h.Scanner.TMDBClient().GetMovieDetails(movie.TMDBID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch tmdb movie details")
		return
	}

	movie.Title = details.Title
	movie.Overview = details.Overview
	movie.PosterPath = details.PosterPath
	movie.BackdropPath = details.BackdropPath
	movie.ReleaseDate = details.ReleaseDate
	movie.VoteAverage = details.VoteAverage
	movie.Genres = tmdb.JoinGenres(details.Genres)
	movie.Runtime = details.Runtime
	movie.Director = details.Director
	if encoded, err := json.Marshal(details.Cast); err == nil {
		movie.Cast = string(encoded)
	}
	movie.Tagline = details.Tagline
	movie.OriginalLanguage = details.OriginalLanguage
	movie.Budget = details.Budget
	movie.Revenue = details.Revenue
	movie.ProductionCompanies = strings.Join(details.ProductionCompanies, ", ")
	movie.ProductionCountries = strings.Join(details.ProductionCountries, ", ")

	dir := filepath.Dir(movie.FilePath)
	scanner.RemoveCoverFiles(dir)
	movie.CoverCached = scanner.DownloadCover(dir, details.PosterPath)

	if err := db.DB.Save(&movie).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not save refreshed movie")
		return
	}

	var wp *models.WatchProgress
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		wp, err = progress.Get(userID, models.MediaTypeMovie, movie.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load progress")
			return
		}
	}

	writeJSON(w, http.StatusOK, toMovieDTO(movie, wp, true, h.Scanner.RemuxState))
}

// RefreshTVShow re-fetches this show's metadata from TMDB by its already-known
// TMDB ID, the tv-show equivalent of RefreshMovie. Episode-level metadata
// (titles/overviews/stills per season) is left untouched — this only
// refreshes the show itself (title, overview, cover, cast, etc.).
func (h *Handlers) RefreshTVShow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tv show id")
		return
	}

	var show models.TVShow
	if err := db.DB.First(&show, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "tv show not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load tv show")
		return
	}

	details, err := h.Scanner.TMDBClient().GetTVDetails(show.TMDBID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not fetch tmdb tv show details")
		return
	}

	show.Title = details.Title
	show.Overview = details.Overview
	show.PosterPath = details.PosterPath
	show.BackdropPath = details.BackdropPath
	show.FirstAirDate = details.FirstAirDate
	show.VoteAverage = details.VoteAverage
	show.Genres = tmdb.JoinGenres(details.Genres)
	show.Creators = strings.Join(details.Creators, ", ")
	if encoded, err := json.Marshal(details.Cast); err == nil {
		show.Cast = string(encoded)
	}

	scanner.RemoveCoverFiles(show.FolderPath)
	show.CoverCached = scanner.DownloadCover(show.FolderPath, details.PosterPath)

	if err := db.DB.Save(&show).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not save refreshed tv show")
		return
	}

	dto, err := h.loadTVShowDetail(r, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not reload tv show")
		return
	}

	writeJSON(w, http.StatusOK, dto)
}
