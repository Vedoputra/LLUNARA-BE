package validator

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
)

type testPayload struct {
	StartDate string `json:"start_date" validate:"required"`
	Email     string `json:"email" validate:"required,email"`
}

func newRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
}

func TestDecodeAndValidate_MissingRequiredField(t *testing.T) {
	req := newRequest(t, `{"email":"a@example.com"}`)

	var dst testPayload
	err := DecodeAndValidate(req, &dst)
	if err == nil {
		t.Fatal("expected an error for missing required field, got nil")
	}

	apiErr, ok := err.(*apierror.APIError)
	if !ok {
		t.Fatalf("expected *apierror.APIError, got %T", err)
	}
	if apiErr.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
	if apiErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("http status = %d, want 422", apiErr.HTTPStatus)
	}
	if _, ok := apiErr.Details["start_date"]; !ok {
		t.Errorf("expected details to mention field %q, got %v", "start_date", apiErr.Details)
	}
}

func TestDecodeAndValidate_InvalidEmail(t *testing.T) {
	req := newRequest(t, `{"start_date":"2026-01-01","email":"not-an-email"}`)

	var dst testPayload
	err := DecodeAndValidate(req, &dst)
	apiErr, ok := err.(*apierror.APIError)
	if !ok {
		t.Fatalf("expected *apierror.APIError, got %T (%v)", err, err)
	}
	if _, ok := apiErr.Details["email"]; !ok {
		t.Errorf("expected details to mention field %q, got %v", "email", apiErr.Details)
	}
}

func TestDecodeAndValidate_ValidPayload(t *testing.T) {
	req := newRequest(t, `{"start_date":"2026-01-01","email":"a@example.com"}`)

	var dst testPayload
	if err := DecodeAndValidate(req, &dst); err != nil {
		t.Fatalf("unexpected error for valid payload: %v", err)
	}
	if dst.StartDate != "2026-01-01" || dst.Email != "a@example.com" {
		t.Errorf("unexpected decoded values: %+v", dst)
	}
}

func TestDecodeAndValidate_MalformedJSON(t *testing.T) {
	req := newRequest(t, `{not valid json`)

	var dst testPayload
	err := DecodeAndValidate(req, &dst)
	apiErr, ok := err.(*apierror.APIError)
	if !ok {
		t.Fatalf("expected *apierror.APIError, got %T", err)
	}
	if apiErr.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
}

func TestDecodeAndValidate_UnknownField(t *testing.T) {
	req := newRequest(t, `{"start_date":"2026-01-01","email":"a@example.com","surprise":"field"}`)

	var dst testPayload
	if err := DecodeAndValidate(req, &dst); err == nil {
		t.Fatal("expected an error for unknown field, got nil")
	}
}

func TestDecodeAndValidate_BodyTooLarge(t *testing.T) {
	huge := bytes.Repeat([]byte("a"), maxBodyBytes+1)
	body := `{"start_date":"` + string(huge) + `","email":"a@example.com"}`
	req := newRequest(t, body)

	var dst testPayload
	err := DecodeAndValidate(req, &dst)
	apiErr, ok := err.(*apierror.APIError)
	if !ok {
		t.Fatalf("expected *apierror.APIError, got %T", err)
	}
	if apiErr.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
}
