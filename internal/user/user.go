package user

import (
	"errors"
	"time"

	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

// Count returns the total number of registered users.
func Count() (int64, error) {
	var count int64
	err := db.DB.Model(&models.User{}).Count(&count).Error
	return count, err
}

func CreateUser(email, password string) (models.User, error) {
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return models.User{}, err
	}

	count, err := Count()
	if err != nil {
		return models.User{}, err
	}

	role := models.RoleUser
	if count == 0 {
		role = models.RoleAdmin
	}

	newUser := models.User{
		UserID:       uuid.New(),
		UserEmail:    email,
		UserPassword: hashed,
		UserRole:     role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := db.DB.Create(&newUser).Error; err != nil {
		return models.User{}, err
	}

	return newUser, nil
}

func Authenticate(email, password string) (models.User, error) {
	var u models.User
	if err := db.DB.Where("user_email = ?", email).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, ErrInvalidCredentials
		}
		return models.User{}, err
	}

	match, err := auth.VerifyPassword(password, u.UserPassword)
	if err != nil || !match {
		return models.User{}, ErrInvalidCredentials
	}

	return u, nil
}

func GetByID(id uuid.UUID) (models.User, error) {
	var u models.User
	if err := db.DB.First(&u, "user_id = ?", id).Error; err != nil {
		return models.User{}, err
	}
	return u, nil
}

func ListAll() ([]models.User, error) {
	var users []models.User
	if err := db.DB.Order("user_email").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func SetPassword(id uuid.UUID, newPassword string) error {
	hashed, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return db.DB.Model(&models.User{}).Where("user_id = ?", id).Update("user_password", hashed).Error
}
