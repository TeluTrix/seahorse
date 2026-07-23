package tmdb

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.themoviedb.org/3"

// topCastLimit caps how many cast members are kept per movie/show, matching
// the "top billed" cast section on an IMDb-style page rather than a full
// credits dump.
const topCastLimit = 15

type Client struct {
	apiKey      string
	httpClient  *http.Client
	movieGenres map[int]string
	tvGenres    map[int]string
}

func New(apiKey string) *Client {
	c := &Client{
		apiKey:      apiKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		movieGenres: map[int]string{},
		tvGenres:    map[int]string{},
	}

	c.loadGenres("/genre/movie/list", c.movieGenres)
	c.loadGenres("/genre/tv/list", c.tvGenres)

	return c
}

type MovieResult struct {
	TMDBID       int
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	ReleaseDate  string
	VoteAverage  float64
	Genres       []string
}

type TVResult struct {
	TMDBID       int
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	FirstAirDate string
	VoteAverage  float64
	Genres       []string
}

type EpisodeResult struct {
	EpisodeNumber int
	Title         string
	Overview      string
	StillPath     string
}

func ImageURL(path string, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", size, path)
}

func (c *Client) get(path string, params url.Values, out interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.apiKey)

	resp, err := c.httpClient.Get(baseURL + path + "?" + params.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb request to %s failed with status %d", path, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) loadGenres(path string, into map[int]string) {
	var data struct {
		Genres []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"genres"`
	}

	if err := c.get(path, nil, &data); err != nil {
		slog.Warn("could not load tmdb genre list", "path", path, "error", err)
		return
	}

	for _, g := range data.Genres {
		into[g.ID] = g.Name
	}
}

func genreNames(ids []int, lookup map[int]string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := lookup[id]; ok {
			names = append(names, name)
		}
	}
	return names
}

func (c *Client) SearchMovie(title string, year int) (*MovieResult, error) {
	var data struct {
		Results []struct {
			ID           int     `json:"id"`
			Title        string  `json:"title"`
			Overview     string  `json:"overview"`
			PosterPath   string  `json:"poster_path"`
			BackdropPath string  `json:"backdrop_path"`
			ReleaseDate  string  `json:"release_date"`
			VoteAverage  float64 `json:"vote_average"`
			GenreIDs     []int   `json:"genre_ids"`
		} `json:"results"`
	}

	params := url.Values{"query": {title}}
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}
	if err := c.get("/search/movie", params, &data); err != nil {
		return nil, err
	}

	// Folder years sometimes differ slightly from TMDB's listed release year
	// (regional releases, festival vs. wide release, etc.) — fall back to an
	// unfiltered search rather than reporting no match at all.
	if len(data.Results) == 0 && year > 0 {
		data.Results = nil
		if err := c.get("/search/movie", url.Values{"query": {title}}, &data); err != nil {
			return nil, err
		}
	}
	if len(data.Results) == 0 {
		return nil, fmt.Errorf("no tmdb match found for movie %q (%d)", title, year)
	}

	top := data.Results[0]
	return &MovieResult{
		TMDBID:       top.ID,
		Title:        top.Title,
		Overview:     top.Overview,
		PosterPath:   top.PosterPath,
		BackdropPath: top.BackdropPath,
		ReleaseDate:  top.ReleaseDate,
		VoteAverage:  top.VoteAverage,
		Genres:       genreNames(top.GenreIDs, c.movieGenres),
	}, nil
}

func (c *Client) SearchTV(title string, year int) (*TVResult, error) {
	var data struct {
		Results []struct {
			ID           int     `json:"id"`
			Name         string  `json:"name"`
			Overview     string  `json:"overview"`
			PosterPath   string  `json:"poster_path"`
			BackdropPath string  `json:"backdrop_path"`
			FirstAirDate string  `json:"first_air_date"`
			VoteAverage  float64 `json:"vote_average"`
			GenreIDs     []int   `json:"genre_ids"`
		} `json:"results"`
	}

	params := url.Values{"query": {title}}
	if year > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}
	if err := c.get("/search/tv", params, &data); err != nil {
		return nil, err
	}

	if len(data.Results) == 0 && year > 0 {
		data.Results = nil
		if err := c.get("/search/tv", url.Values{"query": {title}}, &data); err != nil {
			return nil, err
		}
	}
	if len(data.Results) == 0 {
		return nil, fmt.Errorf("no tmdb match found for tv show %q (%d)", title, year)
	}

	top := data.Results[0]
	return &TVResult{
		TMDBID:       top.ID,
		Title:        top.Name,
		Overview:     top.Overview,
		PosterPath:   top.PosterPath,
		BackdropPath: top.BackdropPath,
		FirstAirDate: top.FirstAirDate,
		VoteAverage:  top.VoteAverage,
		Genres:       genreNames(top.GenreIDs, c.tvGenres),
	}, nil
}

func (c *Client) GetTVSeasonEpisodes(tvID, seasonNumber int) ([]EpisodeResult, error) {
	var data struct {
		Episodes []struct {
			EpisodeNumber int    `json:"episode_number"`
			Name          string `json:"name"`
			Overview      string `json:"overview"`
			StillPath     string `json:"still_path"`
		} `json:"episodes"`
	}

	path := fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber)
	if err := c.get(path, nil, &data); err != nil {
		return nil, err
	}

	episodes := make([]EpisodeResult, 0, len(data.Episodes))
	for _, e := range data.Episodes {
		episodes = append(episodes, EpisodeResult{
			EpisodeNumber: e.EpisodeNumber,
			Title:         e.Name,
			Overview:      e.Overview,
			StillPath:     e.StillPath,
		})
	}
	return episodes, nil
}

type CastMember struct {
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
}

type MovieDetails struct {
	Runtime  int
	Director string
	Cast     []CastMember
}

// GetMovieDetails fetches runtime, director, and top-billed cast for a movie
// in a single request via TMDB's append_to_response mechanism.
func (c *Client) GetMovieDetails(id int) (*MovieDetails, error) {
	var data struct {
		Runtime int `json:"runtime"`
		Credits struct {
			Cast []struct {
				Name        string `json:"name"`
				Character   string `json:"character"`
				ProfilePath string `json:"profile_path"`
				Order       int    `json:"order"`
			} `json:"cast"`
			Crew []struct {
				Name string `json:"name"`
				Job  string `json:"job"`
			} `json:"crew"`
		} `json:"credits"`
	}

	path := fmt.Sprintf("/movie/%d", id)
	params := url.Values{"append_to_response": {"credits"}}
	if err := c.get(path, params, &data); err != nil {
		return nil, err
	}

	director := ""
	for _, crewMember := range data.Credits.Crew {
		if crewMember.Job == "Director" {
			director = crewMember.Name
			break
		}
	}

	cast := make([]CastMember, 0, topCastLimit)
	for i, castMember := range data.Credits.Cast {
		if i >= topCastLimit {
			break
		}
		cast = append(cast, CastMember{
			Name:        castMember.Name,
			Character:   castMember.Character,
			ProfilePath: castMember.ProfilePath,
		})
	}

	return &MovieDetails{Runtime: data.Runtime, Director: director, Cast: cast}, nil
}

type TVDetails struct {
	Creators []string
	Cast     []CastMember
}

// GetTVDetails fetches the show's creators and top-billed aggregate cast
// (roles across all episodes/seasons, TMDB's recommended endpoint for a
// whole-series cast list) in a single request.
func (c *Client) GetTVDetails(id int) (*TVDetails, error) {
	var data struct {
		CreatedBy []struct {
			Name string `json:"name"`
		} `json:"created_by"`
		AggregateCredits struct {
			Cast []struct {
				Name        string `json:"name"`
				ProfilePath string `json:"profile_path"`
				Order       int    `json:"order"`
				Roles       []struct {
					Character string `json:"character"`
				} `json:"roles"`
			} `json:"cast"`
		} `json:"aggregate_credits"`
	}

	path := fmt.Sprintf("/tv/%d", id)
	params := url.Values{"append_to_response": {"aggregate_credits"}}
	if err := c.get(path, params, &data); err != nil {
		return nil, err
	}

	creators := make([]string, 0, len(data.CreatedBy))
	for _, creator := range data.CreatedBy {
		creators = append(creators, creator.Name)
	}

	cast := make([]CastMember, 0, topCastLimit)
	for i, castMember := range data.AggregateCredits.Cast {
		if i >= topCastLimit {
			break
		}
		character := ""
		if len(castMember.Roles) > 0 {
			character = castMember.Roles[0].Character
		}
		cast = append(cast, CastMember{
			Name:        castMember.Name,
			Character:   character,
			ProfilePath: castMember.ProfilePath,
		})
	}

	return &TVDetails{Creators: creators, Cast: cast}, nil
}

// FindEpisode returns the episode metadata matching episodeNumber, if present.
func FindEpisode(episodes []EpisodeResult, episodeNumber int) (EpisodeResult, bool) {
	for _, e := range episodes {
		if e.EpisodeNumber == episodeNumber {
			return e, true
		}
	}
	return EpisodeResult{}, false
}

func JoinGenres(genres []string) string {
	return strings.Join(genres, ", ")
}
