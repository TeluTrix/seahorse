package models

import (
	"time"

	"github.com/google/uuid"
)

type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
)

type WatchProgress struct {
	ID              uuid.UUID `gorm:"primaryKey"`
	UserID          uuid.UUID `gorm:"uniqueIndex:idx_user_media"`
	MediaType       MediaType `gorm:"uniqueIndex:idx_user_media"`
	MediaID         uuid.UUID `gorm:"uniqueIndex:idx_user_media"`
	PositionSeconds float64
	DurationSeconds float64
	Completed       bool
	UpdatedAt       time.Time
}
