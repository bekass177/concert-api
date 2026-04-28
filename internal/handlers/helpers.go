package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

	"concerts-api/internal/models"
)

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg})
}

func writeValidationError(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, models.ValidationErrorResponse{
		Error:  "Validation failed",
		Fields: fields,
	})
}

// --- Token helpers ---

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

const ticketCodeChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateTicketCode() (string, error) {
	code := make([]byte, 10)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(ticketCodeChars))))
		if err != nil {
			return "", err
		}
		code[i] = ticketCodeChars[n.Int64()]
	}
	return string(code), nil
}

// --- DB null helpers ---

func nullableInt64(v sql.NullInt64) *int64 {
	if v.Valid {
		return &v.Int64
	}
	return nil
}

// --- Request decode helper ---

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// isBlank checks if a string is empty or whitespace
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
