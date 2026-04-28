package main

import (
	"fmt"
	"log"
	"net/http"

	"concerts-api/internal/db"
	"concerts-api/router"
)

func main() {
	// Database connection config
	// Change these values to match your setup
	database, err := db.Connect(db.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "12345",
		DBName:   "concerts_db",
		SSLMode:  "disable",
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Build router
	r := router.New(database)

	addr := ":8080"
	fmt.Printf("Concerts API running at http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
