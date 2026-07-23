package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Movie struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	TMDBID       int
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	ReleaseDate  string
	VoteAverage  float64
	Genres       string
	Runtime      int // minutes
	Director     string
	Cast         string `gorm:"type:text"` // JSON-encoded []tmdb.CastMember
	FilePath     string `gorm:"uniqueIndex"`
	CoverCached  bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

type TVShow struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	TMDBID       int
	Title        string
	Overview     string
	PosterPath   string
	BackdropPath string
	FirstAirDate string
	VoteAverage  float64
	Genres       string
	Creators     string // comma-joined, same pattern as Genres
	Cast         string `gorm:"type:text"` // JSON-encoded []tmdb.CastMember
	FolderPath   string `gorm:"uniqueIndex"`
	CoverCached  bool
	Seasons      []Season `gorm:"foreignKey:TVShowID"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

type Season struct {
	ID           uuid.UUID `gorm:"primaryKey"`
	TVShowID     uuid.UUID `gorm:"index"`
	SeasonNumber int
	Episodes     []Episode `gorm:"foreignKey:SeasonID"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Episode struct {
	ID            uuid.UUID `gorm:"primaryKey"`
	SeasonID      uuid.UUID `gorm:"index"`
	EpisodeNumber int
	Title         string
	Overview      string
	StillPath     string
	Runtime       int    // minutes; 0 if TMDB didn't have it
	FilePath      string `gorm:"uniqueIndex"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
