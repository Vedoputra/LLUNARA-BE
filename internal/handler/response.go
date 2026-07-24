package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
)

// writeData writes a successful JSON response wrapped in the API's
// standard { "data": ... } envelope (mirrors apierror's { "error": ... }).
func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

// parseUUIDParam reads a chi URL param and validates it's a UUID, returning
// a ready-to-write VALIDATION_ERROR if not.
func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.UUID{}, apierror.ValidationError("Format ID tidak valid", map[string]any{name: "harus berupa UUID yang valid"})
	}
	return id, nil
}
