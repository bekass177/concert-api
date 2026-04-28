package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"concerts-api/internal/models"
)

type BookingHandler struct {
	DB *sql.DB
}

// POST /api/v1/concerts/{concert-id}/shows/{show-id}/booking
// Upgrades a reservation to full tickets.
func (h *BookingHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
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

	// Validate concert and show
	if exists, err := h.concertShowExists(concertID, showID); err != nil || !exists {
		writeError(w, http.StatusNotFound, "A concert or show with this ID does not exist")
		return
	}

	// Parse request body
	var req models.BookingRequest
	if err := decodeJSON(r, &req); err != nil {
		log.Printf("[BOOKING] Failed to decode request: %v", err)
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("[BOOKING] Request: token=%s, name=%s", req.ReservationToken, req.Name)

	// --- Validation ---
	validationErrors := map[string]string{}

	if isBlank(req.ReservationToken) {
		validationErrors["reservation_token"] = "The reservation_token field is required."
	}
	if isBlank(req.Name) {
		validationErrors["name"] = "The name field is required."
	}
	if isBlank(req.Address) {
		validationErrors["address"] = "The address field is required."
	}
	if isBlank(req.City) {
		validationErrors["city"] = "The city field is required."
	}
	if isBlank(req.Zip) {
		validationErrors["zip"] = "The zip field is required."
	}
	if isBlank(req.Country) {
		validationErrors["country"] = "The country field is required."
	}

	if len(validationErrors) > 0 {
		writeValidationError(w, validationErrors)
		return
	}

	// --- Find reservation ---
	var reservationID int64
	var expiresAt time.Time
	err = h.DB.QueryRow(`
		SELECT id, expires_at FROM reservations WHERE token = $1
	`, req.ReservationToken).Scan(&reservationID, &expiresAt)

	log.Printf("[BOOKING] Looking for token: %s", req.ReservationToken)

	if err == sql.ErrNoRows {
		log.Printf("[BOOKING] Token not found")
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	} else if err != nil {
		log.Printf("[BOOKING] Error querying reservation: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if expiresAt.Before(time.Now()) {
		log.Printf("[BOOKING] Token expired at %v", expiresAt)
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[BOOKING] Found reservation ID: %d", reservationID)

	// --- Begin transaction ---
	tx, err := h.DB.Begin()
	if err != nil {
		log.Printf("[BOOKING] Failed to begin transaction: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer tx.Rollback()

	// Create booking record
	var bookingID int64
	err = tx.QueryRow(`
		INSERT INTO bookings (name, address, city, zip, country, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id
	`, req.Name, req.Address, req.City, req.Zip, req.Country).Scan(&bookingID)
	if err != nil {
		log.Printf("[BOOKING] Failed to create booking: %v", err)
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	log.Printf("[BOOKING] Created booking ID: %d", bookingID)

	// Get all reserved seats for this reservation and show
	seatRows, err := tx.Query(`
		SELECT ls.id, ls.number, lsr.id, lsr.name
		FROM location_seats ls
		JOIN location_seat_rows lsr ON lsr.id = ls.location_seat_row_id
		WHERE ls.reservation_id = $1
		  AND lsr.show_id = $2
		ORDER BY lsr.id, ls.number
	`, reservationID, showID)
	if err != nil {
		log.Printf("[BOOKING] Failed to query seats: %v", err)
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer seatRows.Close()

	type seatInfo struct {
		seatID  int64
		number  int
		rowID   int64
		rowName string
	}

	var seats []seatInfo
	for seatRows.Next() {
		var s seatInfo
		if err := seatRows.Scan(&s.seatID, &s.number, &s.rowID, &s.rowName); err != nil {
			log.Printf("[BOOKING] Failed to scan seat: %v", err)
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		seats = append(seats, s)
	}

	if err := seatRows.Err(); err != nil {
		log.Printf("[BOOKING] Error iterating seats: %v", err)
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	log.Printf("[BOOKING] Found %d reserved seats", len(seats))

	if len(seats) == 0 {
		log.Printf("[BOOKING] No seats found for reservation %d and show %d", reservationID, showID)
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "No seats found for this reservation")
		return
	}

	// Fetch show details for response
	var show models.ShowWithConcert
	var concert models.Concert
	var location models.Location

	err = tx.QueryRow(`
		SELECT s.id, s.start, s."end",
		       c.id, c.artist,
		       l.id, l.name
		FROM shows s
		JOIN concerts c ON c.id = s.concert_id
		JOIN locations l ON l.id = c.location_id
		WHERE s.id = $1
	`, showID).Scan(
		&show.ID, &show.Start, &show.End,
		&concert.ID, &concert.Artist,
		&location.ID, &location.Name,
	)
	if err != nil {
		log.Printf("[BOOKING] Failed to fetch show details: %v", err)
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	concert.Location = location
	show.Concert = concert

	// Create a ticket for each seat
	var tickets []models.Ticket
	createdAt := time.Now().UTC()

	for _, s := range seats {
		code, err := generateTicketCode()
		if err != nil {
			log.Printf("[BOOKING] Failed to generate ticket code: %v", err)
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		var ticketID int64
		err = tx.QueryRow(`
			INSERT INTO tickets (code, booking_id, created_at)
			VALUES ($1, $2, $3)
			RETURNING id
		`, code, bookingID, createdAt).Scan(&ticketID)
		if err != nil {
			log.Printf("[BOOKING] Failed to create ticket: %v", err)
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		log.Printf("[BOOKING] Created ticket ID: %d with code: %s", ticketID, code)

		// Link ticket to seat, clear reservation
		_, err = tx.Exec(`
			UPDATE location_seats
			SET ticket_id = $1, reservation_id = NULL
			WHERE id = $2
		`, ticketID, s.seatID)
		if err != nil {
			log.Printf("[BOOKING] Failed to link ticket to seat: %v", err)
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		tickets = append(tickets, models.Ticket{
			ID:        ticketID,
			Code:      code,
			Name:      req.Name,
			CreatedAt: createdAt,
			Row:       models.Row{ID: s.rowID, Name: s.rowName},
			Seat:      s.number,
			Show:      show,
		})
	}

	// Delete the reservation
	result, err := tx.Exec(`DELETE FROM reservations WHERE id = $1`, reservationID)
	if err != nil {
		log.Printf("[BOOKING] Failed to delete reservation: %v", err)
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("[BOOKING] Failed to get rows affected: %v", err)
	}
	log.Printf("[BOOKING] Deleted %d reservation(s)", rowsAffected)

	if err := tx.Commit(); err != nil {
		log.Printf("[BOOKING] Failed to commit transaction: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	log.Printf("[BOOKING] Successfully created %d ticket(s)", len(tickets))

	if tickets == nil {
		tickets = []models.Ticket{}
	}

	writeJSON(w, http.StatusCreated, models.BookingResponse{Tickets: tickets})
}

func (h *BookingHandler) concertShowExists(concertID, showID int64) (bool, error) {
	var count int
	err := h.DB.QueryRow(`
		SELECT COUNT(*) FROM shows s
		JOIN concerts c ON c.id = s.concert_id
		WHERE c.id = $1 AND s.id = $2
	`, concertID, showID).Scan(&count)
	return count > 0, err
}
