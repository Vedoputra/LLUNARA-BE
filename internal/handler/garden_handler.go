package handler

import (
	"net/http"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type GardenHandler struct {
	service *service.GardenService
}

func NewGardenHandler(svc *service.GardenService) *GardenHandler {
	return &GardenHandler{service: svc}
}

// GetGarden handles GET /api/v1/garden.
func (h *GardenHandler) GetGarden(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	garden, err := h.service.GetGarden(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, garden.ToResponse())
}
