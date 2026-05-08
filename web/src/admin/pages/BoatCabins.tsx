import { useEffect, useMemo, useState, type ChangeEvent, type FormEvent } from "react";
import { useOutletContext } from "react-router-dom";

import { adminApi, type Boat, type CabinLayout, type CabinLayoutInput, type Trip } from "../api";

type Ctx = { boat: Boat; trips: Trip[]; refreshBoat: () => Promise<void> };

export function BoatCabins() {
  const { boat } = useOutletContext<Ctx>();
  const [layout, setLayout] = useState<CabinLayout | null>(null);
  const [mode, setMode] = useState<"ranges" | "paste" | "csv">("ranges");
  const [rangeFrom, setRangeFrom] = useState("1");
  const [rangeTo, setRangeTo] = useState("10");
  const [rangeBerths, setRangeBerths] = useState("A,B");
  const [rangeDeck, setRangeDeck] = useState("");
  const [paste, setPaste] = useState("1,A,B\n2,A,B\n3,A");
  const [csvText, setCsvText] = useState("cabin_label,berth_label,deck,sort_order,notes\n1,A,Lower,10,\n1,B,Lower,11,\n2,A,Lower,20,\n2,B,Lower,21,");
  const [previewCount, setPreviewCount] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      setLayout(await adminApi.boatCabins(boat.id));
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to load cabin layout.");
    }
  }

  useEffect(() => {
    void load();
  }, [boat.id]);

  const input = useMemo<CabinLayoutInput>(() => {
    if (mode === "ranges") {
      return {
        source: "ranges",
        ranges: [{
          from: Number(rangeFrom),
          to: Number(rangeTo),
          berths: rangeBerths.split(",").map((s) => s.trim()).filter(Boolean),
          deck: rangeDeck.trim() || null,
        }],
      };
    }
    if (mode === "csv") return { source: "csv", csv: csvText };
    return { source: "paste", paste };
  }, [mode, rangeFrom, rangeTo, rangeBerths, rangeDeck, paste, csvText]);

  async function preview(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setMessage(null);
    try {
      const p = await adminApi.previewBoatCabins(boat.id, input);
      const berths = (p.cabins ?? []).reduce((sum, c) => sum + c.berths.length, 0);
      setPreviewCount(berths);
      setMessage(`Preview ready: ${p.cabins.length} cabins, ${berths} berths.`);
    } catch (err) {
      setPreviewCount(null);
      setError((err as { message?: string })?.message ?? "Preview failed.");
    }
  }

  async function save() {
    setError(null);
    setMessage(null);
    try {
      const next = await adminApi.replaceBoatCabins(boat.id, input);
      setLayout(next);
      setPreviewCount(null);
      setMessage("Cabin layout saved.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not save layout.");
    }
  }

  async function onCSVFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setCsvText(await file.text());
  }

  async function deactivateCabin(id: string) {
    setError(null);
    try {
      await adminApi.deactivateBoatCabin(boat.id, id);
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not deactivate cabin.");
    }
  }

  async function deactivateBerth(cabinId: string, berthId: string) {
    setError(null);
    try {
      await adminApi.deactivateBoatBerth(boat.id, cabinId, berthId);
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not deactivate berth.");
    }
  }

  return (
    <div className="cabin-layout">
      {error && <div className="error">{error}</div>}
      {message && <div className="callout">{message}</div>}

      <div className="admin-card cabin-layout__builder">
        <div className="admin-card__header">
          <div>
            <h2 className="admin-card__title">Cabin layout</h2>
            <p className="muted">
              {layout ? `${layout.active_cabin_count} active cabins, ${layout.active_berth_count} active berths` : "Loading..."}
            </p>
          </div>
          <div className="segmented">
            {(["ranges", "paste", "csv"] as const).map((m) => (
              <button key={m} type="button" className={mode === m ? "is-active" : ""} onClick={() => setMode(m)}>
                {m === "ranges" ? "Generate" : m.toUpperCase()}
              </button>
            ))}
          </div>
        </div>

        <form onSubmit={preview}>
          {mode === "ranges" && (
            <div className="form-row">
              <div className="field"><label>From cabin</label><input type="number" min="1" value={rangeFrom} onChange={(e) => setRangeFrom(e.target.value)} /></div>
              <div className="field"><label>To cabin</label><input type="number" min="1" value={rangeTo} onChange={(e) => setRangeTo(e.target.value)} /></div>
              <div className="field"><label>Berths</label><input value={rangeBerths} onChange={(e) => setRangeBerths(e.target.value)} placeholder="A,B" /></div>
              <div className="field"><label>Deck</label><input value={rangeDeck} onChange={(e) => setRangeDeck(e.target.value)} placeholder="Lower" /></div>
            </div>
          )}
          {mode === "paste" && (
            <div className="field">
              <label>Paste layout</label>
              <textarea rows={8} value={paste} onChange={(e) => setPaste(e.target.value)} />
              <p className="muted">Use one cabin per row: <code>1,A,B</code></p>
            </div>
          )}
          {mode === "csv" && (
            <div className="field">
              <label>CSV layout</label>
              <input type="file" accept=".csv,text/csv" onChange={onCSVFile} />
              <textarea rows={8} value={csvText} onChange={(e) => setCsvText(e.target.value)} />
              <p className="muted">Required schema: <code>cabin_label,berth_label,deck,sort_order,notes</code>. One berth per row.</p>
            </div>
          )}
          <div className="form-actions">
            <button type="submit" className="secondary">Preview</button>
            <button type="button" className="primary" disabled={previewCount == null} onClick={save}>Save layout</button>
          </div>
        </form>
      </div>

      {!layout || layout.cabins.length === 0 ? (
        <div className="empty-state">
          <h3>No cabin layout</h3>
          <p>Generate, paste, or upload a CSV layout to create berths for assignments.</p>
        </div>
      ) : (
        <table className="admin-table">
          <thead><tr><th>Cabin</th><th>Deck</th><th>Berths</th><th></th></tr></thead>
          <tbody>
            {layout.cabins.map((c) => (
              <tr key={c.id} className={!c.is_active ? "is-muted" : ""}>
                <td>{c.label}</td>
                <td>{c.deck ?? "—"}</td>
                <td>
                  <div className="berth-list">
                    {c.berths.map((b) => (
                      <span key={b.id} className={"berth-pill" + (!b.is_active ? " is-inactive" : "")}>
                        {b.display_label}
                        {b.is_active && <button type="button" onClick={() => deactivateBerth(c.id, b.id)}>×</button>}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="actions-cell">
                  {c.is_active && <button type="button" className="ghost" onClick={() => deactivateCabin(c.id)}>Deactivate</button>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
