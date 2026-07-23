package progress

import (
	"errors"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const completionThresholdRatio = 0.9
const completionThresholdSeconds = 30

func isCompleted(position, duration float64) bool {
	if duration <= 0 {
		return false
	}
	if duration-position <= completionThresholdSeconds {
		return true
	}
	return position/duration >= completionThresholdRatio
}

// Upsert writes progress atomically via an ON CONFLICT clause keyed on the
// (user_id, media_type, media_id) unique index, rather than a read-then-write
// (First then Save), which isn't atomic: two near-simultaneous reports for
// the same item (e.g. a "pause" and an "ended" event firing together on a
// short clip) can both see "no row yet" and then race to insert, tripping
// the unique constraint.
func Upsert(userID uuid.UUID, mediaType models.MediaType, mediaID uuid.UUID, position, duration float64) (models.WatchProgress, error) {
	wp := models.WatchProgress{
		ID:              uuid.New(),
		UserID:          userID,
		MediaType:       mediaType,
		MediaID:         mediaID,
		PositionSeconds: position,
		DurationSeconds: duration,
		Completed:       isCompleted(position, duration),
	}

	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "media_type"}, {Name: "media_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"position_seconds", "duration_seconds", "completed", "updated_at"}),
	}).Create(&wp).Error
	if err != nil {
		return models.WatchProgress{}, err
	}

	// Re-read: on a conflict path, wp.ID differs from the row actually stored.
	stored, err := Get(userID, mediaType, mediaID)
	if err != nil {
		return models.WatchProgress{}, err
	}
	return *stored, nil
}

func Get(userID uuid.UUID, mediaType models.MediaType, mediaID uuid.UUID) (*models.WatchProgress, error) {
	var wp models.WatchProgress
	result := db.DB.Where("user_id = ? AND media_type = ? AND media_id = ?", userID, mediaType, mediaID).First(&wp)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &wp, nil
}

// GetMany returns a map keyed by MediaID for every matching progress row,
// used to annotate a list of episodes without one query per row.
func GetMany(userID uuid.UUID, mediaType models.MediaType, mediaIDs []uuid.UUID) (map[uuid.UUID]models.WatchProgress, error) {
	result := map[uuid.UUID]models.WatchProgress{}
	if len(mediaIDs) == 0 {
		return result, nil
	}

	var rows []models.WatchProgress
	if err := db.DB.Where("user_id = ? AND media_type = ? AND media_id IN ?", userID, mediaType, mediaIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.MediaID] = row
	}
	return result, nil
}
