package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"concerts-api/internal/models"
)

type TicketHandler struct {
	DB *sql.DB
}

// POST /api/v1/tickets
// Returns all tickets for a booking, authenticated by ticket code + name.
func (h *TicketHandler) GetTickets(w http.ResponseWriter, r *http.Request) {
	var req models.TicketLookupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if isBlank(req.Code) || isBlank(req.Name) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Find the ticket by code and verify the name matches the booking
	var bookingID int64
	var bookingName string
	err := h.DB.QueryRow(`
		SELECT b.id, b.name
		FROM tickets t
		JOIN bookings b ON b.id = t.booking_id
		WHERE t.code = $1
	`, req.Code).Scan(&bookingID, &bookingName)

	if err == sql.ErrNoRows || bookingName != req.Name {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get all tickets for this booking
	tickets, err := h.getTicketsByBooking(bookingID, bookingName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, models.TicketsResponse{Tickets: tickets})
}

// POST /api/v1/tickets/{ticket-id}/cancel
// Cancels (deletes) a specific ticket, freeing the seat.
func (h *TicketHandler) CancelTicket(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(chi.URLParam(r, "ticket-id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "A ticket with this ID does not exist")
		return
	}

	var req models.TicketCancelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Find ticket by ID and verify code + name
	var dbCode, bookingName string
	var bookingID int64
	err = h.DB.QueryRow(`
		SELECT t.code, b.name, b.id
		FROM tickets t
		JOIN bookings b ON b.id = t.booking_id
		WHERE t.id = $1
	`, ticketID).Scan(&dbCode, &bookingName, &bookingID)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "A ticket with this ID does not exist")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Verify code and name match
	if dbCode != req.Code || bookingName != req.Name {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Begin transaction — free seat and delete ticket
	tx, err := h.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer tx.Rollback()

	// Free the seat (remove ticket_id reference)
	_, err = tx.Exec(`
		UPDATE location_seats SET ticket_id = NULL WHERE ticket_id = $1
	`, ticketID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Delete the ticket
	_, err = tx.Exec(`DELETE FROM tickets WHERE id = $1`, ticketID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// getTicketsByBooking fetches all tickets for a given booking with full details.
func (h *TicketHandler) getTicketsByBooking(bookingID int64, bookingName string) ([]models.Ticket, error) {
	rows, err := h.DB.Query(`
		SELECT
			t.id, t.code, t.created_at,
			ls.number,
			lsr.id, lsr.name,
			s.id, s.start, s."end",
			c.id, c.artist,
			l.id, l.name
		FROM tickets t
		JOIN location_seats ls ON ls.ticket_id = t.id
		JOIN location_seat_rows lsr ON lsr.id = ls.location_seat_row_id
		JOIN shows s ON s.id = lsr.show_id
		JOIN concerts c ON c.id = s.concert_id
		JOIN locations l ON l.id = c.location_id
		WHERE t.booking_id = $1
		ORDER BY t.id
	`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		var show models.ShowWithConcert
		var concert models.Concert
		var location models.Location

		err := rows.Scan(
			&t.ID, &t.Code, &t.CreatedAt,
			&t.Seat,
			&t.Row.ID, &t.Row.Name,
			&show.ID, &show.Start, &show.End,
			&concert.ID, &concert.Artist,
			&location.ID, &location.Name,
		)
		if err != nil {
			return nil, err
		}

		concert.Location = location
		show.Concert = concert
		t.Name = bookingName
		t.Show = show
		tickets = append(tickets, t)
	}

	if tickets == nil {
		tickets = []models.Ticket{}
	}

	return tickets, rows.Err()
}
