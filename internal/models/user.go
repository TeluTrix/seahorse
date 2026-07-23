package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	UserID       uuid.UUID `gorm:"primaryKey"`
	UserEmail    string    `gorm:"uniqueIndex"`
	UserPassword string
	UserRole     Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}
