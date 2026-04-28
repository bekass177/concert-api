package router

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"concerts-api/internal/handlers"
)

func New(db *sql.DB) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// Instantiate handlers
	concertH := &handlers.ConcertHandler{DB: db}
	seatingH := &handlers.SeatingHandler{DB: db}
	reservationH := &handlers.ReservationHandler{DB: db}
	bookingH := &handlers.BookingHandler{DB: db}
	ticketH := &handlers.TicketHandler{DB: db}

	// Routes
	r.Route("/api/v1", func(r chi.Router) {

		// Concerts
		r.Get("/concerts", concertH.ListConcerts)
		r.Get("/concerts/{concert-id}", concertH.GetConcert)

		// Seating & Reservation & Booking
		r.Route("/concerts/{concert-id}/shows/{show-id}", func(r chi.Router) {
			r.Get("/seating", seatingH.GetSeating)
			r.Post("/reservation", reservationH.CreateReservation)
			r.Post("/booking", bookingH.CreateBooking)
		})

		// Tickets
		r.Post("/tickets", ticketH.GetTickets)
		r.Post("/tickets/{ticket-id}/cancel", ticketH.CancelTicket)
	})

	return r
}
