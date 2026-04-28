package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

type Location struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Show struct {
	ID    int    `json:"id"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type Concert struct {
	ID       int      `json:"id"`
	Artist   string   `json:"artist"`
	Location Location `json:"location"`
	Shows    []Show   `json:"shows"`
}

var db *sql.DB

func main() {
	connStr := "user=postgres password=12345 dbname=concerts_db sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/api/v1/concerts", getConcerts)

	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getConcerts(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT c.id, c.artist, l.id, l.name, s.id, s.start, s.finish 
		FROM concerts c
		JOIN locations l ON c.location_id = l.id
		JOIN shows s ON s.concert_id = c.id`

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	concertsMap := make(map[int]*Concert)

	for rows.Next() {
		var cID, lID, sID int
		var artist, lName, sStart, sEnd string
		rows.Scan(&cID, &artist, &lID, &lName, &sID, &sStart, &sEnd)

		if _, ok := concertsMap[cID]; !ok {
			concertsMap[cID] = &Concert{
				ID:     cID,
				Artist: artist,
				Location: Location{ID: lID, Name: lName},
				Shows:    []Show{},
			}
		}
		concertsMap[cID].Shows = append(concertsMap[cID].Shows, Show{ID: sID, Start: sStart, End: sEnd})
	}

	var result []Concert
	for _, c := range concertsMap {
		result = append(result, *c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]Concert{"concerts": result})
}