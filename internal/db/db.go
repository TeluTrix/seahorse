package db

import (
	"log"
	"os"
	"time"

	"github.com/TeluTrix/seahorse/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func OpenConnection(dbPath string) error {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return err
	}

	return DB.AutoMigrate(
		&models.User{},
		&models.Movie{},
		&models.TVShow{},
		&models.Season{},
		&models.Episode{},
		&models.WatchProgress{},
	)
}
