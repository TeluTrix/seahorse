package user

import (
	"time"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	UserID       uuid.UUID `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	UserRole     string    `json:"user_role"`
	UserPassword string    `json:"-"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func convertToSchema(user User) models.User {
	var userToStore models.User

	userToStore.CreatedAt = time.Now()
	userToStore.UpdatedAt = time.Now()
	userToStore.UserID = uuid.New()
	userToStore.UserEmail = user.UserEmail
	userToStore.UserRole = user.UserRole
	userToStore.UserPassword = user.UserPassword

	return userToStore
}

func CreateUser(userIn User) error {
	hashedPassword, err := HashPassword(userIn.UserPassword)
	if err != nil {
		return err
	}

	userIn.UserPassword = hashedPassword

	user := convertToSchema(userIn)

	result := db.DB.Create(&user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
