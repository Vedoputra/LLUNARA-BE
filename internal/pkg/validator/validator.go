// Package validator wraps go-playground/validator to decode and validate
// JSON request bodies in one step, translating failures into
// apierror.ValidationError so handlers can return them directly.
package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
)

// maxBodyBytes caps request bodies at 1 MB to prevent memory abuse.
const maxBodyBytes = 1 << 20

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Report validation errors using JSON field names (snake_case, matching
	// the API contract) instead of Go struct field names.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})

	return v
}

// DecodeAndValidate decodes the JSON request body into dst and validates it
// against dst's `validate` struct tags. Any failure — malformed JSON, an
// oversized body, or a failed validation rule — is returned as an
// *apierror.APIError (VALIDATION_ERROR, 422) ready to pass to
// apierror.WriteError.
func DecodeAndValidate(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return decodeError(err)
	}

	if err := validate.Struct(dst); err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			return apierror.ValidationError("", fieldDetails(validationErrs))
		}
		return apierror.ValidationError("", nil)
	}

	return nil
}

func decodeError(err error) error {
	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		details := map[string]any{unmarshalTypeErr.Field: fmt.Sprintf("harus bertipe %s", unmarshalTypeErr.Type)}
		return apierror.ValidationError("Format data tidak sesuai", details)
	}

	if err.Error() == "http: request body too large" {
		return apierror.ValidationError("Ukuran data melebihi batas maksimum 1 MB", nil)
	}

	return apierror.ValidationError("Data yang dikirim bukan JSON yang valid", nil)
}

func fieldDetails(errs validator.ValidationErrors) map[string]any {
	details := make(map[string]any, len(errs))
	for _, fe := range errs {
		details[fe.Field()] = describeTag(fe)
	}
	return details
}

func describeTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "harus berupa email yang valid"
	case "min":
		return fmt.Sprintf("minimal %s", fe.Param())
	case "max":
		return fmt.Sprintf("maksimal %s", fe.Param())
	case "uuid", "uuid4":
		return "harus berupa UUID yang valid"
	case "oneof":
		return fmt.Sprintf("harus salah satu dari: %s", fe.Param())
	default:
		return fmt.Sprintf("tidak valid (%s)", fe.Tag())
	}
}
