import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { useGlobalCommandShortcut } from "../components";

import { CommandPalette } from "./CommandPalette";
import { fetchCockpit, type CockpitResponse } from "./CockpitApi";
import { adminCockpitFixture, directorCockpitFixture } from "./fixtures";
import { ActionRail } from "./components/ActionRail";
import { ActivityStream } from "./components/ActivityStream";
import { MoneyStrip } from "./components/MoneyStrip";
import { ReadinessMatrix } from "./components/ReadinessMatrix";
import { SignalStack } from "./components/SignalStack";
import { VoyageMap } from "./components/VoyageMap";

import styles from "./Cockpit.module.css";

export function Cockpit() {
  const [data, setData] = useState<CockpitResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [fixtureRole, setFixtureRole] = useState(() =>
    fixtureRoleFromLocation(),
  );
  useGlobalCommandShortcut(() => setPaletteOpen(true));

  useEffect(() => {
    let cancelled = false;
    fetchCockpit()
      .then((next) => {
        if (!cancelled) {
          setData(next);
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled)
          setError(e instanceof Error ? e.message : "Failed to load cockpit.");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const view = useMemo(() => {
    if (fixtureRole) return fixtureForRole(fixtureRole);
    return data ?? fixtureForRole("admin");
  }, [data, fixtureRole]);
  const isFixture = fixtureRole != null || !data;
  const roleLabel =
    view.identity.role === "org_admin" ? "Org Admin" : "Cruise Director";

  return (
    <div className={styles.cockpit}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>{roleLabel} cockpit</span>
          <h1>Cockpit</h1>
        </div>
        <div className={styles.headerActions}>
          {isFixture && (
            <span className={error ? styles.errorBadge : styles.fixtureBadge}>
              {error ? "Fixture fallback" : "Demo pulse"}
            </span>
          )}
          <button type="button" onClick={() => setPaletteOpen(true)}>
            Command
          </button>
        </div>
      </header>

      {error && <div className={styles.errorLine}>{error}</div>}
      {!error && view.failed_sections?.length ? (
        <div className={styles.warningLine}>
          Degraded sections:{" "}
          {view.failed_sections.map(formatSection).join(", ")}
        </div>
      ) : null}
      {isFixture && (
        <div className={styles.fixtureTools}>
          <span>Fixture review</span>
          <button
            type="button"
            onClick={() => setFixture("admin", setFixtureRole)}
          >
            Org Admin
          </button>
          <button
            type="button"
            onClick={() => setFixture("director", setFixtureRole)}
          >
            Cruise Director
          </button>
          {data && (
            <button
              type="button"
              onClick={() => setFixture(null, setFixtureRole)}
            >
              Live data
            </button>
          )}
        </div>
      )}

      <MissionControl view={view} />
      <VoyageMap voyages={view.voyage_lanes} />
      <MoneyStrip money={view.money} />

      <div className={styles.signalRow}>
        <SignalStack signals={view.blockers} />
        <ReadinessMatrix
          voyages={view.voyage_lanes}
          inventory={view.inventory}
        />
        <ActionRail commands={view.commands} />
      </div>

      <ActivityStream activity={view.activity} />
      <CommandPalette
        commands={view.commands}
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
      />
    </div>
  );
}

function MissionControl({ view }: { view: CockpitResponse }) {
  // Each mission-control tile is a launcher into the vertical it
  // summarizes — click "Boats" to land on the fleet view, etc.
  const metrics: [string, string | number, string][] = view.admin_cockpit
    ? [
        ["Setup", `${view.admin_cockpit.setup_percent}%`, "/admin/onboarding"],
        ["Boats", view.admin_cockpit.boat_count, "/admin/fleet"],
        ["Active", view.admin_cockpit.active_voyages, "/admin/trips"],
        ["Upcoming", view.admin_cockpit.upcoming_voyages, "/admin/trips"],
        ["Leads", view.admin_cockpit.lead_count, "/admin/trips"],
        ["Quotes", view.admin_cockpit.quote_count, "/admin/trips"],
      ]
    : [
        [
          "Assigned",
          view.director_cockpit?.assigned_voyages ?? 0,
          "/admin/trips",
        ],
        ["Active", view.director_cockpit?.active_voyages ?? 0, "/admin/trips"],
        [
          "Upcoming",
          view.director_cockpit?.upcoming_voyages ?? 0,
          "/admin/trips",
        ],
        [
          "Open folios",
          view.director_cockpit?.open_folios ?? 0,
          "/admin/trips",
        ],
        ["Signals", view.director_cockpit?.blockers ?? 0, "/admin/audit"],
      ];
  return (
    <section className={styles.missionControl} aria-label="Mission control">
      {metrics.map(([label, value, route]) => (
        <Link key={label} to={route} className={styles.missionMetric}>
          <span>{label}</span>
          <strong>{value}</strong>
        </Link>
      ))}
    </section>
  );
}

function fixtureForRole(role: "admin" | "director") {
  return role === "director" ? directorCockpitFixture : adminCockpitFixture;
}

function formatSection(section: string) {
  return section.replaceAll("_", " ");
}

function fixtureRoleFromLocation(): "admin" | "director" | null {
  if (typeof window === "undefined") return null;
  const fromQuery = new URLSearchParams(window.location.search).get(
    "cockpit_fixture",
  );
  if (fromQuery === "admin" || fromQuery === "director") return fromQuery;
  const raw = window.localStorage.getItem("cockpit_fixture_role");
  return raw === "admin" || raw === "director" ? raw : null;
}

function setFixture(
  role: "admin" | "director" | null,
  setFixtureRole: (role: "admin" | "director" | null) => void,
) {
  setFixtureRole(role);
  if (typeof window === "undefined") return;
  if (role) {
    window.localStorage.setItem("cockpit_fixture_role", role);
  } else {
    window.localStorage.removeItem("cockpit_fixture_role");
  }
}
