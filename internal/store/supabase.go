// Package store writes survey rows to Supabase through its REST API (PostgREST).
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"oss-agent01-survey/internal/survey"
)

// maxErrorBody caps how much of a failed response is kept in a StatusError.
const maxErrorBody = 1024

// StatusError reports a non-2xx response from Supabase.
type StatusError struct {
	// StatusCode is the HTTP status Supabase returned.
	StatusCode int
	// Code and Message come from the PostgREST error body, when it has one.
	// Unlike Body, they never contain the submitted row, so they are safe to log.
	Code, Message string
	// Body is the start of the raw response body, at most 1 KB. A PostgREST
	// error body can echo the rejected row, so Body is kept out of Error().
	Body string
}

// Error implements error. When the body is a PostgREST error, only its code
// and message are included, so the error is safe to log.
func (e *StatusError) Error() string {
	if e.Code != "" || e.Message != "" {
		return fmt.Sprintf("supabase returned %d: %s %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("supabase returned %d: %s", e.StatusCode, e.Body)
}

// Supabase inserts rows into one table. It is safe for concurrent use.
type Supabase struct {
	endpoint string
	key      string
	client   *http.Client
}

// NewSupabase returns a client for table in the project at baseURL, using a
// secret (or legacy service_role) key. A nil client means http.DefaultClient.
func NewSupabase(baseURL, key, table string, client *http.Client) *Supabase {
	if client == nil {
		client = http.DefaultClient
	}
	return &Supabase{
		endpoint: fmt.Sprintf("%s/rest/v1/%s", strings.TrimRight(baseURL, "/"), table),
		key:      key,
		client:   client,
	}
}

// Save inserts one row. A non-2xx answer is returned as a *StatusError.
func (s *Supabase) Save(ctx context.Context, r survey.Row) error {
	body, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("supabase insert: encode row: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("supabase insert: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")
	req.Header.Set("apikey", s.key)
	// Legacy service_role keys are JWTs and also go in Authorization.
	// New sb_secret_ keys only need the apikey header.
	if strings.HasPrefix(s.key, "eyJ") {
		req.Header.Set("Authorization", "Bearer "+s.key)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("supabase insert: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, maxErrorBody))
		se := &StatusError{StatusCode: res.StatusCode, Body: strings.TrimSpace(string(detail))}
		var pg struct{ Code, Message string }
		if json.Unmarshal(detail, &pg) == nil {
			se.Code, se.Message = pg.Code, pg.Message
		}
		return fmt.Errorf("supabase insert: %w", se)
	}
	return nil
}
