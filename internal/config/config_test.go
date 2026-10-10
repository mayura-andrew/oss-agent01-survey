package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{
		"SUPABASE_URL":        "https://x.supabase.co/",
		"SUPABASE_SECRET_KEY": "sb_secret_1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Port: "8080", SupabaseURL: "https://x.supabase.co", SupabaseKey: "sb_secret_1",
		Table: "survey_responses", RateLimit: 5, RateWindow: 10 * time.Minute, LogFormat: "text",
	}
	if c != want {
		t.Fatalf("got %+v\nwant %+v", c, want)
	}
}

func TestLoadAll(t *testing.T) {
	c, err := Load(env(map[string]string{
		"PORT": "9000", "SUPABASE_URL": "http://localhost:54321", "SUPABASE_SECRET_KEY": "k",
		"SUPABASE_TABLE": "t", "ALLOWED_ORIGIN": "https://a.example", "TRUST_PROXY": "true",
		"RATE_LIMIT": "20", "LOG_FORMAT": "json",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "9000" || c.Table != "t" || c.AllowedOrigin != "https://a.example" || !c.TrustProxy || c.RateLimit != 20 || c.LogFormat != "json" {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadTrustProxyOnlyTrue(t *testing.T) {
	c, _ := Load(env(map[string]string{"TRUST_PROXY": "1"}))
	if c.TrustProxy {
		t.Fatal("TRUST_PROXY=1 must not enable proxy trust")
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"nothing set", nil, []string{"SUPABASE_URL and SUPABASE_SECRET_KEY must be set"}},
		{"everything wrong", map[string]string{
			"SUPABASE_URL": "x.supabase.co", "RATE_LIMIT": "0", "LOG_FORMAT": "xml",
		}, []string{
			"RATE_LIMIT must be a positive number",
			"SUPABASE_URL and SUPABASE_SECRET_KEY must be set",
			"SUPABASE_URL must be an absolute http(s) URL",
			"LOG_FORMAT must be text or json",
		}},
		{"rate not a number", map[string]string{"SUPABASE_URL": "https://x", "SUPABASE_SECRET_KEY": "k", "RATE_LIMIT": "ten"}, []string{"RATE_LIMIT"}},
		{"ftp url", map[string]string{"SUPABASE_URL": "ftp://x", "SUPABASE_SECRET_KEY": "k"}, []string{"absolute http(s) URL"}},
		{"unparsable url", map[string]string{"SUPABASE_URL": "http://[::1", "SUPABASE_SECRET_KEY": "k"}, []string{"absolute http(s) URL"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(env(c.env))
			if err == nil {
				t.Fatal("want error")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q missing %q", err, w)
				}
			}
		})
	}
}
