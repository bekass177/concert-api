package models

import "time"

// --- Core domain models ---

type Location struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Show struct {
	ID    int64     `json:"id"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type ShowWithConcert struct {
	ID      int64     `json:"id"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Concert Concert   `json:"concert"`
}

type Concert struct {
	ID       int64    `json:"id"`
	Artist   string   `json:"artist"`
	Location Location `json:"location"`
	Shows    []Show   `json:"shows"`
}

type Row struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SeatingRow struct {
	ID    int64     `json:"id"`
	Name  string    `json:"name"`
	Seats SeatStats `json:"seats"`
}

type SeatStats struct {
	Total       int   `json:"total"`
	Unavailable []int `json:"unavailable"`
}

type Ticket struct {
	ID        int64           `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	CreatedAt time.Time       `json:"created_at"`
	Row       Row             `json:"row"`
	Seat      int             `json:"seat"`
	Show      ShowWithConcert `json:"show"`
}

// --- Request bodies ---

type ReservationRequest struct {
	ReservationToken string        `json:"reservation_token"`
	Reservations     []SeatRequest `json:"reservations"`
	Duration         *int          `json:"duration"`
}

type SeatRequest struct {
	Row  int64 `json:"row"`
	Seat int   `json:"seat"`
}

type BookingRequest struct {
	ReservationToken string `json:"reservation_token"`
	Name             string `json:"name"`
	Address          string `json:"address"`
	City             string `json:"city"`
	Zip              string `json:"zip"`
	Country          string `json:"country"`
}

type TicketLookupRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type TicketCancelRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// --- Response wrappers ---

type ConcertsResponse struct {
	Concerts []Concert `json:"concerts"`
}

type ConcertResponse struct {
	Concert Concert `json:"concert"`
}

type SeatingResponse struct {
	Rows []SeatingRow `json:"rows"`
}

type ReservationResponse struct {
	Reserved         bool      `json:"reserved"`
	ReservationToken string    `json:"reservation_token"`
	ReservedUntil    time.Time `json:"reserved_until"`
}

type BookingResponse struct {
	Tickets []Ticket `json:"tickets"`
}

type TicketsResponse struct {
	Tickets []Ticket `json:"tickets"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ValidationErrorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}
