package handler

import (
	"net/http"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/validator"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type SymptomHandler struct {
	service *service.SymptomService
}

func NewSymptomHandler(svc *service.SymptomService) *SymptomHandler {
	return &SymptomHandler{service: svc}
}

// ListSymptoms handles GET /api/v1/symptoms.
func (h *SymptomHandler) ListSymptoms(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	symptoms, err := h.service.ListSymptoms(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	responses := make([]model.SymptomResponse, len(symptoms))
	for i, s := range symptoms {
		responses[i] = s.ToResponse()
	}
	writeData(w, http.StatusOK, responses)
}

// CreateSymptom handles POST /api/v1/symptoms.
func (h *SymptomHandler) CreateSymptom(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	var req model.CreateSymptomRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		apierror.WriteError(w, err)
		return
	}

	created, err := h.service.CreateSymptom(r.Context(), userID, req)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusCreated, created.ToResponse())
}

// DeleteSymptom handles DELETE /api/v1/symptoms/{id}.
func (h *SymptomHandler) DeleteSymptom(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	symptomID, err := parseUUIDParam(r, "id")
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	if err := h.service.DeleteSymptom(r.Context(), userID, symptomID); err != nil {
		apierror.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
