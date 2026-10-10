// Command survey serves the OSS-AGENT01 onboarding survey and stores answers
// in Supabase. It uses only the Go standard library.
//
// The Supabase secret key never leaves this server: the browser talks only to
// this service, and the database table has row-level security switched on with
// no public policies, so the key is the only way in.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

//go:embed web
var webFiles embed.FS

// ---------- Configuration ----------

type config struct {
	Port          string
	SupabaseURL   string
	SupabaseKey   string
	Table         string
	AllowedOrigin string // set only if the page is hosted on a different origin
	TrustProxy    bool   // read client IP from X-Forwarded-For (behind a proxy)
	RateLimit     int    // submissions per IP per window
	RateWindow    time.Duration
}

func loadConfig() (config, error) {
	c := config{
		Port:          envOr("PORT", "8080"),
		SupabaseURL:   strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseKey:   os.Getenv("SUPABASE_SECRET_KEY"),
		Table:         envOr("SUPABASE_TABLE", "survey_responses"),
		AllowedOrigin: os.Getenv("ALLOWED_ORIGIN"),
		TrustProxy:    os.Getenv("TRUST_PROXY") == "true",
		RateLimit:     5,
		RateWindow:    10 * time.Minute,
	}
	if v := os.Getenv("RATE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return c, fmt.Errorf("RATE_LIMIT must be a positive number")
		}
		c.RateLimit = n
	}
	if c.SupabaseURL == "" || c.SupabaseKey == "" {
		return c, errors.New("SUPABASE_URL and SUPABASE_SECRET_KEY must be set")
	}
	return c, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ---------- Survey answers ----------

// submission is what the browser sends. Unknown fields are rejected.
type submission struct {
	Consent                  bool     `json:"consent"`
	Roles                    []string `json:"roles"`
	RoleOther                string   `json:"role_other"`
	OSSExperience            string   `json:"oss_experience"`
	Barriers                 []string `json:"barriers"`
	LikertStructure          int      `json:"likert_structure"`
	LikertEnvSetup           int      `json:"likert_env_setup"`
	LikertDocsOutdated       int      `json:"likert_docs_outdated"`
	LikertTracingComfortable int      `json:"likert_tracing_comfortable"`
	TimeToConfidence         string   `json:"time_to_confidence"`
	ExplorationMethods       []string `json:"exploration_methods"`
	DiagramFrequency         string   `json:"diagram_frequency"`
	FeatureZeroInstall       int      `json:"feature_zero_install"`
	FeatureVisualMap         int      `json:"feature_visual_map"`
	FeatureAIAssistant       int      `json:"feature_ai_assistant"`
	FeaturePrivacy           int      `json:"feature_privacy"`
	LikelihoodToUse          int      `json:"likelihood_to_use"`
	BiggestFrustration       string   `json:"biggest_frustration"`
	HowOvercome              string   `json:"how_overcome"`
	CompletionSeconds        int      `json:"completion_seconds"`
	Website                  string   `json:"website"` // honeypot, never stored
}

// row is what gets written to Supabase.
type row struct {
	Consent                  bool     `json:"consent"`
	Roles                    []string `json:"roles"`
	RoleOther                *string  `json:"role_other"`
	OSSExperience            string   `json:"oss_experience"`
	Barriers                 []string `json:"barriers"`
	LikertStructure          int      `json:"likert_structure"`
	LikertEnvSetup           int      `json:"likert_env_setup"`
	LikertDocsOutdated       int      `json:"likert_docs_outdated"`
	LikertTracingComfortable int      `json:"likert_tracing_comfortable"`
	TimeToConfidence         string   `json:"time_to_confidence"`
	ExplorationMethods       []string `json:"exploration_methods"`
	DiagramFrequency         string   `json:"diagram_frequency"`
	FeatureZeroInstall       int      `json:"feature_zero_install"`
	FeatureVisualMap         int      `json:"feature_visual_map"`
	FeatureAIAssistant       int      `json:"feature_ai_assistant"`
	FeaturePrivacy           int      `json:"feature_privacy"`
	LikelihoodToUse          int      `json:"likelihood_to_use"`
	BiggestFrustration       *string  `json:"biggest_frustration"`
	HowOvercome              *string  `json:"how_overcome"`
	CompletionSeconds        int      `json:"completion_seconds"`
	SurveyVersion            string   `json:"survey_version"`
}

