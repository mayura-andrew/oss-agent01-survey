package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"oss-agent01-survey/internal/survey"
)

const validBody = `{"consent":true,"roles":["student","other"],"role_other":"  Lecturer  ","oss_experience":"beginner","barriers":["no_guidance","low_confidence"],"likert_structure":4,"likert_env_setup":5,"likert_docs_outdated":3,"likert_tracing_comfortable":2,"time_to_confidence":"1_3h","exploration_methods":["readme_docs","llm_tools"],"diagram_frequency":"rarely","feature_zero_install":5,"feature_visual_map":4,"feature_ai_assistant":3,"feature_privacy":5,"likelihood_to_use":4,"biggest_frustration":"Setup","how_overcome":"","completion_seconds":321,"website":""}`

type fakeSaver struct {
	mu   sync.Mutex
	err  error
	rows []survey.Row
}

func (f *fakeSaver) Save(_ context.Context, r survey.Row) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, r)
	return f.err
}

// fakeLimiter allows `limit` hits per key and records the keys it saw.
type fakeLimiter struct {
	mu    sync.Mutex
	limit int
	seen  map[string]int
}

func (f *fakeLimiter) Allow(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seen == nil {
		f.seen = map[string]int{}
	}
	f.seen[key]++
	return f.seen[key] <= f.limit
}

type harness struct {
	h       http.Handler
	saver   *fakeSaver
	limiter *fakeLimiter
	logs    *bytes.Buffer
}

func newHarness(t *testing.T, opts Options) *harness {
	t.Helper()
	hs := &harness{saver: &fakeSaver{}, limiter: &fakeLimiter{limit: 100}, logs: &bytes.Buffer{}}
	site := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>survey</title>")}}
	log := slog.New(slog.NewTextHandler(hs.logs, nil))
	h, err := New(log, hs.saver, hs.limiter, site, opts)
	if err != nil {
		t.Fatal(err)
	}
	hs.h = h
	return hs
}

func (hs *harness) do(method, path, ctype, body string, hdr ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	hs.h.ServeHTTP(rec, req)
	return rec
}

func TestStatusCodes(t *testing.T) {
	const ctJSON = "application/json"
	cases := []struct {
		name, method, ctype, body string
		saveErr                   error
		limit                     int
		want, saved               int
		wantError                 string
	}{
		{"201 saved", "POST", ctJSON, validBody, nil, 100, 201, 1, ""},
		{"201 charset content type", "POST", "application/json; charset=utf-8", validBody, nil, 100, 201, 1, ""},
		{"201 honeypot stores nothing", "POST", ctJSON, strings.Replace(validBody, `"website":""`, `"website":"x"`, 1), nil, 100, 201, 0, ""},
		{"400 bad JSON", "POST", ctJSON, "{", nil, 100, 400, 0, "invalid JSON"},
		{"400 unknown field", "POST", ctJSON, `{"consent":true,"name":"x"}`, nil, 100, 400, 0, "invalid JSON"},
		{"400 body over 16 KB", "POST", ctJSON, `{"biggest_frustration":"` + strings.Repeat("a", maxBodyBytes) + `"}`, nil, 100, 400, 0, "invalid JSON"},
		{"405 GET", "GET", "", "", nil, 100, 405, 0, "use POST"},
		{"405 PUT", "PUT", ctJSON, validBody, nil, 100, 405, 0, "use POST"},
		{"405 OPTIONS without CORS", "OPTIONS", "", "", nil, 100, 405, 0, "use POST"},
		{"415 not JSON", "POST", "text/plain", validBody, nil, 100, 415, 0, "send JSON"},
		{"415 no content type", "POST", "", validBody, nil, 100, 415, 0, "send JSON"},
		{"422 invalid answers", "POST", ctJSON, `{"consent":false}`, nil, 100, 422, 0, "invalid answers"},
		{"429 rate limited", "POST", ctJSON, validBody, nil, 0, 429, 0, "too many submissions, try again later"},
		{"502 storage failure", "POST", ctJSON, validBody, errors.New("boom"), 100, 502, 1, "could not save answers"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hs := newHarness(t, Options{})
			hs.saver.err, hs.limiter.limit = c.saveErr, c.limit
			rec := hs.do(c.method, "/api/responses", c.ctype, c.body)
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.want, rec.Body)
			}
			if len(hs.saver.rows) != c.saved {
				t.Fatalf("saved %d rows, want %d", len(hs.saver.rows), c.saved)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type %q", ct)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body not JSON: %s", rec.Body)
			}
			if c.wantError != "" && body["error"] != c.wantError {
				t.Fatalf("error %v, want %q", body["error"], c.wantError)
			}
			if c.want == 201 && body["status"] != "ok" {
				t.Fatalf("body %s", rec.Body)
			}
			if c.want == 405 && rec.Header().Get("Allow") != "POST" {
				t.Fatalf("Allow %q", rec.Header().Get("Allow"))
			}
		})
	}
}

