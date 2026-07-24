package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testKID = "test-kid"

// newTestJWKS starts an httptest server serving a JWKS containing pub, and
// returns a keyfunc.Keyfunc fetching from it — the same shape Auth uses
// against Supabase's real JWKS endpoint in production.
func newTestJWKS(t *testing.T, pub *ecdsa.PublicKey) keyfunc.Keyfunc {
	t.Helper()

	jwk, err := jwkset.NewJWKFromKey(pub, jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: testKID, ALG: jwkset.AlgES256},
	})
	if err != nil {
		t.Fatalf("build jwk: %v", err)
	}

	body, err := json.Marshal(jwkset.JWKSMarshal{Keys: []jwkset.JWKMarshal{jwk.Marshal()}})
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	k, err := keyfunc.NewDefaultCtx(ctx, []string{srv.URL})
	if err != nil {
		t.Fatalf("create keyfunc: %v", err)
	}
	return k
}

func genKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return priv
}

func makeToken(t *testing.T, priv *ecdsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(priv)
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

func runAuth(t *testing.T, jwks keyfunc.Keyfunc, authHeader string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	passed := false
	handler := Auth(jwks)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	priv := genKey(t)
	jwks := newTestJWKS(t, &priv.PublicKey)

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
			rec, passed := runAuth(t, jwks, tt.authHeader)
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
	priv := genKey(t)
	jwks := newTestJWKS(t, &priv.PublicKey)
	userID := uuid.New()

	tests := []struct {
		name   string
		mutate func(jwt.MapClaims)
	}{
		{"expired token", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{"wrong audience", func(c jwt.MapClaims) { c["aud"] = "anon" }},
		{"missing audience", func(c jwt.MapClaims) { delete(c, "aud") }},
		{"subject is not a uuid", func(c jwt.MapClaims) { c["sub"] = "not-a-uuid" }},
		{"missing subject", func(c jwt.MapClaims) { delete(c, "sub") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := validClaims(userID.String())
			tt.mutate(claims)
			token := makeToken(t, priv, testKID, claims)

			rec, passed := runAuth(t, jwks, "Bearer "+token)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if passed {
				t.Error("handler should not have been called")
			}
		})
	}
}

func TestAuth_RejectsTokenSignedByUntrustedKey(t *testing.T) {
	priv := genKey(t)
	jwks := newTestJWKS(t, &priv.PublicKey)

	otherPriv := genKey(t) // never published in the JWKS
	token := makeToken(t, otherPriv, testKID, validClaims(uuid.New().String()))

	rec, passed := runAuth(t, jwks, "Bearer "+token)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if passed {
		t.Error("handler should not have been called for a token signed by an untrusted key")
	}
}

func TestAuth_RejectsHS256Token(t *testing.T) {
	// Guards against algorithm-confusion: even a well-formed token must be
	// rejected outright if it isn't ES256, regardless of what's in the JWKS.
	priv := genKey(t)
	jwks := newTestJWKS(t, &priv.PublicKey)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(uuid.New().String()))
	signed, err := token.SignedString([]byte("some-secret"))
	if err != nil {
		t.Fatalf("sign HS256 token: %v", err)
	}

	rec, passed := runAuth(t, jwks, "Bearer "+signed)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if passed {
		t.Error("handler should not have been called for an HS256 token")
	}
}

func TestAuth_AllowsValidTokenAndSetsUserID(t *testing.T) {
	priv := genKey(t)
	jwks := newTestJWKS(t, &priv.PublicKey)
	userID := uuid.New()
	token := makeToken(t, priv, testKID, validClaims(userID.String()))

	var gotUserID uuid.UUID
	var gotErr error
	handler := Auth(jwks)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
