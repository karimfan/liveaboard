import { useEffect, useState, type ChangeEvent } from "react";

import { adminApi, type AdminUser, type Trip } from "./api";
import type { ApiError } from "../lib/api";

// useCruiseDirectors fetches the org's active cruise directors once
// and caches them in component state. Used by trip-list views to
// populate the assignment dropdown without each row re-fetching.
export function useCruiseDirectors(canEdit: boolean) {
  const [directors, setDirectors] = useState<AdminUser[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!canEdit) {
      setLoaded(true);
      return;
    }
    let cancelled = false;
    adminApi
      .listUsers()
      .then((res) => {
        if (cancelled) return;
        const dirs = (res.users ?? []).filter(
          (u) => u.role === "cruise_director" && u.is_active,
        );
        setDirectors(dirs);
        setLoaded(true);
      })
      .catch(() => {
        if (cancelled) return;
        setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [canEdit]);

  return { directors, loaded };
}

// AssignDirector is the per-row cell. For admins it renders a select;
// for everyone else it renders the name (or "Unassigned" chip). The
// select POSTs the change immediately on selection and calls
// onAssigned so the parent can patch its trip list in place.
export function AssignDirector({
  trip,
  directors,
  canEdit,
  onAssigned,
}: {
  trip: Trip;
  directors: AdminUser[];
  canEdit: boolean;
  onAssigned: (tripId: string, directorId: string | null, name: string | null) => void;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!canEdit) {
    return trip.cruise_director_name ? (
      <>{trip.cruise_director_name}</>
    ) : (
      <span className="chip chip--warn">Unassigned</span>
    );
  }

  async function onChange(e: ChangeEvent<HTMLSelectElement>) {
    setError(null);
    const v = e.target.value;
    const directorId = v === "" ? null : v;
    setSubmitting(true);
    try {
      await adminApi.assignCruiseDirector(trip.id, directorId);
      const name = directorId
        ? directors.find((d) => d.id === directorId)?.full_name ?? null
        : null;
      onAssigned(trip.id, directorId, name);
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Failed to assign");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <select
        value={trip.cruise_director_user_id ?? ""}
        onChange={onChange}
        disabled={submitting}
        style={{ minWidth: 160 }}
        aria-label={`Cruise director for ${trip.boat_name}`}
      >
        <option value="">— Unassigned —</option>
        {directors.map((d) => (
          <option key={d.id} value={d.id}>
            {d.full_name}
          </option>
        ))}
      </select>
      {error && (
        <div className="muted" style={{ color: "var(--c-error)", fontSize: 12, marginTop: 2 }}>
          {error}
        </div>
      )}
    </div>
  );
}
