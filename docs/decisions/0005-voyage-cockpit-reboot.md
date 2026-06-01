# 0005 — Voyage Cockpit Reboot

**Status:** Accepted (Sprint 027)
**Date:** 2026-05-31

## Context

Liveaboard has accumulated enough backend capability to be operationally useful, but the authenticated app still reads like a themed admin dashboard. Sprint 025's Triptych runtime evaluation helped expose directions, but keeping multiple palettes, layouts, and motion modes in the production app made the product feel undecided.

Sprint 026 Phase 1 already landed data foundations for leads, quotes, offline payments, crew, equipment, readiness, guest certifications, portal requests, and rate limiting. Sprint 027 uses those shipped rows where useful, but pauses public funnel, guest portal, and broad new UI expansion until the core authenticated experience is excellent.

## Decision

The authenticated `/admin` first screen becomes the **Voyage Cockpit**: a role-specific command surface for Org Admins and Cruise Directors. It combines voyage lanes, blockers, readiness, money, inventory, activity, and route commands into a single operating surface backed by `GET /api/admin/cockpit`.

Triptych is closed and superseded. Production no longer supports URL/localStorage design switching, multiple palettes, multiple shell layouts, or the floating switcher. Weak evaluation code is deleted instead of kept dev-only.

The cockpit is allowed to use a route-scoped immersive visual treatment. Auth pages and non-cockpit public surfaces can keep the older Sprint 011 background until redesigned; the cockpit does not treat that background as sacred.

## Consequences

- `themes.css` collapses the winning semantic token set into `:root`.
- `DesignModeProvider`, `TriptychSwitcher`, RailNav, CanvasNav, and TodayCanvas are deleted or folded into the cockpit.
- `ui_redesign_switcher` is removed from backend dev flags and frontend dev flag types.
- New cockpit UI code lives under `web/src/admin/cockpit/` and ships with rich Org Admin and Cruise Director fixtures.
- The deeper run-loop page migration is deferred to Sprint 028.

## Security

`GET /api/admin/cockpit` returns bounded projections only. Every query scopes by organization; Cruise Director data scopes to assigned trips. Cockpit commands are navigation shortcuts and do not bypass existing protected mutation endpoints.
