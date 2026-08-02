package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/gorilla/mux"
)

type ActorDTO struct {
	Name       string `json:"name"`
	ProfileURL string `json:"profile_url,omitempty"`
	Credits    int    `json:"credits"`
}

type ActorsPageDTO struct {
	Actors   []ActorDTO `json:"actors"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
}

// aggregateActors groups every cast credit across all movies and tv shows by
// name. There's no stable actor ID stored anywhere — cast is saved as
// unstructured JSON per title (see models.Movie.Cast/models.TVShow.Cast) —
// so this is a best-effort grouping by name string; two different real
// people who happen to share a name will collide. An acceptable tradeoff for
// a personal library, same spirit as how genres are already deduped by
// string rather than a normalized table.
func aggregateActors() (map[string]*ActorDTO, error) {
	// "cast" is quoted throughout because it collides with SQLite's CAST(...)
	// keyword when used bare in a WHERE clause (unlike in Pluck's column list,
	// which GORM already quotes automatically).
	var movieCasts []string
	if err := db.DB.Model(&models.Movie{}).Where(`"cast" <> ''`).Pluck("cast", &movieCasts).Error; err != nil {
		return nil, err
	}
	var showCasts []string
	if err := db.DB.Model(&models.TVShow{}).Where(`"cast" <> ''`).Pluck("cast", &showCasts).Error; err != nil {
		return nil, err
	}

	actors := map[string]*ActorDTO{}
	for _, raw := range append(movieCasts, showCasts...) {
		var members []tmdb.CastMember
		if err := json.Unmarshal([]byte(raw), &members); err != nil {
			continue
		}
		for _, m := range members {
			a, ok := actors[m.Name]
			if !ok {
				a = &ActorDTO{Name: m.Name}
				actors[m.Name] = a
			}
			if a.ProfileURL == "" && m.ProfilePath != "" {
				a.ProfileURL = tmdb.ImageURL(m.ProfilePath, "w185")
			}
			a.Credits++
		}
	}
	return actors, nil
}

// ListActors returns every actor appearing in the library, optionally
// filtered by a case-insensitive substring match on name, paginated and
// sorted alphabetically.
func (h *Handlers) ListActors(w http.ResponseWriter, r *http.Request) {
	actors, err := aggregateActors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load actors")
		return
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	list := make([]ActorDTO, 0, len(actors))
	for _, a := range actors {
		if q == "" || strings.Contains(strings.ToLower(a.Name), q) {
			list = append(list, *a)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	page, pageSize := h.parsePagination(r)
	total := len(list)
	start := min(page-1, total) * pageSize
	if start > total {
		start = total
	}
	end := min(start+pageSize, total)

	writeJSON(w, http.StatusOK, ActorsPageDTO{Actors: list[start:end], Page: page, PageSize: pageSize, Total: total})
}

type ActorFilmographyDTO struct {
	Name    string      `json:"name"`
	Movies  []MovieDTO  `json:"movies"`
	TVShows []TVShowDTO `json:"tv_shows"`
}

// ActorFilmography returns every movie and tv show the named actor (matched
// case-insensitively) appears in.
func (h *Handlers) ActorFilmography(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	needle := strings.ToLower(name)

	var movies []models.Movie
	if err := db.DB.Where(`"cast" <> ''`).Find(&movies).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load movies")
		return
	}
	var shows []models.TVShow
	if err := db.DB.Where(`"cast" <> ''`).Find(&shows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load tv shows")
		return
	}

	movieDTOs := make([]MovieDTO, 0)
	for _, m := range movies {
		if castIncludes(m.Cast, needle) {
			movieDTOs = append(movieDTOs, toMovieDTO(m, nil, false, noRemuxStatus))
		}
	}
	showDTOs := make([]TVShowDTO, 0)
	for _, s := range shows {
		if castIncludes(s.Cast, needle) {
			showDTOs = append(showDTOs, toTVShowDTO(s, nil, false, noRemuxStatus))
		}
	}

	writeJSON(w, http.StatusOK, ActorFilmographyDTO{Name: name, Movies: movieDTOs, TVShows: showDTOs})
}

func castIncludes(raw, lowerName string) bool {
	var members []tmdb.CastMember
	if err := json.Unmarshal([]byte(raw), &members); err != nil {
		return false
	}
	for _, m := range members {
		if strings.ToLower(m.Name) == lowerName {
			return true
		}
	}
	return false
}
