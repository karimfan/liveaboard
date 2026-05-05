import { useEffect, useRef, useState, type ChangeEvent } from "react";

import { adminApi, type AdminUser, type Trip, type TripDirectorsView } from "./api";
import type { ApiError } from "../lib/api";

// Sprint 013 — multi-director assignment. Each trip can carry any
// number of Cruise Directors via the trip_cruise_directors join
// table. The UI is a chip list: existing assignments render as
// removable chips; an "Add" select picks from unassigned directors
// in the org. Each add/remove is one network call (with email
// notification dispatched server-side).

// useCruiseDirectors fetches the org's active cruise directors once
// per page and caches them in component state. Used by trip-list
// views to populate the per-row assignment dropdown.
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

// AssignDirector renders the per-row cell. For admins it shows
// removable chips for each assigned director plus an "Add" select.
// For cruise directors viewing their own trips it renders names
// (or the Unassigned chip).
export function AssignDirector({
  trip,
  directors,
  canEdit,
  onChanged,
}: {
  trip: Trip;
  directors: AdminUser[];
  canEdit: boolean;
  onChanged: (tripId: string, ids: string[], names: string[]) => void;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Tracks the last user-initiated select value so we can reset
  // the picker after a successful add.
  const pickerRef = useRef<HTMLSelectElement | null>(null);

  // Read-only view for non-admins.
  if (!canEdit) {
    if (trip.cruise_director_names.length === 0) {
      return <span className="chip chip--warn">Unassigned</span>;
    }
    return (
      <div className="director-chips director-chips--readonly">
        {trip.cruise_director_names.map((n, i) => (
          <span key={i} className="director-chip">
            {n}
          </span>
        ))}
      </div>
    );
  }

  const assignedIDs = new Set(trip.cruise_director_user_ids);
  const available = directors.filter((d) => !assignedIDs.has(d.id));

  function applyResult(res: TripDirectorsView) {
    onChanged(res.trip_id, res.cruise_director_user_ids, res.cruise_director_names);
  }

  async function add(e: ChangeEvent<HTMLSelectElement>) {
    setError(null);
    const userId = e.target.value;
    if (!userId) return;
    setSubmitting(true);
    try {
      const res = await adminApi.addCruiseDirector(trip.id, userId);
      applyResult(res);
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Failed to assign");
    } finally {
      setSubmitting(false);
      // Reset the picker to the placeholder option.
      if (pickerRef.current) pickerRef.current.value = "";
    }
  }

  async function remove(userId: string) {
    setError(null);
    setSubmitting(true);
    try {
      const res = await adminApi.removeCruiseDirector(trip.id, userId);
      applyResult(res);
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Failed to remove");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="director-assign">
      <div className="director-chips">
        {trip.cruise_director_user_ids.map((id, i) => {
          const name = trip.cruise_director_names[i] ?? id;
          return (
            <span key={id} className="director-chip">
              {name}
              <button
                type="button"
                className="director-chip__x"
                aria-label={`Remove ${name}`}
                onClick={() => remove(id)}
                disabled={submitting}
              >
                ×
              </button>
            </span>
          );
        })}
        {trip.cruise_director_user_ids.length === 0 && (
          <span className="chip chip--warn">Unassigned</span>
        )}
      </div>
      {available.length > 0 && (
        <select
          ref={pickerRef}
          className="select-inline director-assign__picker"
          defaultValue=""
          onChange={add}
          disabled={submitting}
          aria-label={`Add cruise director to ${trip.boat_name}`}
        >
          <option value="">+ Add director</option>
          {available.map((d) => (
            <option key={d.id} value={d.id}>
              {d.full_name}
            </option>
          ))}
        </select>
      )}
      {error && (
        <div style={{ color: "var(--c-error)", fontSize: 12, marginTop: 2 }}>
          {error}
        </div>
      )}
    </div>
  );
}
