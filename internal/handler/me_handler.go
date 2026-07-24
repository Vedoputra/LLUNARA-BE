package handler

import (
	"encoding/json"
	"net/http"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
)

// HandleMe proves the auth flow end-to-end: it returns exactly the user_id
// that Auth extracted from the verified JWT, never a client-supplied value.
func HandleMe(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"user_id": userID.String()})
}