func TestChecksRunBeforeRateLimit(t *testing.T) {
	hs := newHarness(t, Options{})
	hs.do("GET", "/api/responses", "", "")
	hs.do("POST", "/api/responses", "text/plain", validBody)
	if len(hs.limiter.seen) != 0 {
		t.Fatalf("wrong method / content type counted against the limit: %v", hs.limiter.seen)
	}
}

func TestInvalidAnswersBody(t *testing.T) {
	hs := newHarness(t, Options{})
	rec := hs.do("POST", "/api/responses", "application/json", strings.Replace(validBody, `"likert_structure":4`, `"likert_structure":9`, 1))
	var got struct {
		Error  string            `json:"error"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "invalid answers" || len(got.Errors) != 1 || got.Errors["likert_structure"] != "must be from 1 to 5" {
		t.Fatalf("body %s", rec.Body)
	}
}

func TestSavedRow(t *testing.T) {
	hs := newHarness(t, Options{})
	hs.do("POST", "/api/responses", "application/json", validBody)
	if len(hs.saver.rows) != 1 {
		t.Fatal("nothing saved")
	}
	got, _ := json.Marshal(hs.saver.rows[0])
	want := `{"consent":true,"roles":["student","other"],"role_other":"Lecturer","oss_experience":"beginner","barriers":["no_guidance","low_confidence"],"likert_structure":4,"likert_env_setup":5,"likert_docs_outdated":3,"likert_tracing_comfortable":2,"time_to_confidence":"1_3h","exploration_methods":["readme_docs","llm_tools"],"diagram_frequency":"rarely","feature_zero_install":5,"feature_visual_map":4,"feature_ai_assistant":3,"feature_privacy":5,"likelihood_to_use":4,"biggest_frustration":"Setup","how_overcome":null,"completion_seconds":321,"survey_version":"v1"}`
	if string(got) != want {
		t.Fatalf("row\n got %s\nwant %s", got, want)
	}
}

func TestSecurityHeaders(t *testing.T) {
	hs := newHarness(t, Options{})
	for _, path := range []string{"/", "/healthz", "/api/responses"} {
		rec := hs.do("GET", path, "", "")
		for k, want := range map[string]string{
			"Content-Security-Policy":    "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
			"X-Content-Type-Options":     "nosniff",
			"Referrer-Policy":            "no-referrer",
			"Permissions-Policy":         "camera=(), microphone=(), geolocation=(), interest-cohort=()",
			"Cross-Origin-Opener-Policy": "same-origin",
		} {
			if got := rec.Header().Get(k); got != want {
				t.Errorf("%s %s = %q", path, k, got)
			}
		}
	}
}

func TestStaticAndHealth(t *testing.T) {
	hs := newHarness(t, Options{})
	if rec := hs.do("GET", "/", "", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "survey") {
		t.Fatalf("/ %d %s", rec.Code, rec.Body)
	}
	if rec := hs.do("GET", "/missing.js", "", ""); rec.Code != 404 {
		t.Fatalf("missing file %d", rec.Code)
	}
	rec := hs.do("GET", "/healthz", "", "")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("/healthz %d %s", rec.Code, rec.Body)
	}
}

func TestCORS(t *testing.T) {
	t.Run("off by default", func(t *testing.T) {
		hs := newHarness(t, Options{})
		rec := hs.do("POST", "/api/responses", "application/json", validBody)
		if rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Header().Get("Vary") != "" {
			t.Fatalf("CORS headers without ALLOWED_ORIGIN: %v", rec.Header())
		}
	})
	t.Run("on when configured", func(t *testing.T) {
		hs := newHarness(t, Options{AllowedOrigin: "https://a.example"})
		rec := hs.do("OPTIONS", "/api/responses", "", "")
		if rec.Code != 204 ||
			rec.Header().Get("Access-Control-Allow-Origin") != "https://a.example" ||
			rec.Header().Get("Access-Control-Allow-Methods") != "POST" ||
			rec.Header().Get("Access-Control-Allow-Headers") != "Content-Type" ||
			rec.Header().Get("Vary") != "Origin" {
			t.Fatalf("preflight %d %v", rec.Code, rec.Header())
		}
		for _, c := range []struct{ method, ctype, body string }{
			{"POST", "application/json", validBody},
			{"POST", "text/plain", ""},
			{"GET", "", ""},
		} {
			rec := hs.do(c.method, "/api/responses", c.ctype, c.body)
			if rec.Header().Get("Access-Control-Allow-Origin") != "https://a.example" {
				t.Fatalf("%s %s: no CORS header", c.method, c.ctype)
			}
		}
		if hs.do("GET", "/", "", "").Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("CORS header on the static site")
		}
	})
}

func TestClientIP(t *testing.T) {
	for _, c := range []struct {
		trust bool
		xff   string
		want  string
	}{
		{false, "", "192.0.2.1"},
		{false, "203.0.113.9", "192.0.2.1"},
		{true, "", "192.0.2.1"},
		{true, " 203.0.113.9 , 10.0.0.1", "203.0.113.9"},
	} {
		hs := newHarness(t, Options{TrustProxy: c.trust})
		hs.do("POST", "/api/responses", "application/json", validBody, "X-Forwarded-For", c.xff)
		if hs.limiter.seen[c.want] != 1 || len(hs.limiter.seen) != 1 {
			t.Errorf("trust=%v xff=%q: limiter keys %v, want %s", c.trust, c.xff, hs.limiter.seen, c.want)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "no-port"
	if got := clientIP(r, false); got != "no-port" {
		t.Fatalf("got %q", got)
	}
}

func TestLogging(t *testing.T) {
	hs := newHarness(t, Options{})
	hs.saver.err = errors.New("supabase returned 400: 23514 check")
	hs.do("POST", "/api/responses", "application/json", strings.Replace(validBody, "Setup", "MY-SECRET-ANSWER", 1))
	logs := hs.logs.String()
	for _, want := range []string{"insert failed", "method=POST", "path=/api/responses", "status=502", "duration="} {
		if !strings.Contains(logs, want) {
			t.Errorf("log missing %q:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "MY-SECRET-ANSWER") || strings.Contains(logs, "Lecturer") {
		t.Fatalf("answers logged:\n%s", logs)
	}

	hs = newHarness(t, Options{})
	hs.do("GET", "/", "", "")
	if !strings.Contains(hs.logs.String(), "status=200") {
		t.Fatalf("implicit 200 not logged:\n%s", hs.logs)
	}
}

func TestNewRejectsMissingDeps(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	site := fstest.MapFS{}
	saver, lim := &fakeSaver{}, &fakeLimiter{}
	for name, call := range map[string]func() (http.Handler, error){
		"logger":  func() (http.Handler, error) { return New(nil, saver, lim, site, Options{}) },
		"saver":   func() (http.Handler, error) { return New(log, nil, lim, site, Options{}) },
		"limiter": func() (http.Handler, error) { return New(log, saver, nil, site, Options{}) },
		"site":    func() (http.Handler, error) { return New(log, saver, lim, nil, Options{}) },
	} {
		if h, err := call(); err == nil || h != nil {
			t.Errorf("nil %s accepted", name)
		}
	}
}

func TestWriteJSONEncodeError(t *testing.T) {
	var logs bytes.Buffer
	a := &api{log: slog.New(slog.NewTextHandler(&logs, nil))}
	a.writeJSON(httptest.NewRecorder(), http.StatusOK, map[string]any{"bad": make(chan int)})
	if !strings.Contains(logs.String(), "write response") {
		t.Fatalf("encode error not logged: %s", logs.String())
	}
}

func TestStatusRecorderUnwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec}
	if sr.Unwrap() != rec {
		t.Fatal("Unwrap lost the writer")
	}
}
