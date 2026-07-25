package handler

import (
	"net/http"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/validator"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type ReminderHandler struct {
	service *service.ReminderService
}

func NewReminderHandler(svc *service.ReminderService) *ReminderHandler {
	return &ReminderHandler{service: svc}
}

// ListReminders handles GET /api/v1/reminders.
func (h *ReminderHandler) ListReminders(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	reminders, err := h.service.ListReminders(r.Context(), userID)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	responses := make([]model.ReminderResponse, len(reminders))
	for i, rem := range reminders {
		responses[i] = rem.ToResponse()
	}
	writeData(w, http.StatusOK, responses)
}

// UpsertReminder handles PUT /api/v1/reminders.
func (h *ReminderHandler) UpsertReminder(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	var req model.UpsertReminderRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		apierror.WriteError(w, err)
		return
	}

	rem, err := h.service.UpsertReminder(r.Context(), userID, req)
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	writeData(w, http.StatusOK, rem.ToResponse())
}

// DeleteReminder handles DELETE /api/v1/reminders/{id}.
func (h *ReminderHandler) DeleteReminder(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		apierror.WriteError(w, err)
		return
	}

	if err := h.service.DeleteReminder(r.Context(), userID, id); err != nil {
		apierror.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
