package store

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oss-agent01-survey/internal/survey"
)

type captured struct {
	method, path string
	header       http.Header
	body         []byte
}

func fakeSupabase(t *testing.T, status int, reply string) (*httptest.Server, *captured) {
	t.Helper()
	got := &captured{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.header = r.Method, r.URL.Path, r.Header.Clone()
		got.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(ts.Close)
	return ts, got
}

func TestSaveRequest(t *testing.T) {
	text := "slow builds"
	row := survey.Row{Consent: true, Roles: []string{"student"}, BiggestFrustration: &text, SurveyVersion: survey.Version}

	for _, c := range []struct {
		name, key, wantAuth string
		status              int
	}{
		{"secret key", "sb_secret_abc", "", http.StatusCreated},
		{"legacy JWT key", "eyJhbGciOi.x.y", "Bearer eyJhbGciOi.x.y", http.StatusNoContent},
	} {
		t.Run(c.name, func(t *testing.T) {
			ts, got := fakeSupabase(t, c.status, "")
			s := NewSupabase(ts.URL+"/", c.key, "survey_responses", ts.Client())
			if err := s.Save(context.Background(), row); err != nil {
				t.Fatalf("Save: %v", err)
			}
			if got.method != http.MethodPost || got.path != "/rest/v1/survey_responses" {
				t.Fatalf("request %s %s", got.method, got.path)
			}
			for k, want := range map[string]string{
				"apikey":        c.key,
				"Prefer":        "return=minimal",
				"Content-Type":  "application/json",
				"Authorization": c.wantAuth,
			} {
				if v := got.header.Get(k); v != want {
					t.Errorf("header %s = %q, want %q", k, v, want)
				}
			}
			var sent map[string]any
			if err := json.Unmarshal(got.body, &sent); err != nil {
				t.Fatal(err)
			}
			if sent["biggest_frustration"] != text || sent["how_overcome"] != nil || sent["survey_version"] != "v1" {
				t.Fatalf("body %s", got.body)
			}
			if _, ok := sent["website"]; ok {
				t.Fatal("honeypot field sent")
			}
		})
	}
}

func TestSaveErrorStatus(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError} {
		reply := `{"code":"23514","message":"violates check constraint","details":"Failing row contains (secret answers)"}`
		ts, _ := fakeSupabase(t, status, reply)
		err := NewSupabase(ts.URL, "k", "t", ts.Client()).Save(context.Background(), survey.Row{})
		var se *StatusError
		if !errors.As(err, &se) {
			t.Fatalf("%d: want *StatusError, got %v", status, err)
		}
		if se.StatusCode != status || se.Code != "23514" || se.Message != "violates check constraint" || se.Body != reply {
			t.Fatalf("%d: %+v", status, se)
		}
		if !strings.HasPrefix(err.Error(), "supabase insert: supabase returned") {
			t.Fatalf("error not wrapped: %v", err)
		}
		if strings.Contains(err.Error(), "secret answers") {
			t.Fatalf("error text leaks the row: %v", err)
		}
	}
}

func TestSaveErrorBodyTruncated(t *testing.T) {
	ts, _ := fakeSupabase(t, http.StatusBadGateway, strings.Repeat("x", 5000))
	err := NewSupabase(ts.URL, "k", "t", ts.Client()).Save(context.Background(), survey.Row{})
	var se *StatusError
	if !errors.As(err, &se) || len(se.Body) != maxErrorBody || se.Code != "" {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "supabase returned 502: xxx") {
		t.Fatalf("non-PostgREST body missing from error: %.60s", err)
	}
}

func TestSaveTransportError(t *testing.T) {
	ts, _ := fakeSupabase(t, http.StatusCreated, "")
	ts.Close()
	err := NewSupabase(ts.URL, "k", "t", nil).Save(context.Background(), survey.Row{})
	var se *StatusError
	if err == nil || errors.As(err, &se) || !strings.HasPrefix(err.Error(), "supabase insert:") {
		t.Fatalf("got %v", err)
	}
}

func TestSaveBadURL(t *testing.T) {
	err := NewSupabase("http://bad host", "k", "t", nil).Save(context.Background(), survey.Row{})
	if err == nil || !strings.HasPrefix(err.Error(), "supabase insert:") {
		t.Fatalf("got %v", err)
	}
}

func TestSaveCancelled(t *testing.T) {
	ts, _ := fakeSupabase(t, http.StatusCreated, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewSupabase(ts.URL, "k", "t", ts.Client()).Save(ctx, survey.Row{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
