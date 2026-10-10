package survey

import (
	"strconv"
	"strings"
	"testing"
)

func valid() Submission {
	return Submission{
		Consent:                  true,
		Roles:                    []string{"student"},
		OSSExperience:            "beginner",
		Barriers:                 []string{"no_guidance"},
		LikertStructure:          3,
		LikertEnvSetup:           3,
		LikertDocsOutdated:       3,
		LikertTracingComfortable: 3,
		TimeToConfidence:         "1_3h",
		ExplorationMethods:       []string{"readme_docs"},
		DiagramFrequency:         "rarely",
		FeatureZeroInstall:       3,
		FeatureVisualMap:         3,
		FeatureAIAssistant:       3,
		FeaturePrivacy:           3,
		LikelihoodToUse:          3,
		CompletionSeconds:        120,
	}
}

// sinhala returns n Sinhala letters: 3 bytes each in UTF-8, so a byte count
// would exceed the limits long before a character count does.
func sinhala(n int) string { return strings.Repeat("ස", n) }

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Submission)
		field   string // "" means the submission must be valid
		message string
	}{
		{"valid", func(*Submission) {}, "", ""},
		{"no consent", func(s *Submission) { s.Consent = false }, "consent", "consent is required"},

		{"roles empty", func(s *Submission) { s.Roles = nil }, "roles", "select at least one answer"},
		{"roles unknown", func(s *Submission) { s.Roles = []string{"wizard"} }, "roles", "contains an unknown answer"},
		{"roles duplicate", func(s *Submission) { s.Roles = []string{"student", "student"} }, "roles", "contains a duplicate answer"},
		{"roles all five", func(s *Submission) {
			s.Roles = []string{"student", "junior", "experienced", "maintainer", "other"}
			s.RoleOther = "x"
		}, "", ""},
		{"roles too many", func(s *Submission) {
			s.Roles = []string{"student", "junior", "experienced", "maintainer", "other", "student"}
			s.RoleOther = "x"
		}, "roles", "select no more than 5"},

		{"other without text", func(s *Submission) { s.Roles = []string{"other"}; s.RoleOther = "   " }, "role_other", "describe the other role"},
		{"other text at limit", func(s *Submission) { s.Roles = []string{"other"}; s.RoleOther = sinhala(MaxOtherText) }, "", ""},
		{"other text too long", func(s *Submission) { s.Roles = []string{"other"}; s.RoleOther = sinhala(MaxOtherText + 1) }, "role_other", "must be 200 characters or fewer"},

		{"experience missing", func(s *Submission) { s.OSSExperience = "" }, "oss_experience", "choose one answer"},
		{"experience unknown", func(s *Submission) { s.OSSExperience = "guru" }, "oss_experience", "choose one answer"},

		{"barriers empty", func(s *Submission) { s.Barriers = []string{} }, "barriers", "select at least one answer"},
		{"barriers unknown", func(s *Submission) { s.Barriers = []string{"lazy"} }, "barriers", "contains an unknown answer"},
		{"barriers duplicate", func(s *Submission) { s.Barriers = []string{"no_need", "no_need"} }, "barriers", "contains a duplicate answer"},
		{"already contribute alone", func(s *Submission) { s.Barriers = []string{"already_contribute"} }, "", ""},
		{"already contribute combined", func(s *Submission) { s.Barriers = []string{"already_contribute", "no_need"} }, "barriers", "'already contribute' can't be combined with other reasons"},
		{"barriers too many", func(s *Submission) {
			s.Barriers = []string{"never_heard", "no_need", "no_guidance", "low_confidence", "no_benefit", "low_motivation", "already_contribute", "no_need"}
		}, "barriers", "select no more than 7"},

		{"time missing", func(s *Submission) { s.TimeToConfidence = "" }, "time_to_confidence", "choose one answer"},
		{"time unknown", func(s *Submission) { s.TimeToConfidence = "forever" }, "time_to_confidence", "choose one answer"},

		{"methods empty", func(s *Submission) { s.ExplorationMethods = nil }, "exploration_methods", "select at least one answer"},
		{"methods at max", func(s *Submission) { s.ExplorationMethods = []string{"readme_docs", "llm_tools", "visual_tools"} }, "", ""},
		{"methods over max", func(s *Submission) {
			s.ExplorationMethods = []string{"readme_docs", "llm_tools", "visual_tools", "ask_community"}
		}, "exploration_methods", "select no more than 3"},
		{"methods unknown", func(s *Submission) { s.ExplorationMethods = []string{"guessing"} }, "exploration_methods", "contains an unknown answer"},
		{"methods duplicate", func(s *Submission) { s.ExplorationMethods = []string{"llm_tools", "llm_tools"} }, "exploration_methods", "contains a duplicate answer"},

		{"diagram missing", func(s *Submission) { s.DiagramFrequency = "" }, "diagram_frequency", "choose one answer"},
		{"diagram unknown", func(s *Submission) { s.DiagramFrequency = "sometimes" }, "diagram_frequency", "choose one answer"},

		{"frustration at limit", func(s *Submission) { s.BiggestFrustration = sinhala(MaxOpenText) }, "", ""},
		{"frustration too long", func(s *Submission) { s.BiggestFrustration = sinhala(MaxOpenText + 1) }, "biggest_frustration", "must be 2000 characters or fewer"},
		{"overcome at limit", func(s *Submission) { s.HowOvercome = sinhala(MaxOpenText) }, "", ""},
		{"overcome too long", func(s *Submission) { s.HowOvercome = sinhala(MaxOpenText + 1) }, "how_overcome", "must be 2000 characters or fewer"},
		{"trimmed text fits", func(s *Submission) { s.HowOvercome = "  " + sinhala(MaxOpenText) + "  " }, "", ""},
	}

	scales := map[string]func(*Submission, int){
		"likert_structure":           func(s *Submission, v int) { s.LikertStructure = v },
		"likert_env_setup":           func(s *Submission, v int) { s.LikertEnvSetup = v },
		"likert_docs_outdated":       func(s *Submission, v int) { s.LikertDocsOutdated = v },
		"likert_tracing_comfortable": func(s *Submission, v int) { s.LikertTracingComfortable = v },
		"feature_zero_install":       func(s *Submission, v int) { s.FeatureZeroInstall = v },
		"feature_visual_map":         func(s *Submission, v int) { s.FeatureVisualMap = v },
		"feature_ai_assistant":       func(s *Submission, v int) { s.FeatureAIAssistant = v },
		"feature_privacy":            func(s *Submission, v int) { s.FeaturePrivacy = v },
		"likelihood_to_use":          func(s *Submission, v int) { s.LikelihoodToUse = v },
	}
	for field, set := range scales {
		for _, v := range []int{0, 1, 5, 6} {
			want, msg := "", ""
			if v == 0 || v == 6 {
				want, msg = field, "must be from 1 to 5"
			}
			cases = append(cases, struct {
				name    string
				mutate  func(*Submission)
				field   string
				message string
			}{field + "=" + strconv.Itoa(v), func(s *Submission) { set(s, v) }, want, msg})
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := valid()
			c.mutate(&s)
			errs := s.Validate()
			if c.field == "" {
				if len(errs) != 0 {
					t.Fatalf("want valid, got %v", errs)
				}
				return
			}
			if len(errs) != 1 || errs[c.field] != c.message {
				t.Fatalf("want {%s: %q}, got %v", c.field, c.message, errs)
			}
		})
	}
}

