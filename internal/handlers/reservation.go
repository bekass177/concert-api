package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"concerts-api/internal/models"
)

type ReservationHandler struct {
	DB *sql.DB
}

const defaultDuration = 300 // seconds

// POST /api/v1/concerts/{concert-id}/shows/{show-id}/reservation
// Reserves one or more seats for a show.
func (h *ReservationHandler) CreateReservation(w http.ResponseWriter, r *http.Request) {
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

	// Validate concert/show
	if exists, err := h.concertShowExists(concertID, showID); err != nil || !exists {
		writeError(w, http.StatusNotFound, "A concert or show with this ID does not exist")
		return
	}

	// Parse request body
	var req models.ReservationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set default duration if not provided
	duration := defaultDuration
	if req.Duration != nil {
		duration = *req.Duration
	}

	// --- Validate fields ---
	validationErrors := map[string]string{}

	if req.Reservations == nil {
		validationErrors["reservations"] = "The reservations field is required."
	}

	if req.Duration != nil && (duration < 1 || duration > 300) {
		validationErrors["duration"] = "The duration must be between 1 and 300."
	}

	if len(validationErrors) > 0 {
		writeValidationError(w, validationErrors)
		return
	}

	// --- Validate token BEFORE transaction ---
	var reservationID int64
	var token string
	isReplacing := false

	if req.ReservationToken != "" {
		// Validate token exists
		err := h.DB.QueryRow(`
			SELECT id FROM reservations WHERE token = $1
		`, req.ReservationToken).Scan(&reservationID)

		if err == sql.ErrNoRows {
			writeError(w, http.StatusForbidden, "Invalid reservation token")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		token = req.ReservationToken
		isReplacing = true
	} else {
		// Generate new token
		token, err = generateToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	// --- Begin transaction ---
	tx, err := h.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer tx.Rollback()

	expiresAt := time.Now().UTC().Add(time.Duration(duration) * time.Second)

	// If replacing, clear old and update; else create new
	if isReplacing {
		// Clear old seat reservations for this show only
		_, err = tx.Exec(`
			UPDATE location_seats
			SET reservation_id = NULL
			WHERE reservation_id = $1
			  AND location_seat_row_id IN (
			    SELECT id FROM location_seat_rows WHERE show_id = $2
			  )
		`, reservationID, showID)
		if err != nil {
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		// Update expires_at for existing reservation
		_, err = tx.Exec(`
			UPDATE reservations SET expires_at = $1 WHERE id = $2
		`, expiresAt, reservationID)
		if err != nil {
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	} else {
		// Create new reservation
		err = tx.QueryRow(`
			INSERT INTO reservations (token, expires_at)
			VALUES ($1, $2)
			RETURNING id
		`, token, expiresAt).Scan(&reservationID)
		if err != nil {
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	// --- Validate and reserve each seat ---
	if len(req.Reservations) > 0 {
		seatValidationErrors := map[string]string{}

		for _, sr := range req.Reservations {
			// Validate row exists and belongs to this show
			var rowExists bool
			err := tx.QueryRow(`
				SELECT EXISTS(
				  SELECT 1 FROM location_seat_rows
				  WHERE id = $1 AND show_id = $2
				)
			`, sr.Row, showID).Scan(&rowExists)

			if err != nil {
				tx.Rollback()
				writeError(w, http.StatusInternalServerError, "Internal server error")
				return
			}

			if !rowExists {
				seatValidationErrors["reservations"] = fmt.Sprintf("Seat %d in row %d is invalid.", sr.Seat, sr.Row)
				break
			}

			// Validate seat exists in this row
			var seatID int64
			var seatReservationID sql.NullInt64
			var seatTicketID sql.NullInt64

			err = tx.QueryRow(`
				SELECT id, reservation_id, ticket_id
				FROM location_seats
				WHERE location_seat_row_id = $1 AND number = $2
			`, sr.Row, sr.Seat).Scan(&seatID, &seatReservationID, &seatTicketID)

			if err == sql.ErrNoRows {
				seatValidationErrors["reservations"] = fmt.Sprintf("Seat %d in row %d is invalid.", sr.Seat, sr.Row)
				break
			} else if err != nil {
				tx.Rollback()
				writeError(w, http.StatusInternalServerError, "Internal server error")
				return
			}

			// Check if seat is already taken by a ticket
			if seatTicketID.Valid {
				seatValidationErrors["reservations"] = fmt.Sprintf("Seat %d in row %d is already taken.", sr.Seat, sr.Row)
				break
			}

			// Check if seat is reserved by someone else (active reservation, not our own)
			if seatReservationID.Valid && seatReservationID.Int64 != reservationID {
				var isExpired bool
				err = tx.QueryRow(`
					SELECT expires_at <= NOW() FROM reservations WHERE id = $1
				`, seatReservationID.Int64).Scan(&isExpired)

				if err != nil && err != sql.ErrNoRows {
					tx.Rollback()
					writeError(w, http.StatusInternalServerError, "Internal server error")
					return
				}

				if err == nil && !isExpired {
					seatValidationErrors["reservations"] = fmt.Sprintf("Seat %d in row %d is already taken.", sr.Seat, sr.Row)
					break
				}
			}
		}

		if len(seatValidationErrors) > 0 {
			tx.Rollback()
			writeValidationError(w, seatValidationErrors)
			return
		}

		// All seats valid — now reserve them
		for _, sr := range req.Reservations {
			_, err = tx.Exec(`
				UPDATE location_seats
				SET reservation_id = $1
				WHERE location_seat_row_id = $2 AND number = $3
			`, reservationID, sr.Row, sr.Seat)
			if err != nil {
				tx.Rollback()
				writeError(w, http.StatusInternalServerError, "Internal server error")
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, models.ReservationResponse{
		Reserved:         true,
		ReservationToken: token,
		ReservedUntil:    expiresAt,
	})
}

func (h *ReservationHandler) concertShowExists(concertID, showID int64) (bool, error) {
	var count int
	err := h.DB.QueryRow(`
		SELECT COUNT(*) FROM shows s
		JOIN concerts c ON c.id = s.concert_id
		WHERE c.id = $1 AND s.id = $2
	`, concertID, showID).Scan(&count)
	return count > 0, err
}
