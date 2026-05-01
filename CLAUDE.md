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
