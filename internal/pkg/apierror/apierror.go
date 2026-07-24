// Package apierror provides a standardized error shape for API responses,
// per PRD Section 7.3. Client-facing messages never leak internal details;
// the original error is logged instead.
package apierror

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// APIError is the error shape returned to clients: { "error": {...} }.
type APIError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	HTTPStatus int            `json:"-"`

	// cause is the original internal error, logged but never serialized.
	cause error
}

func (e *APIError) Error() string {
	return e.Message
}

func (e *APIError) Unwrap() error {
	return e.cause
}

func Unauthorized(message string) *APIError {
	if message == "" {
		message = "Autentikasi diperlukan"
	}
	return &APIError{Code: "UNAUTHORIZED", Message: message, HTTPStatus: http.StatusUnauthorized}
}

func Forbidden(message string) *APIError {
	if message == "" {
		message = "Akses ditolak"
	}
	return &APIError{Code: "FORBIDDEN", Message: message, HTTPStatus: http.StatusForbidden}
}

func NotFound(message string) *APIError {
	if message == "" {
		message = "Data tidak ditemukan"
	}
	return &APIError{Code: "NOT_FOUND", Message: message, HTTPStatus: http.StatusNotFound}
}

func ValidationError(message string, details map[string]any) *APIError {
	if message == "" {
		message = "Data yang dikirim tidak valid"
	}
	return &APIError{Code: "VALIDATION_ERROR", Message: message, Details: details, HTTPStatus: http.StatusUnprocessableEntity}
}

func CycleOverlap(message string, details map[string]any) *APIError {
	if message == "" {
		message = "Tanggal ini bertumpuk dengan siklus yang sudah tercatat"
	}
	return &APIError{Code: "CYCLE_OVERLAP", Message: message, Details: details, HTTPStatus: http.StatusConflict}
}

func InsufficientData(message string) *APIError {
	if message == "" {
		message = "Data belum mencukupi untuk operasi ini"
	}
	return &APIError{Code: "INSUFFICIENT_DATA", Message: message, HTTPStatus: http.StatusUnprocessableEntity}
}

// Internal wraps an unexpected internal error. The client only ever sees a
// generic message; err is kept as the cause for logging in WriteError.
func Internal(err error) *APIError {
	return &APIError{
		Code:       "INTERNAL_ERROR",
		Message:    "Terjadi kesalahan pada server, silakan coba lagi",
		HTTPStatus: http.StatusInternalServerError,
		cause:      err,
	}
}

type errorResponse struct {
	Error *APIError `json:"error"`
}

// WriteError logs the real error (never the sanitized client message alone)
// and writes the standardized { "error": {...} } JSON response. Any error
// that isn't already an *APIError is treated as an unexpected internal one.
func WriteError(w http.ResponseWriter, err error) {
	apiErr, ok := err.(*APIError)
	if !ok {
		apiErr = Internal(err)
	}

	logCause := apiErr.cause
	if logCause == nil {
		logCause = apiErr
	}
	slog.Error("api error", "code", apiErr.Code, "http_status", apiErr.HTTPStatus, "error", logCause)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: apiErr})
}
