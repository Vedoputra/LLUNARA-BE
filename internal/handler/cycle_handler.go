package handler

import (
	"net/http"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/validator"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type CycleHandler struct {
	service *service.CycleService
}

func NewCycleHandler(svc *service.CycleService) *CycleHandler {
	return &CycleHandler{service: svc}
}

// StartCycle handles POST /api/v1/cycles.
func (h *CycleHandler) StartCycle(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	var req model.StartCycleRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		apierror.WriteError(w, err)
		return
	}

	startDate, err := req.ParseStartDate()
	if err != nil {
		apierror.WriteError(w, apierror.ValidationError("Format tanggal tidak valid", map[string]any{"start_date": "harus format YYYY-MM-DD"}))
		return
	}

	cycle, err := h.service.StartCycle(r.Context(), userID, startDate)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusCreated, cycle.ToResponse())
}

// UpdateCycle handles PATCH /api/v1/cycles/{id} — records when a period
// ended.
func (h *CycleHandler) UpdateCycle(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	cycleID, err := parseUUIDParam(r, "id")
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	var req model.UpdateCycleRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		apierror.WriteError(w, err)
		return
	}

	endDate, err := req.ParseEndDate()
	if err != nil {
		apierror.WriteError(w, apierror.ValidationError("Format tanggal tidak valid", map[string]any{"end_date": "harus format YYYY-MM-DD"}))
		return
	}

	cycle, err := h.service.EndCycle(r.Context(), userID, cycleID, endDate)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, cycle.ToResponse())
}

// DeleteCycle handles DELETE /api/v1/cycles/{id}.
func (h *CycleHandler) DeleteCycle(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	cycleID, err := parseUUIDParam(r, "id")
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	if err := h.service.DeleteCycle(r.Context(), userID, cycleID); err != nil {
		apierror.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCycles handles GET /api/v1/cycles.
func (h *CycleHandler) ListCycles(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	cycles, err := h.service.ListCycles(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	responses := make([]model.CycleResponse, len(cycles))
	for i, c := range cycles {
		responses[i] = c.ToResponse()
	}
	writeData(w, http.StatusOK, responses)
}