const surveyVersion = "v1"

var (
	roleValues      = set("student", "junior", "experienced", "maintainer", "other")
	experienceVals  = set("beginner", "intermediate", "advanced")
	barrierValues   = set("never_heard", "no_need", "no_guidance", "low_confidence", "no_benefit", "low_motivation", "already_contribute")
	timeValues      = set("lt_1h", "1_3h", "4_8h", "gt_8h", "give_up")
	methodValues    = set("readme_docs", "ide_navigation", "mental_diagrams", "tests_debugging", "ask_community", "visual_tools", "llm_tools")
	diagramValues   = set("always", "frequently", "rarely", "never")
	maxOpenText     = 2000
	maxOtherText    = 200
	maxMethodsChose = 3
)

func set(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// validate checks every answer and returns field errors keyed by JSON name.
func (s *submission) validate() map[string]string {
	errs := map[string]string{}

	if !s.Consent {
		errs["consent"] = "consent is required"
	}
	if msg := checkList(s.Roles, roleValues, 1, len(roleValues)); msg != "" {
		errs["roles"] = msg
	}
	s.RoleOther = strings.TrimSpace(s.RoleOther)
	if contains(s.Roles, "other") && s.RoleOther == "" {
		errs["role_other"] = "describe the other role"
	}
	if utf8.RuneCountInString(s.RoleOther) > maxOtherText {
		errs["role_other"] = fmt.Sprintf("must be %d characters or fewer", maxOtherText)
	}
	if !experienceVals[s.OSSExperience] {
		errs["oss_experience"] = "choose one answer"
	}
	if msg := checkList(s.Barriers, barrierValues, 1, len(barrierValues)); msg != "" {
		errs["barriers"] = msg
	} else if contains(s.Barriers, "already_contribute") && len(s.Barriers) > 1 {
		errs["barriers"] = "'already contribute' can't be combined with other reasons"
	}
	for name, v := range map[string]int{
		"likert_structure":           s.LikertStructure,
		"likert_env_setup":           s.LikertEnvSetup,
		"likert_docs_outdated":       s.LikertDocsOutdated,
		"likert_tracing_comfortable": s.LikertTracingComfortable,
		"feature_zero_install":       s.FeatureZeroInstall,
		"feature_visual_map":         s.FeatureVisualMap,
		"feature_ai_assistant":       s.FeatureAIAssistant,
		"feature_privacy":            s.FeaturePrivacy,
		"likelihood_to_use":          s.LikelihoodToUse,
	} {
		if v < 1 || v > 5 {
			errs[name] = "must be from 1 to 5"
		}
	}
	if !timeValues[s.TimeToConfidence] {
		errs["time_to_confidence"] = "choose one answer"
	}
	if msg := checkList(s.ExplorationMethods, methodValues, 1, maxMethodsChose); msg != "" {
		errs["exploration_methods"] = msg
	}
	if !diagramValues[s.DiagramFrequency] {
		errs["diagram_frequency"] = "choose one answer"
	}
	s.BiggestFrustration = strings.TrimSpace(s.BiggestFrustration)
	s.HowOvercome = strings.TrimSpace(s.HowOvercome)
	if utf8.RuneCountInString(s.BiggestFrustration) > maxOpenText {
		errs["biggest_frustration"] = fmt.Sprintf("must be %d characters or fewer", maxOpenText)
	}
	if utf8.RuneCountInString(s.HowOvercome) > maxOpenText {
		errs["how_overcome"] = fmt.Sprintf("must be %d characters or fewer", maxOpenText)
	}
	if s.CompletionSeconds < 0 || s.CompletionSeconds > 86400 {
		s.CompletionSeconds = 0 // unknown; keep the answers
	}
	return errs
}

func checkList(vals []string, allowed map[string]bool, min, max int) string {
	if len(vals) < min {
		return "select at least one answer"
	}
	if len(vals) > max {
		return fmt.Sprintf("select no more than %d", max)
	}
	seen := map[string]bool{}
	for _, v := range vals {
		if !allowed[v] {
			return "contains an unknown answer"
		}
		if seen[v] {
			return "contains a duplicate answer"
		}
		seen[v] = true
	}
	return ""
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *submission) toRow() row {
	r := row{
		Consent:                  true,
		Roles:                    s.Roles,
		OSSExperience:            s.OSSExperience,
		Barriers:                 s.Barriers,
		LikertStructure:          s.LikertStructure,
		LikertEnvSetup:           s.LikertEnvSetup,
		LikertDocsOutdated:       s.LikertDocsOutdated,
		LikertTracingComfortable: s.LikertTracingComfortable,
		TimeToConfidence:         s.TimeToConfidence,
		ExplorationMethods:       s.ExplorationMethods,
		DiagramFrequency:         s.DiagramFrequency,
		FeatureZeroInstall:       s.FeatureZeroInstall,
		FeatureVisualMap:         s.FeatureVisualMap,
		FeatureAIAssistant:       s.FeatureAIAssistant,
		FeaturePrivacy:           s.FeaturePrivacy,
		LikelihoodToUse:          s.LikelihoodToUse,
		BiggestFrustration:       optional(s.BiggestFrustration),
		HowOvercome:              optional(s.HowOvercome),
		CompletionSeconds:        s.CompletionSeconds,
		SurveyVersion:            surveyVersion,
	}
	if contains(s.Roles, "other") {
		r.RoleOther = optional(s.RoleOther)
	}
	return r
}

// ---------- Supabase client ----------

type store struct {
	url, key, table string
	client          *http.Client
}

// insert writes one row through Supabase's REST API (PostgREST).
func (st *store) insert(ctx context.Context, r row) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/rest/v1/%s", st.url, st.table)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")
	req.Header.Set("apikey", st.key)
	// Legacy service_role keys are JWTs and also go in Authorization.
	// New sb_secret_ keys only need the apikey header.
	if strings.HasPrefix(st.key, "eyJ") {
		req.Header.Set("Authorization", "Bearer "+st.key)
	}
	res, err := st.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("supabase returned %d: %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

// ---------- Rate limiting ----------

type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*bucket
}

