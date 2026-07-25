package supabaseadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteUser_SucceedsOnFirstTry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization header = %q, want Bearer secret", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "secret")
	if err := client.DeleteUser(context.Background(), uuid.New()); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDeleteUser_RetriesTransientFailureThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"bad_jwt"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "secret")
	if err := client.DeleteUser(context.Background(), uuid.New()); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 failure + 1 success), got %d", calls)
	}
}

func TestDeleteUser_GivesUpAfterMaxAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "secret")
	if err := client.DeleteUser(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if calls != maxDeleteAttempts {
		t.Errorf("expected %d calls, got %d", maxDeleteAttempts, calls)
	}
}
