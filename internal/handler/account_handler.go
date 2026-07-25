package handler

import (
	"net/http"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type AccountHandler struct {
	service *service.AccountService
}

func NewAccountHandler(svc *service.AccountService) *AccountHandler {
	return &AccountHandler{service: svc}
}

// DeleteAccount handles DELETE /api/v1/account.
func (h *AccountHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	if err := h.service.DeleteAccount(r.Context(), userID); err != nil {
		apierror.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
