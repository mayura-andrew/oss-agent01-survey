-- OSS-AGENT01 onboarding survey: responses table.
-- Run this once in the Supabase dashboard: SQL Editor -> New query -> paste -> Run.
--
-- One row per participant, one column per question, so the data exports
-- straight to CSV for analysis. The checks mirror the Go API's validation,
-- so bad data can't get in even if the API is bypassed.

create table if not exists public.survey_responses (
  id                          uuid primary key default gen_random_uuid(),
  created_at                  timestamptz not null default now(),
  survey_version              text not null default 'v1',

  consent                     boolean not null check (consent),

  -- Section 1: about you
  roles                       text[] not null
                              check (cardinality(roles) between 1 and 5
                                 and roles <@ array['student','junior','experienced','maintainer','other']),
  role_other                  text check (char_length(role_other) <= 200),
  oss_experience              text not null
                              check (oss_experience in ('beginner','intermediate','advanced')),
  barriers                    text[] not null
                              check (cardinality(barriers) between 1 and 7
                                 and barriers <@ array['never_heard','no_need','no_guidance','low_confidence','no_benefit','low_motivation','already_contribute']),

  -- Section 2: pain points (1 = strongly disagree, 5 = strongly agree)
  likert_structure            smallint not null check (likert_structure between 1 and 5),
  likert_env_setup            smallint not null check (likert_env_setup between 1 and 5),
  likert_docs_outdated        smallint not null check (likert_docs_outdated between 1 and 5),
  likert_tracing_comfortable  smallint not null check (likert_tracing_comfortable between 1 and 5),
  time_to_confidence          text not null
                              check (time_to_confidence in ('lt_1h','1_3h','4_8h','gt_8h','give_up')),

  -- Section 3: current practice
  exploration_methods         text[] not null
                              check (cardinality(exploration_methods) between 1 and 3
                                 and exploration_methods <@ array['readme_docs','ide_navigation','mental_diagrams','tests_debugging','ask_community','visual_tools','llm_tools']),
  diagram_frequency           text not null
                              check (diagram_frequency in ('always','frequently','rarely','never')),

  -- Section 4: feature value (1 = not valuable, 5 = extremely valuable)
  feature_zero_install        smallint not null check (feature_zero_install between 1 and 5),
  feature_visual_map          smallint not null check (feature_visual_map between 1 and 5),
  feature_ai_assistant        smallint not null check (feature_ai_assistant between 1 and 5),
  feature_privacy             smallint not null check (feature_privacy between 1 and 5),
  likelihood_to_use           smallint not null check (likelihood_to_use between 1 and 5),
  biggest_frustration         text check (char_length(biggest_frustration) <= 2000),
  how_overcome                text check (char_length(how_overcome) <= 2000),

  -- Data quality: seconds from consent to submit (0 = unknown)
  completion_seconds          integer not null default 0 check (completion_seconds between 0 and 86400)
);

comment on table public.survey_responses is
  'OSS-AGENT01 codebase onboarding survey. Anonymous: no names, emails, or IP addresses are stored.';
comment on column public.survey_responses.likert_tracing_comfortable is
  'Q7 is positively worded ("comfortable"): reverse-code it (6 - value) before combining with Q4-Q6 into a difficulty score.';

-- Lock the table down. Row-level security on, and no policies for anon or
-- authenticated users, so the public anon key can neither read nor write.
-- Only the Go API (using the secret key) can insert.
alter table public.survey_responses enable row level security;
revoke all on public.survey_responses from anon, authenticated;
