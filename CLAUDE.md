# Liveaboard

Multi-tenant SaaS platform for scuba diving liveaboard operators.

## Tech Stack

- **Frontend**: TypeScript, React (Vite or Next.js)
- **Backend**: Go (stdlib + minimal dependencies)
- **Database**: PostgreSQL (multi-tenant with row-level security)
## Key Requirements

- Online by default (offline support may be revisited later)
- Multi-tenant with strict data isolation between organizations
- Real-time ledger tracking of guest consumption during trips
- Designed for fast-paced crew environments

## Environment

- Local development only for now — everything runs on the developer's machine
- PostgreSQL runs locally
- Cloud deployment (AWS/GCP) will come later

## Sprint Ledger — Read First

Before doing any work in this repo, ground yourself in the sprint ledger.
It is the canonical record of what has shipped, what is in flight, and
what is planned next.

1. Run `go run docs/sprints/tracker.go list` to see every sprint and its
   status (`planned`, `in_progress`, `completed`, `skipped`).
2. Read every `docs/sprints/SPRINT-NNN.md` for the latest 2–3 sprints
   (and any sprint marked `in_progress`). Each sprint doc lists what was
   built, the architecture decisions, and the Definition of Done.
3. Skim `docs/product/organization-admin-user-stories.md` and
   `docs/product/personas.md` for the product backlog and persona
   boundaries — these drive sprint scoping.
4. The `docs/sprints/drafts/` directory holds the planning artifacts
   (intent → drafts → critique → merge notes) that produced each final
   sprint doc; consult them when you need decision context.

Do not assume the codebase from memory or from file names alone — the
sprint ledger is more authoritative and more current than any single
file's comments.

## Design System

Always read DESIGN.md before making any visual or UI decisions.
All font choices, colors, spacing, and aesthetic direction are defined there.
Do not deviate without explicit user approval.
In QA mode, flag any code that doesn't match DESIGN.md.

## Development Rules

- Code must compile before committing
- Tests must be written for all changes
- Tests must pass before committing
- `go vet` and linter must pass (backend)
- Code must be formatted (`gofmt` for Go, `prettier` for TypeScript)
- Each commit should be focused — no unrelated changes
