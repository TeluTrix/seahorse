package db

import (
	"os"

	"github.com/TeluTrix/seahorse/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func OpenConnection() {
	var err error
	DB, err = gorm.Open(sqlite.Open(os.Getenv("SEAHORSE_DB")), &gorm.Config{})
	if err != nil {
		panic(1)
	}

	if err := DB.AutoMigrate(&models.User{}); err != nil {
		panic(err)
	}
}
