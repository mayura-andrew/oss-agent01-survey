// Package survey defines the OSS-AGENT01 onboarding survey answers, the rules
// they must satisfy, and the row shape written to storage. It does no I/O.
package survey

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Limits and fixed values for a submission.
const (
	// Version is stored with every row so later survey revisions can be told apart.
	Version = "v1"
	// MaxOpenText is the longest answer, in characters, for an open question.
	MaxOpenText = 2000
	// MaxOtherText is the longest "other role" description, in characters.
	MaxOtherText = 200
	// MaxMethods is how many exploration methods a participant may choose.
	MaxMethods = 3
	// MaxCompletionSeconds is the longest plausible completion time (one day).
	MaxCompletionSeconds = 86400
)

// Allowed answers for each choice question. Read-only after package load.
var (
	roleValues       = setOf("student", "junior", "experienced", "maintainer", "other")
	experienceValues = setOf("beginner", "intermediate", "advanced")
	barrierValues    = setOf("never_heard", "no_need", "no_guidance", "low_confidence", "no_benefit", "low_motivation", "already_contribute")
	timeValues       = setOf("lt_1h", "1_3h", "4_8h", "gt_8h", "give_up")
	methodValues     = setOf("readme_docs", "ide_navigation", "mental_diagrams", "tests_debugging", "ask_community", "visual_tools", "llm_tools")
	diagramValues    = setOf("always", "frequently", "rarely", "never")
)

// Submission is what the browser sends.
type Submission struct {
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

// Row is one stored response. Optional text is nil when left blank.
type Row struct {
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

// IsBot reports whether the hidden honeypot field was filled in.
func (s *Submission) IsBot() bool { return s.Website != "" }

// Validate normalises s in place (trims free text, resets an implausible
// completion time to 0) and returns field errors keyed by JSON name. An empty
// map means the submission is valid.
func (s *Submission) Validate() map[string]string {
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
	if utf8.RuneCountInString(s.RoleOther) > MaxOtherText {
		errs["role_other"] = fmt.Sprintf("must be %d characters or fewer", MaxOtherText)
	}
	if !experienceValues[s.OSSExperience] {
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
	if msg := checkList(s.ExplorationMethods, methodValues, 1, MaxMethods); msg != "" {
		errs["exploration_methods"] = msg
	}
	if !diagramValues[s.DiagramFrequency] {
		errs["diagram_frequency"] = "choose one answer"
	}
	s.BiggestFrustration = strings.TrimSpace(s.BiggestFrustration)
	s.HowOvercome = strings.TrimSpace(s.HowOvercome)
	if s.BiggestFrustration == "" {
		errs["biggest_frustration"] = "answer this question"
	}
	if utf8.RuneCountInString(s.BiggestFrustration) > MaxOpenText {
		errs["biggest_frustration"] = fmt.Sprintf("must be %d characters or fewer", MaxOpenText)
	}
	if utf8.RuneCountInString(s.HowOvercome) > MaxOpenText {
		errs["how_overcome"] = fmt.Sprintf("must be %d characters or fewer", MaxOpenText)
	}
	if s.CompletionSeconds < 0 || s.CompletionSeconds > MaxCompletionSeconds {
		s.CompletionSeconds = 0 // unknown; keep the answers
	}
	return errs
}

// ToRow converts a validated submission into the row to store. The honeypot
// is dropped, and role_other is kept only when "other" was chosen.
func (s *Submission) ToRow() Row {
	r := Row{
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
		SurveyVersion:            Version,
	}
	if contains(s.Roles, "other") {
		r.RoleOther = optional(s.RoleOther)
	}
	return r
}

func setOf(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

func checkList(vals []string, allowed map[string]bool, minN, maxN int) string {
	if len(vals) < minN {
		return "select at least one answer"
	}
	if len(vals) > maxN {
		return fmt.Sprintf("select no more than %d", maxN)
	}
	seen := make(map[string]bool, len(vals))
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
