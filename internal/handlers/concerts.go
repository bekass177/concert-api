package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"concerts-api/internal/models"
)

type ConcertHandler struct {
	DB *sql.DB
}

// GET /api/v1/concerts
// Returns a list of all concerts with their shows.
func (h *ConcertHandler) ListConcerts(w http.ResponseWriter, r *http.Request) {
	concerts, err := h.fetchConcerts(0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, models.ConcertsResponse{Concerts: concerts})
}

// GET /api/v1/concerts/{concert-id}
// Returns a single concert by ID.
func (h *ConcertHandler) GetConcert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "concert-id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "A concert with this ID does not exist")
		return
	}

	concerts, err := h.fetchConcerts(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if len(concerts) == 0 {
		writeError(w, http.StatusNotFound, "A concert with this ID does not exist")
		return
	}

	writeJSON(w, http.StatusOK, models.ConcertResponse{Concert: concerts[0]})
}

// fetchConcerts retrieves concerts from the DB.
// If concertID > 0, only that concert is returned.
func (h *ConcertHandler) fetchConcerts(concertID int64) ([]models.Concert, error) {
	query := `
		SELECT c.id, c.artist, l.id, l.name
		FROM concerts c
		JOIN locations l ON l.id = c.location_id
	`
	args := []any{}

	if concertID > 0 {
		query += " WHERE c.id = $1"
		args = append(args, concertID)
	}

	query += " ORDER BY c.id"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var concerts []models.Concert
	for rows.Next() {
		var c models.Concert
		if err := rows.Scan(&c.ID, &c.Artist, &c.Location.ID, &c.Location.Name); err != nil {
			return nil, err
		}
		c.Shows = []models.Show{}
		concerts = append(concerts, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetch shows for each concert
	for i := range concerts {
		shows, err := h.fetchShows(concerts[i].ID)
		if err != nil {
			return nil, err
		}
		concerts[i].Shows = shows
	}

	if concerts == nil {
		concerts = []models.Concert{}
	}

	return concerts, nil
}

// fetchShows returns all shows for the given concert.
func (h *ConcertHandler) fetchShows(concertID int64) ([]models.Show, error) {
	rows, err := h.DB.Query(
		`SELECT id, start, "end" FROM shows WHERE concert_id = $1 ORDER BY start`,
		concertID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shows []models.Show
	for rows.Next() {
		var s models.Show
		if err := rows.Scan(&s.ID, &s.Start, &s.End); err != nil {
			return nil, err
		}
		shows = append(shows, s)
	}

	if shows == nil {
		shows = []models.Show{}
	}

	return shows, rows.Err()
}
