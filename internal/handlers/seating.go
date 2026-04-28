package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"concerts-api/internal/models"
)

type SeatingHandler struct {
	DB *sql.DB
}

// GET /api/v1/concerts/{concert-id}/shows/{show-id}/seating
// Returns seating info (rows + unavailable seats) for a show.
func (h *SeatingHandler) GetSeating(w http.ResponseWriter, r *http.Request) {
	concertID, err := strconv.ParseInt(chi.URLParam(r, "concert-id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "A concert or show with this ID does not exist")
		return
	}

	showID, err := strconv.ParseInt(chi.URLParam(r, "show-id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "A concert or show with this ID does not exist")
		return
	}

	// Validate concert and show exist and belong together
	if exists, err := h.concertShowExists(concertID, showID); err != nil || !exists {
		writeError(w, http.StatusNotFound, "A concert or show with this ID does not exist")
		return
	}

	// Get all rows for this show
	seatRows, err := h.getSeatingRows(showID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, models.SeatingResponse{Rows: seatRows})
}

// concertShowExists checks that the concert and show both exist
// and that the show belongs to the concert.
func (h *SeatingHandler) concertShowExists(concertID, showID int64) (bool, error) {
	var count int
	err := h.DB.QueryRow(`
		SELECT COUNT(*) FROM shows s
		JOIN concerts c ON c.id = s.concert_id
		WHERE c.id = $1 AND s.id = $2
	`, concertID, showID).Scan(&count)
	return count > 0, err
}

// getSeatingRows returns all rows for a show with their seats and availability.
func (h *SeatingHandler) getSeatingRows(showID int64) ([]models.SeatingRow, error) {
	// Get all rows ordered by their position
	rows, err := h.DB.Query(`
		SELECT id, name
		FROM location_seat_rows
		WHERE show_id = $1
		ORDER BY "order", id
	`, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seatRows []models.SeatingRow
	for rows.Next() {
		var sr models.SeatingRow
		if err := rows.Scan(&sr.ID, &sr.Name); err != nil {
			return nil, err
		}
		seatRows = append(seatRows, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// For each row, fetch seat stats
	for i := range seatRows {
		stats, err := h.getSeatStats(seatRows[i].ID)
		if err != nil {
			return nil, err
		}
		seatRows[i].Seats = stats
	}

	if seatRows == nil {
		seatRows = []models.SeatingRow{}
	}

	return seatRows, nil
}

// getSeatStats returns total seats and list of unavailable seat numbers for a row.
// A seat is unavailable if:
//   - It has a ticket_id (permanently booked), OR
//   - It has a reservation_id linked to a non-expired reservation
func (h *SeatingHandler) getSeatStats(rowID int64) (models.SeatStats, error) {
	// Count total seats in this row
	var total int
	err := h.DB.QueryRow(`
		SELECT COUNT(*) FROM location_seats WHERE location_seat_row_id = $1
	`, rowID).Scan(&total)
	if err != nil {
		return models.SeatStats{}, err
	}

	// Get unavailable seat numbers
	rows, err := h.DB.Query(`
		SELECT ls.number
		FROM location_seats ls
		LEFT JOIN reservations res ON res.id = ls.reservation_id
		WHERE ls.location_seat_row_id = $1
		  AND (
		    ls.ticket_id IS NOT NULL
		    OR (ls.reservation_id IS NOT NULL AND res.expires_at > NOW())
		  )
		ORDER BY ls.number
	`, rowID)
	if err != nil {
		return models.SeatStats{}, err
	}
	defer rows.Close()

	unavailable := []int{}
	for rows.Next() {
		var num int
		if err := rows.Scan(&num); err != nil {
			return models.SeatStats{}, err
		}
		unavailable = append(unavailable, num)
	}

	return models.SeatStats{
		Total:       total,
		Unavailable: unavailable,
	}, rows.Err()
}
