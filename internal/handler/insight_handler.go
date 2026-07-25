package handler

import (
	"net/http"
	"strconv"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type InsightHandler struct {
	service *service.InsightService
}

func NewInsightHandler(svc *service.InsightService) *InsightHandler {
	return &InsightHandler{service: svc}
}

// GetCycleSummary handles GET /api/v1/insights/summary.
func (h *InsightHandler) GetCycleSummary(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	summary, err := h.service.GetCycleSummary(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, summary.ToResponse())
}

// GetSymptomInsights handles GET /api/v1/insights/symptoms?months=.
func (h *InsightHandler) GetSymptomInsights(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	months, err := parseMonthsParam(r)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	insights, err := h.service.GetSymptomInsights(r.Context(), userID, months)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, insights.ToResponse())
}

// GetMoodInsights handles GET /api/v1/insights/mood?months=.
func (h *InsightHandler) GetMoodInsights(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	months, err := parseMonthsParam(r)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	insights, err := h.service.GetMoodInsights(r.Context(), userID, months)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, insights.ToResponse())
}

// parseMonthsParam returns 0 (letting the service apply its default) when
// the query param is absent, or a VALIDATION_ERROR-ready error if present
// but not a sane integer.
func parseMonthsParam(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("months")
	if raw == "" {
		return 0, nil
	}
	months, err := strconv.Atoi(raw)
	if err != nil || months < 1 || months > 24 {
		return 0, apierror.ValidationError("Parameter months tidak valid", map[string]any{"months": "harus berupa angka antara 1 dan 24"})
	}
	return months, nil
}
