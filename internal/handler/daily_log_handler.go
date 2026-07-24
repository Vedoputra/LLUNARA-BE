package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/validator"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type DailyLogHandler struct {
	service *service.DailyLogService
}

func NewDailyLogHandler(svc *service.DailyLogService) *DailyLogHandler {
	return &DailyLogHandler{service: svc}
}

// UpsertLog handles POST /api/v1/daily-logs.
func (h *DailyLogHandler) UpsertLog(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	var req model.UpsertDailyLogRequest
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

// ListLogs handles GET /api/v1/daily-logs?from=&to=.
func (h *DailyLogHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
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

	responses := make([]model.DailyLogResponse, len(logs))
	for i, l := range logs {
		responses[i] = l.ToResponse()
	}
	writeData(w, http.StatusOK, responses)
}

// DeleteLog handles DELETE /api/v1/daily-logs/{date}.
func (h *DailyLogHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	date, err := time.Parse(model.DateLayout, chi.URLParam(r, "date"))
	if err != nil {
		apierror.WriteError(w, apierror.ValidationError("Format tanggal tidak valid", map[string]any{"date": "harus format YYYY-MM-DD"}))
		return
	}

	if err := h.service.DeleteLog(r.Context(), userID, date); err != nil {
		apierror.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
