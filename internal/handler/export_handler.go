package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Vedoputra/LLUNARA-BE/internal/middleware"
	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/service"
)

type ExportHandler struct {
	service *service.ExportService
}

func NewExportHandler(svc *service.ExportService) *ExportHandler {
	return &ExportHandler{service: svc}
}

// Export handles POST /api/v1/export?format=csv|pdf&from=&to=.
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		apierror.WriteError(w, apierror.Unauthorized(""))
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "pdf" {
		apierror.WriteError(w, apierror.ValidationError("Format tidak didukung", map[string]any{"format": "harus csv atau pdf"}))
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

	switch format {
	case "csv":
		data, err := h.service.GenerateCSV(r.Context(), userID, from, to)
		if err != nil {
			apierror.WriteError(w, err)
			return
		}
		writeAttachment(w, data, "text/csv", fmt.Sprintf("llunara-export-%s-to-%s.csv", from.Format(model.DateLayout), to.Format(model.DateLayout)))

	case "pdf":
		data, err := h.service.GeneratePDF(r.Context(), userID, from, to)
		if err != nil {
			apierror.WriteError(w, err)
			return
		}
		writeAttachment(w, data, "application/pdf", fmt.Sprintf("llunara-export-%s-to-%s.pdf", from.Format(model.DateLayout), to.Format(model.DateLayout)))
	}
}

func writeAttachment(w http.ResponseWriter, data []byte, contentType, filename string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
