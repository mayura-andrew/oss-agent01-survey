# OSS-AGENT01 onboarding survey

An online survey for the OSS-AGENT01 final-year project. It collects anonymous answers about how developers and students get started with open-source codebases, and stores them in Supabase through a small Go service.

```
browser (survey page)  ──POST /api/responses──▶  Go service  ──REST──▶  Supabase (Postgres)
                       ◀── page, CSS, JS, fonts ─┘  (secret key stays here)
```

The browser never talks to Supabase directly, so the Supabase secret key is never exposed. The Go service serves the page itself, so the whole survey runs on one address.

## What's in this folder

| Path | What it is |
| --- | --- |
| `cmd/survey/` | The program's entry point: reads settings, wires the parts together, and shuts down gracefully. Standard library only, no dependencies. |
| `internal/survey/` | The questions' allowed answers, validation rules, and the row that gets stored. |
| `internal/httpapi/` | HTTP routes, the submit handler, and middleware (security headers, CORS, rate limiting, request logging). |
| `internal/store/` | The Supabase REST client. |
| `internal/ratelimit/` | The per-visitor submission limiter. |
| `internal/config/` | Reads and checks the environment settings. |
| `embed.go` | Builds `web/` into the binary. |
| `web/` | The survey page: `index.html`, `styles.css`, `app.js`, a self-hosted font, and the favicon. Built into the Go binary. |
| `supabase/schema.sql` | Creates the `survey_responses` table, one column per question, with checks and row-level security. |
| `Dockerfile` | Builds a small, non-root container. |
| `Makefile`, `.golangci.yml` | `make run`, `build`, `test`, `vet`, `fmt`, `lint`. |
| `.env.example` | The settings you need to fill in. |

## 1. Set up Supabase (about 5 minutes)

