package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// These tests run against the real Supabase project — the repository layer
// is a thin wrapper over SQL, and the thing that actually matters (does the
// user_id filter really keep users out of each other's rows?) can only be
// proven against a real database, not a hand-rolled mock.
var (
	testPool           *pgxpool.Pool
	testUserID         uuid.UUID
	skipTests          bool
	testSupabaseURL    string
	testSupabaseSecret string
)

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	supabaseURL := os.Getenv("SUPABASE_URL")
	secretKey := os.Getenv("SUPABASE_SECRET_KEY")
	testSupabaseURL = supabaseURL
	testSupabaseSecret = secretKey

	if dbURL == "" || supabaseURL == "" || secretKey == "" {
		fmt.Println("repository tests: DATABASE_URL/SUPABASE_URL/SUPABASE_SECRET_KEY not set, skipping")
		skipTests = true
		os.Exit(m.Run())
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		fmt.Println("repository tests: failed to connect to database:", err)
		skipTests = true
		os.Exit(m.Run())
	}
	testPool = pool

	userID, err := createTestUser(supabaseURL, secretKey)
	if err != nil {
		fmt.Println("repository tests: failed to create test user:", err)
		skipTests = true
		os.Exit(m.Run())
	}
	testUserID = userID

	code := m.Run()

	_, _ = testPool.Exec(ctx, "delete from auth.users where id = $1", testUserID)
	testPool.Close()

	os.Exit(code)
}

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if skipTests {
		t.Skip("skipping repository integration test: no database credentials in environment")
	}
}

// createTestUser occasionally hits a transient "bad_jwt" 403 from
// Supabase's admin API when using the newer secret-key auth format — not
// something in our control, so retry a few times before giving up.
func createTestUser(supabaseURL, secretKey string) (uuid.UUID, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		id, err := createTestUserOnce(supabaseURL, secretKey)
		if err == nil {
			return id, nil
		}
		lastErr = err
	}
	return uuid.Nil, lastErr
}

func createTestUserOnce(supabaseURL, secretKey string) (uuid.UUID, error) {
	email := fmt.Sprintf("llunara-repo-test-%d@example.com", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]any{"email": email, "password": "TestPassword123!", "email_confirm": true})

	req, err := http.NewRequest(http.MethodPost, supabaseURL+"/auth/v1/admin/users", bytes.NewReader(body))
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("apikey", secretKey)
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return uuid.Nil, fmt.Errorf("create test user: status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(result.ID)
}