type bucket struct {
	count int
	reset time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	l := &limiter{limit: limit, window: window, hits: map[string]*bucket{}}
	go func() {
		for range time.Tick(window) {
			l.mu.Lock()
			now := time.Now()
			for ip, b := range l.hits {
				if now.After(b.reset) {
					delete(l.hits, ip)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.hits[ip]
	if !ok || now.After(b.reset) {
		l.hits[ip] = &bucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------- HTTP ----------

type server struct {
	cfg     config
	store   *store
	limiter *limiter
	log     *slog.Logger
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AllowedOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowedOrigin)
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "send JSON"})
		return
	}
	if !s.limiter.allow(clientIP(r, s.cfg.TrustProxy)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many submissions, try again later"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var sub submission
	if err := dec.Decode(&sub); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Bots fill the hidden field. Pretend success, store nothing.
	if sub.Website != "" {
		s.log.Info("honeypot triggered")
		writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
		return
	}

	if errs := sub.validate(); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "invalid answers", "errors": errs})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.store.insert(ctx, sub.toRow()); err != nil {
		s.log.Error("insert failed", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not save answers"})
		return
	}
	s.log.Info("response saved")
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// securityHeaders locks the page down to its own origin.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *server) routes() http.Handler {
	site, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/responses", s.handleSubmit)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("/", http.FileServer(http.FS(site)))
	return securityHeaders(mux)
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := loadConfig()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	s := &server{
		cfg:     cfg,
		store:   &store{url: cfg.SupabaseURL, key: cfg.SupabaseKey, table: cfg.Table, client: &http.Client{Timeout: 10 * time.Second}},
		limiter: newLimiter(cfg.RateLimit, cfg.RateWindow),
		log:     log,
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("survey server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("server stopped")
}
