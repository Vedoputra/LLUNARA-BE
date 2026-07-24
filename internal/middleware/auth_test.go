package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret-for-unit-tests-only"

func makeToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func validClaims(sub string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub": sub,
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

func runAuth(t *testing.T, secret, authHeader string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	passed := false
	handler := Auth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passed = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, passed
}

func TestAuth_RejectsMissingOrGarbageTokens(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
	}{
		{"no header at all", ""},
		{"missing bearer prefix", "sometoken"},
		{"garbage token", "Bearer not-a-real-jwt"},
		{"empty bearer token", "Bearer "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, passed := runAuth(t, testSecret, tt.authHeader)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if passed {
				t.Error("handler should not have been called")
			}
		})
	}
}

func TestAuth_RejectsInvalidClaims(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name   string
		mutate func(jwt.MapClaims)
		secret string // secret used to sign; empty means testSecret
	}{
		{
			name:   "signed with wrong secret",
			mutate: func(c jwt.MapClaims) {},
			secret: "a-completely-different-secret",
		},
		{
			name: "expired token",
			mutate: func(c jwt.MapClaims) {
				c["exp"] = time.Now().Add(-time.Hour).Unix()
			},
		},
		{
			name: "wrong audience",
			mutate: func(c jwt.MapClaims) {
				c["aud"] = "anon"
			},
		},
		{
			name: "missing audience",
			mutate: func(c jwt.MapClaims) {
				delete(c, "aud")
			},
		},
		{
			name: "subject is not a uuid",
			mutate: func(c jwt.MapClaims) {
				c["sub"] = "not-a-uuid"
			},
		},
		{
			name: "missing subject",
			mutate: func(c jwt.MapClaims) {
				delete(c, "sub")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := validClaims(userID.String())
			tt.mutate(claims)

			signSecret := tt.secret
			if signSecret == "" {
				signSecret = testSecret
			}
			token := makeToken(t, signSecret, claims)

			rec, passed := runAuth(t, testSecret, "Bearer "+token)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if passed {
				t.Error("handler should not have been called")
			}
		})
	}
}

func TestAuth_RejectsNoneAlgorithm(t *testing.T) {
	// alg=none tokens must never be accepted regardless of claims.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims(uuid.New().String()))
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none-alg token: %v", err)
	}

	rec, passed := runAuth(t, testSecret, "Bearer "+signed)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if passed {
		t.Error("handler should not have been called for alg=none token")
	}
}

func TestAuth_AllowsValidTokenAndSetsUserID(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, testSecret, validClaims(userID.String()))

	var gotUserID uuid.UUID
	var gotErr error
	handler := Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, gotErr = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotErr != nil {
		t.Fatalf("GetUserID returned error: %v", gotErr)
	}
	if gotUserID != userID {
		t.Errorf("user id = %v, want %v", gotUserID, userID)
	}
}

func TestGetUserID_MissingFromContext(t *testing.T) {
	if _, err := GetUserID(context.Background()); err == nil {
		t.Fatal("expected error when user_id is not present in context")
	}
}
