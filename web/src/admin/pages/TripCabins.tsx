import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type TripCabinBoard, type TripManifest } from "../api";

export function TripCabins() {
  const { id = "" } = useParams<{ id: string }>();
  const [board, setBoard] = useState<TripCabinBoard | null>(null);
  const [manifest, setManifest] = useState<TripManifest | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      const [b, m] = await Promise.all([adminApi.tripCabinBoard(id), adminApi.tripManifest(id)]);
      setBoard(b);
      setManifest(m);
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to load cabin board.");
    }
  }

  useEffect(() => {
    if (id) void load();
  }, [id]);

  const unassigned = useMemo(() => board?.unassigned_guests ?? [], [board]);

  async function assign(guestId: string, berthId: string) {
    if (!guestId) return;
    setError(null);
    setMessage(null);
    try {
      await adminApi.assignGuestCabin(id, guestId, { berth_id: berthId });
      setMessage("Cabin assignment updated.");
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not assign berth.");
    }
  }

  async function unassign(guestId: string) {
    setError(null);
    setMessage(null);
    try {
      await adminApi.unassignGuestCabin(id, guestId);
      setMessage("Guest unassigned.");
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not unassign guest.");
    }
  }

  if (!board || !manifest) {
    return (
      <>
        <div className="admin-breadcrumb"><Link to={`/admin/trips/${id}/manifest`}>Manifest</Link></div>
        {error ? <div className="error">{error}</div> : <div className="muted">Loading...</div>}
      </>
    );
  }

  return (
    <>
      <div className="admin-breadcrumb"><Link to={`/admin/trips/${id}/manifest`}>Manifest</Link></div>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Cabin board</h1>
          <div className="admin-page-subtitle">
            {manifest.trip.boat_name} - {manifest.trip.start_date} to {manifest.trip.end_date}
          </div>
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      {message && <div className="callout">{message}</div>}

      {board.cabins.length === 0 ? (
        <div className="empty-state">
          <h3>No cabin layout</h3>
          <p>This boat needs a cabin layout before guests can be assigned.</p>
        </div>
      ) : (
        <div className="cabin-board">
          {board.cabins.map((c) => (
            <section className="admin-card cabin-card" key={c.id}>
              <div className="admin-card__header">
                <div>
                  <h2 className="admin-card__title">Cabin {c.label}</h2>
                  {c.deck && <p className="muted">{c.deck}</p>}
                </div>
              </div>
              <div className="cabin-card__berths">
                {c.berths.map((b) => (
                  <div className="berth-row" key={b.id}>
                    <div>
                      <strong>{b.display_label}</strong>
                      {b.guest ? (
                        <div>{b.guest.full_name}<br /><span className="muted">{b.guest.email}</span></div>
                      ) : (
                        <span className="muted">Available</span>
                      )}
                    </div>
                    <div className="berth-row__actions">
                      {b.guest ? (
                        <button type="button" className="ghost" onClick={() => unassign(b.guest!.id)}>Unassign</button>
                      ) : (
                        <select defaultValue="" onChange={(e) => assign(e.target.value, b.id)}>
                          <option value="">Assign guest...</option>
                          {unassigned.map((g) => (
                            <option key={g.id} value={g.id}>{g.full_name}</option>
                          ))}
                        </select>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}

      {unassigned.length > 0 && (
        <div className="admin-card">
          <h2 className="admin-card__title">Needs cabin</h2>
          <table className="admin-table">
            <tbody>
              {unassigned.map((g) => (
                <tr key={g.id}>
                  <td>{g.full_name}</td>
                  <td>{g.email}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
