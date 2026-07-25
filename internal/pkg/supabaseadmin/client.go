// Package supabaseadmin wraps the small slice of the Supabase Auth Admin
// API this backend needs — currently just deleting a user.
package supabaseadmin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// maxDeleteAttempts retries the delete call a few times: Supabase's Admin
// API has been observed to intermittently return a transient "bad_jwt" 403
// with this project's secret-key format, so a single failure isn't
// trustworthy evidence the delete actually failed.
const maxDeleteAttempts = 3

// Client calls the Supabase Auth Admin API using the project's secret key.
type Client struct {
	baseURL    string
	secretKey  string
	httpClient *http.Client
}

func NewClient(baseURL, secretKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		secretKey:  secretKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// DeleteUser removes userID from auth.users (and, via Supabase's own
// cascade, its auth-schema sessions/identities/refresh tokens).
func (c *Client) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", c.baseURL, userID)

	var lastErr error
	for attempt := 1; attempt <= maxDeleteAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt-1) * time.Second):
			}
		}

		err := c.deleteOnce(ctx, url)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("delete supabase user after %d attempts: %w", maxDeleteAttempts, lastErr)
}

func (c *Client) deleteOnce(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("apikey", c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
}
