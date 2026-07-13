package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/TeluTrix/seahorse/internal/api"
	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/gorilla/mux"
)

var ListenOn = os.Getenv("SEAHORSE_LISTEN_ON")
var Port = os.Getenv("SEAHORSE_PORT")

func main() {
	db.OpenConnection()

	r := mux.NewRouter()
	r.HandleFunc("/user", api.GetOwnUser).Methods("GET")
	r.HandleFunc("/user", api.CreateUser).Methods("POST")

	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf("%s:%s", ListenOn, Port),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