func TestValidateReportsEveryField(t *testing.T) {
	var s Submission
	errs := s.Validate()
	for _, f := range []string{"consent", "roles", "oss_experience", "barriers", "likert_structure", "time_to_confidence", "exploration_methods", "diagram_frequency", "likelihood_to_use"} {
		if errs[f] == "" {
			t.Errorf("no error for %s", f)
		}
	}
}

func TestValidateCompletionSeconds(t *testing.T) {
	for in, want := range map[int]int{-1: 0, 0: 0, 1: 1, MaxCompletionSeconds: MaxCompletionSeconds, MaxCompletionSeconds + 1: 0} {
		s := valid()
		s.CompletionSeconds = in
		if errs := s.Validate(); len(errs) != 0 {
			t.Fatalf("%d: completion time must never reject a submission: %v", in, errs)
		}
		if s.CompletionSeconds != want {
			t.Errorf("CompletionSeconds %d became %d, want %d", in, s.CompletionSeconds, want)
		}
	}
}

func TestValidateTrims(t *testing.T) {
	s := valid()
	s.Roles = []string{"other"}
	s.RoleOther = "  tutor\n"
	s.BiggestFrustration = "\tbuild  "
	s.HowOvercome = "   "
	s.Validate()
	if s.RoleOther != "tutor" || s.BiggestFrustration != "build" || s.HowOvercome != "" {
		t.Fatalf("not trimmed: %q %q %q", s.RoleOther, s.BiggestFrustration, s.HowOvercome)
	}
}

func TestToRow(t *testing.T) {
	t.Run("blank optional text is null", func(t *testing.T) {
		s := valid()
		r := s.ToRow()
		if r.RoleOther != nil || r.BiggestFrustration != nil || r.HowOvercome != nil {
			t.Fatalf("want nil optional fields, got %v %v %v", r.RoleOther, r.BiggestFrustration, r.HowOvercome)
		}
		if !r.Consent || r.SurveyVersion != Version || r.CompletionSeconds != 120 {
			t.Fatalf("unexpected row %+v", r)
		}
	})
	t.Run("filled text is kept", func(t *testing.T) {
		s := valid()
		s.BiggestFrustration, s.HowOvercome = "a", "b"
		r := s.ToRow()
		if r.BiggestFrustration == nil || *r.BiggestFrustration != "a" || r.HowOvercome == nil || *r.HowOvercome != "b" {
			t.Fatalf("lost text: %+v", r)
		}
	})
	t.Run("role_other dropped unless other chosen", func(t *testing.T) {
		s := valid()
		s.RoleOther = "tutor"
		if r := s.ToRow(); r.RoleOther != nil {
			t.Fatalf("role_other kept without 'other': %q", *r.RoleOther)
		}
		s.Roles = []string{"student", "other"}
		if r := s.ToRow(); r.RoleOther == nil || *r.RoleOther != "tutor" {
			t.Fatalf("role_other lost with 'other': %v", r.RoleOther)
		}
	})
}

func TestIsBot(t *testing.T) {
	s := valid()
	if s.IsBot() {
		t.Fatal("empty honeypot flagged")
	}
	s.Website = "http://spam"
	if !s.IsBot() {
		t.Fatal("filled honeypot not flagged")
	}
}