1. Create a project at [supabase.com](https://supabase.com).
2. Open **SQL Editor → New query**, paste all of `supabase/schema.sql`, and select **Run**.
3. Open **Project Settings → API** and copy:
   - the **Project URL** → `SUPABASE_URL`
   - the **secret key** (`sb_secret_…`), or the legacy **service_role** key → `SUPABASE_SECRET_KEY`

Use the secret key only on the server. The table has row-level security on and no public policies, so the public anon key can neither read nor write it.

## 2. Run it on your computer

You need Go 1.22 or newer.

```bash
cp .env.example .env          # then fill in the two Supabase values
set -a; source .env; set +a   # load the settings
go run ./cmd/survey           # or: make run
```

Run the tests with `make test` (`go test -race -cover ./...`) and the linters with `make lint` ([golangci-lint](https://golangci-lint.run) v2).

Open <http://localhost:8080>, complete the survey, then check **Table Editor → survey_responses** in Supabase to see the row.

## 3. Deploy

Any host that runs a container or a Go binary works: Render, Fly.io, Railway, Google Cloud Run, or a DigitalOcean droplet.

```bash
docker build -t oss-survey .
docker run -p 8080:8080 --env-file .env oss-survey
```

Behind a host's proxy or load balancer (Render, Fly, Nginx), set `TRUST_PROXY=true` so the per-person submission limit sees real visitor addresses. Serve the site over HTTPS; most hosts do this for you.

### Hosting the page on GitHub Pages

The page can also be served from GitHub Pages while the Go server runs elsewhere. [.github/workflows/pages.yml](.github/workflows/pages.yml) publishes `web/` on every push to `main` that touches it.

1. Set the repository variable `API_URL` to the full submit endpoint, e.g. `https://your-api.example.com/api/responses`.
2. On the API server, set `ALLOWED_ORIGIN=https://<user>.github.io` (scheme and host only, no path).
3. In **Settings → Pages**, set **Source** to **GitHub Actions**.

## Settings

| Variable | Required | Default | Meaning |
| --- | --- | --- | --- |
| `SUPABASE_URL` | Yes | – | Your project URL |
| `SUPABASE_SECRET_KEY` | Yes | – | Secret or service_role key, server-side only |
| `PORT` | No | `8080` | Port to listen on |
| `SUPABASE_TABLE` | No | `survey_responses` | Table name |
| `RATE_LIMIT` | No | `5` | Submissions per IP address per 10 minutes |
| `TRUST_PROXY` | No | `false` | Read visitor IP from `X-Forwarded-For` |
| `ALLOWED_ORIGIN` | No | – | Only if you host the page on a different domain from the API |
| `LOG_FORMAT` | No | `text` | `text` or `json` |

## How the data is protected

- **Anonymous by design.** No names, emails, or IP addresses are stored. The page loads nothing from other sites (the font is self-hosted), so there's no third-party tracking.
- **Validated twice.** The Go service checks every answer, and the database checks it again, so bad data can't get in even if the API is bypassed.
- **Spam control.** A hidden "honeypot" field catches simple bots, and each IP address can submit only a few times per 10 minutes.
- **Locked-down page.** A strict Content Security Policy only allows the page's own files.
- **Saved progress.** Answers are kept in the participant's own browser while they fill the survey in, so a refresh doesn't lose them. They are cleared after submitting.

## Getting the data out

In Supabase, open **Table Editor → survey_responses → Export → CSV**. Each row is one participant; list answers (roles, barriers, methods) are Postgres arrays.

Useful starting queries (SQL Editor):

```sql
-- How many responses, by experience level
select oss_experience, count(*) from survey_responses group by 1 order by 2 desc;

-- Average pain-point scores. Q7 is positively worded, so reverse it (6 - x).
select round(avg(likert_structure),2)          as structure_hard,
       round(avg(likert_env_setup),2)          as setup_discourages,
       round(avg(likert_docs_outdated),2)      as docs_outdated,
       round(avg(6 - likert_tracing_comfortable),2) as tracing_hard
from survey_responses;

-- Most-used exploration methods
select m as method, count(*) from survey_responses, unnest(exploration_methods) m group by 1 order by 2 desc;

-- Remove very fast responses (likely not read carefully) before analysis
select * from survey_responses where completion_seconds = 0 or completion_seconds >= 60;
```

## Design rationale: color

The color system follows the six sources the team provided.

| Decision | Why | Source |
| --- | --- | --- |
| Only three colors: a cool neutral (paper and white), dark navy ink, and one blue accent | A small palette keeps the visual hierarchy clear and stops colors competing for attention | [NN/g](https://www.nngroup.com/articles/color-enhance-design/); [Gusia, 2024](https://knowledgecenter.ubt-uni.net/cgi/viewcontent.cgi?article=4924&context=conference) |
| 60-30-10 split: about 60% neutral background, 30% ink and structure, 10% accent | Gives balance; the accent is kept for what should stand out most | [NN/g](https://www.nngroup.com/articles/color-enhance-design/); [IxDF](https://ixdf.org/literature/topics/color); [Medium guide](https://medium.com/@qamarjafari1717/the-ultimate-guide-to-color-and-color-psychology-in-ux-ui-2025-how-designers-use-color-to-5419f3283951) |
| The blue accent is used for one job only: actions, selections, progress, and focus | Consistent color meaning lets people learn the interface quickly | [NN/g](https://www.nngroup.com/articles/color-enhance-design/); [Gusia, 2024](https://knowledgecenter.ubt-uni.net/cgi/viewcontent.cgi?article=4924&context=conference) |
| Blue for the accent, with low overall stimulation (soft neutrals, close hues) | Blue is commonly read as calm and trustworthy, which suits a research survey where people should feel safe answering honestly; a calm, low-stimulation palette suits careful reading | [IxDF](https://ixdf.org/literature/topics/color); [UX Collective](https://uxdesign.cc/the-power-of-colour-in-ux-1748c16491f3); [Google Design](https://design.google/library/color-theory-ruxandra-duru) |
| One small warm amber touch (brand mark and thank-you ring) | A pinch of warm "completes" an otherwise cool palette without adding stimulation | [Google Design](https://design.google/library/color-theory-ruxandra-duru) |
| Red only for errors, and always with an icon and words | Avoids anxiety from overusing red, and never relies on color alone (WCAG 1.4.1) | [IxDF](https://ixdf.org/literature/topics/color); [Gusia, 2024](https://knowledgecenter.ubt-uni.net/cgi/viewcontent.cgi?article=4924&context=conference) |
| All text at least 4.5:1 contrast; borders and controls at least 3:1 | WCAG 2.1 AA for text (1.4.3) and for UI components (1.4.11) | [Gusia, 2024](https://knowledgecenter.ubt-uni.net/cgi/viewcontent.cgi?article=4924&context=conference); [IxDF](https://ixdf.org/literature/topics/color) |
| Test with real participants before the main launch | Color meaning varies by culture and person, so a small pilot should confirm the choices work for Sri Lankan students and developers | [NN/g](https://www.nngroup.com/articles/color-enhance-design/); [IxDF](https://ixdf.org/literature/topics/color); [UX Collective](https://uxdesign.cc/the-power-of-colour-in-ux-1748c16491f3) |

Measured contrast ratios: ink on white 16.0:1, muted text on white 6.5:1, accent on white 7.2:1 (also passes AAA), white on accent 7.2:1, error on white 6.5:1, control borders 3.7:1.

The typeface is **Atkinson Hyperlegible Next**, designed by the Braille Institute so that easily confused letters (I, l, 1; O, 0) stay distinct. That fits a survey that people should read carefully. It is licensed under the SIL Open Font License (`web/fonts/OFL.txt`).

## Changes made to the questionnaire

Review these with your supervisor before launch:

1. **Renumbered Q1–Q17.** The original had two questions numbered Q3.
2. **Added "Not applicable: I already contribute" to Q3.** Without it, experienced contributors had no honest answer. Choosing it clears the other reasons, and choosing a reason clears it.
3. **Q3 and Q1 are "select all that apply"**, and Q9 enforces "up to 3".
4. **Q16 and Q17 (open text) are optional**, to lower the effort of finishing.
5. **Added a consent step** (18 or older, voluntary, anonymous), which ethics review normally expects for research with people.

Two things worth considering:

- **Q10 has no middle option.** Always / Frequently / Rarely / Never skips "Sometimes", which pushes undecided people toward Frequently or Rarely.
- **Q7 is positively worded** ("comfortable"), while Q4–Q6 describe difficulty. Reverse-code Q7 when combining them (the analysis query above does this).

To change wording or options, edit the `SECTIONS` list in `web/app.js`. If you add or rename an option, also update the matching list in `internal/survey/survey.go` and the check in `supabase/schema.sql`, and bump `Version` in `internal/survey/survey.go`.
