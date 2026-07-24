package apierror

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteError_Format(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{"unauthorized", Unauthorized(""), "UNAUTHORIZED", http.StatusUnauthorized},
		{"forbidden", Forbidden(""), "FORBIDDEN", http.StatusForbidden},
		{"not found", NotFound(""), "NOT_FOUND", http.StatusNotFound},
		{"validation", ValidationError("", map[string]any{"field": "start_date"}), "VALIDATION_ERROR", http.StatusUnprocessableEntity},
		{"cycle overlap", CycleOverlap("", map[string]any{"conflicting_cycle_id": "abc"}), "CYCLE_OVERLAP", http.StatusConflict},
		{"insufficient data", InsufficientData(""), "INSUFFICIENT_DATA", http.StatusUnprocessableEntity},
		{"internal", Internal(errors.New("db connection reset")), "INTERNAL_ERROR", http.StatusInternalServerError},
		{"unknown error type", errors.New("boom"), "INTERNAL_ERROR", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var body struct {
				Error struct {
					Code    string         `json:"code"`
					Message string         `json:"message"`
					Details map[string]any `json:"details,omitempty"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}

			if body.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tt.wantCode)
			}
			if body.Error.Message == "" {
				t.Error("message must not be empty")
			}
		})
	}
}

func TestInternal_DoesNotLeakCause(t *testing.T) {
	sensitive := errors.New("pq: password authentication failed for user \"postgres\"")
	apiErr := Internal(sensitive)

	b, err := json.Marshal(apiErr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if got := string(b); strings.Contains(got, "password authentication failed") {
		t.Errorf("internal error details leaked into client response: %s", got)
	}
}
