package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/user"
	"github.com/google/uuid"
)

func GetOwnUser(w http.ResponseWriter, r *http.Request) {
	var u user.User
	u.UserID = uuid.New()
	u.UserEmail = "max.mustermann@hotmail.com"
	u.UserPassword = "123456789"

	response, err := json.Marshal(u)
	if err != nil {
		slog.Error("Couldn't return user.")
	}

	io.Writer.Write(w, response)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var newUser user.User
	newUser.UserID = uuid.New()

	err := json.NewDecoder(r.Body).Decode(&newUser)
	if err != nil {
		panic(1)
	}

	errDB := user.CreateUser(newUser)
	if errDB != nil {
		slog.Error(errDB.Error())
	}

	response, err := json.Marshal(newUser)
	if err != nil {
		slog.Error("Couldn't return user.")
	}

	io.Writer.Write(w, response)
}
