package handler

import (
	"net/http"
	"time"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/validator"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type WellnessHandler struct {
	service *service.WellnessService
}

func NewWellnessHandler(svc *service.WellnessService) *WellnessHandler {
	return &WellnessHandler{service: svc}
}

// UpsertLog handles POST /api/v1/wellness.
func (h *WellnessHandler) UpsertLog(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	var req model.UpsertWellnessLogRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		apierror.WriteError(w, err)
		return
	}

	log, err := h.service.UpsertLog(r.Context(), userID, req)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, log.ToResponse())
}

// ListLogs handles GET /api/v1/wellness?from=&to=.
func (h *WellnessHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	from, err := time.Parse(model.DateLayout, r.URL.Query().Get("from"))
	if err != nil {
		apierror.WriteError(w, apierror.ValidationError("Parameter from tidak valid", map[string]any{"from": "harus format YYYY-MM-DD"}))
		return
	}
	to, err := time.Parse(model.DateLayout, r.URL.Query().Get("to"))
	if err != nil {
		apierror.WriteError(w, apierror.ValidationError("Parameter to tidak valid", map[string]any{"to": "harus format YYYY-MM-DD"}))
		return
	}

	logs, err := h.service.ListLogs(r.Context(), userID, from, to)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	responses := make([]model.WellnessLogResponse, len(logs))
	for i, l := range logs {
		responses[i] = l.ToResponse()
	}
	writeData(w, http.StatusOK, responses)
}
