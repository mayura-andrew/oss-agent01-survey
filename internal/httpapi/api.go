// Package httpapi serves the survey page and the submission API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"oss-agent01-survey/internal/survey"
)

const (
	// maxBodyBytes caps the size of a submission request body.
	maxBodyBytes = 16 << 10
	// saveTimeout bounds how long one submission may wait on storage.
	saveTimeout = 10 * time.Second
)

// ResponseSaver stores one validated survey response.
type ResponseSaver interface {
	Save(ctx context.Context, r survey.Row) error
}

// Limiter decides whether a client, identified by key, may submit again.
type Limiter interface {
	Allow(key string) bool
}

// Options are the optional behaviours of the API.
type Options struct {
	// AllowedOrigin enables CORS on the API for this one origin. Empty disables CORS.
	AllowedOrigin string
	// TrustProxy reads the client IP from X-Forwarded-For for rate limiting.
	TrustProxy bool
}

type api struct {
	log     *slog.Logger
	saver   ResponseSaver
	limiter Limiter
	opts    Options
}

// New returns the full HTTP handler: the submission API, a health check, and
// the static site, wrapped in request logging and security headers.
func New(log *slog.Logger, saver ResponseSaver, limiter Limiter, site fs.FS, opts Options) (http.Handler, error) {
	switch {
	case log == nil:
		return nil, errors.New("httpapi: nil logger")
	case saver == nil:
		return nil, errors.New("httpapi: nil ResponseSaver")
	case limiter == nil:
		return nil, errors.New("httpapi: nil Limiter")
	case site == nil:
		return nil, errors.New("httpapi: nil site filesystem")
	}
	a := &api{log: log, saver: saver, limiter: limiter, opts: opts}

	mux := http.NewServeMux()
	mux.Handle("POST /api/responses", a.cors(a.requireJSON(a.rateLimit(http.HandlerFunc(a.submit)))))
	if opts.AllowedOrigin != "" {
		mux.Handle("OPTIONS /api/responses", a.cors(http.HandlerFunc(preflight)))
	}
	mux.Handle("/api/responses", a.cors(http.HandlerFunc(a.methodNotAllowed)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		a.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("/", http.FileServer(http.FS(site)))
	return a.logRequests(securityHeaders(mux)), nil
}

// submit decodes, validates and stores one survey response.
func (a *api) submit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var sub survey.Submission
	if err := dec.Decode(&sub); err != nil {
		a.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Bots fill the hidden field. Pretend success, store nothing.
	if sub.IsBot() {
		a.log.Info("honeypot triggered")
		a.writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
		return
	}

	if errs := sub.Validate(); len(errs) > 0 {
		a.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "invalid answers", "errors": errs})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), saveTimeout)
	defer cancel()
	if err := a.saver.Save(ctx, sub.ToRow()); err != nil {
		a.log.Error("insert failed", "err", err)
		a.writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not save answers"})
		return
	}
	a.log.Info("response saved")
	a.writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (a *api) methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", http.MethodPost)
	a.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
}

func preflight(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", http.MethodPost)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		a.log.Warn("write response", "err", err)
	}
}
