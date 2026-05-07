import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";

import {
  adminApi,
  type Boat,
  type CatalogCategory,
  type CatalogItem,
  type FXRate,
  type InventoryBoatSummary,
} from "../api";

type Tab = "items" | "categories" | "boats" | "fx";

const chargeTypes = ["sale", "rental", "service", "fee", "gratuity", "deposit", "damage", "included"];
const units = ["each", "can", "bottle", "glass", "fill", "day", "week", "trip", "session", "item", "bag", "night", "person"];

export function Inventory() {
  const [tab, setTab] = useState<Tab>("items");
  const [categories, setCategories] = useState<CatalogCategory[]>([]);
  const [items, setItems] = useState<CatalogItem[]>([]);
  const [boats, setBoats] = useState<Boat[]>([]);
  const [summary, setSummary] = useState<InventoryBoatSummary[]>([]);
  const [rates, setRates] = useState<FXRate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  async function load() {
    setError(null);
    setLoading(true);
    try {
      const [cats, its, fleet, inv, fx] = await Promise.all([
        adminApi.listCatalogCategories(),
        adminApi.listCatalogItems(),
        adminApi.listBoats(),
        adminApi.inventoryBoatSummary(),
        adminApi.listFXRates(),
      ]);
      setCategories(cats.categories ?? []);
      setItems(its.items ?? []);
      setBoats(fleet.boats ?? []);
      setSummary(inv.boats ?? []);
      setRates(fx.rates ?? []);
    } catch (e) {
      setError((e as { message?: string })?.message ?? "Failed to load inventory.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const filteredItems = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((i) =>
      `${i.name} ${i.category_name} ${i.unit} ${i.charge_type}`.toLowerCase().includes(q),
    );
  }, [items, search]);

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Inventory</h1>
          <div className="admin-page-subtitle">
            Catalog items, USD prices, per-boat stock, and checkout rates.
          </div>
        </div>
        <button className="secondary" type="button" onClick={() => void applyDefaults(load, setError)}>
          Apply missing defaults
        </button>
      </div>

      <div className="tabs">
        {(["items", "categories", "boats", "fx"] as Tab[]).map((t) => (
          <button
            key={t}
            type="button"
            className={"tab" + (tab === t ? " is-active" : "")}
            onClick={() => setTab(t)}
          >
            {t === "fx" ? "FX Rates" : title(t)}
          </button>
        ))}
      </div>

      {error && <div className="error">{error}</div>}
      {loading ? (
        <div className="muted">Loading…</div>
      ) : (
        <>
          {tab === "items" && (
            <ItemsTab
              items={filteredItems}
              categories={categories.filter((c) => !c.archived_at)}
              search={search}
              setSearch={setSearch}
              reload={load}
              setError={setError}
            />
          )}
          {tab === "categories" && (
            <CategoriesTab categories={categories} reload={load} setError={setError} />
          )}
          {tab === "boats" && (
            <BoatStockTab boats={boats} summary={summary} />
          )}
          {tab === "fx" && (
            <FXTab rates={rates} reload={load} setError={setError} />
          )}
        </>
      )}
    </>
  );
}

function ItemsTab({
  items,
  categories,
  search,
  setSearch,
  reload,
  setError,
}: {
  items: CatalogItem[];
  categories: CatalogCategory[];
  search: string;
  setSearch: (s: string) => void;
  reload: () => Promise<void>;
  setError: (s: string | null) => void;
}) {
  const [editing, setEditing] = useState<CatalogItem | null>(null);
  return (
    <>
      <div className="filter-bar">
        <input value={search} onChange={(e) => setSearch(e.target.value)} type="search" placeholder="Search items..." />
        <div className="filter-bar__spacer" />
        <button type="button" className="primary" onClick={() => setEditing(blankItem(categories))}>
          + Add item
        </button>
      </div>
      <table className="admin-table">
        <thead>
          <tr>
            <th>Category</th>
            <th>Item</th>
            <th>Unit</th>
            <th>Type</th>
            <th>Stock</th>
            <th className="num">USD</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {items.map((i) => (
            <tr key={i.id} onClick={() => setEditing(i)} className="click-row">
              <td>{i.category_name}</td>
              <td>{i.name}</td>
              <td>{i.unit}</td>
              <td>{i.charge_type}</td>
              <td>{i.stock_mode}</td>
              <td className="num">{usd(i.price_usd_cents)}</td>
              <td><span className={"chip " + (i.is_active && !i.archived_at ? "chip--active" : "chip--full")}>{i.archived_at ? "Archived" : i.is_active ? "Active" : "Inactive"}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
      {editing && (
        <ItemEditor
          item={editing}
          categories={categories}
          close={() => setEditing(null)}
          saved={() => {
            setEditing(null);
            void reload();
          }}
          setError={setError}
        />
      )}
    </>
  );
}

function CategoriesTab({ categories, reload, setError }: { categories: CatalogCategory[]; reload: () => Promise<void>; setError: (s: string | null) => void }) {
  const [name, setName] = useState("");
  async function add(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await adminApi.createCatalogCategory({ name, sort_order: categories.length * 10 + 10 });
      setName("");
      await reload();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to create category.");
    }
  }
  return (
    <div className="admin-grid">
      <form className="admin-card" onSubmit={add}>
        <h2 className="admin-card__title">New category</h2>
        <div className="field">
          <label>Name</label>
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <button className="primary" type="submit">Add category</button>
      </form>
      <table className="admin-table">
        <thead><tr><th>Name</th><th>Items</th><th>Sort</th><th>Status</th></tr></thead>
        <tbody>
          {categories.map((c) => (
            <tr key={c.id}>
              <td>{c.name}</td>
              <td>{c.item_count}</td>
              <td>{c.sort_order}</td>
              <td>{c.archived_at ? "Archived" : "Active"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BoatStockTab({ boats, summary }: { boats: Boat[]; summary: InventoryBoatSummary[] }) {
  const byBoat = new Map(summary.map((s) => [s.boat_id, s]));
  return (
    <table className="admin-table">
      <thead><tr><th>Boat</th><th className="num">Low stock</th><th className="num">Out</th><th></th></tr></thead>
      <tbody>
        {boats.map((b) => {
          const s = byBoat.get(b.id);
          return (
            <tr key={b.id}>
              <td>{b.name}</td>
              <td className="num">{s?.low_stock_count ?? 0}</td>
              <td className="num">{s?.out_stock_count ?? 0}</td>
              <td><Link to={`/admin/fleet/${b.id}/inventory`}>Open stock</Link></td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function FXTab({ rates, reload, setError }: { rates: FXRate[]; reload: () => Promise<void>; setError: (s: string | null) => void }) {
  const [quoteCurrency, setQuoteCurrency] = useState("EUR");
  const [num, setNum] = useState("92");
  const [den, setDen] = useState("100");
  async function add(e: FormEvent) {
    e.preventDefault();
    const now = new Date();
    const expires = new Date(now.getTime() + 24 * 60 * 60 * 1000);
    setError(null);
    try {
      await adminApi.createFXRate({
        provider: "manual",
        base_currency: "USD",
        quote_currency: quoteCurrency,
        rate_numerator: Number(num),
        rate_denominator: Number(den),
        as_of: now.toISOString(),
        expires_at: expires.toISOString(),
      });
      await reload();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to add rate.");
    }
  }
  return (
    <div className="admin-grid">
      <form className="admin-card" onSubmit={add}>
        <h2 className="admin-card__title">Manual USD rate</h2>
        <div className="field"><label>Target currency</label><input value={quoteCurrency} onChange={(e) => setQuoteCurrency(e.target.value.toUpperCase())} maxLength={3} /></div>
        <div className="form-row">
          <div className="field"><label>Numerator</label><input type="number" min="1" value={num} onChange={(e) => setNum(e.target.value)} /></div>
          <div className="field"><label>Denominator</label><input type="number" min="1" value={den} onChange={(e) => setDen(e.target.value)} /></div>
        </div>
        <button className="primary" type="submit">Add rate</button>
      </form>
      <table className="admin-table">
        <thead><tr><th>Pair</th><th>Rate</th><th>Provider</th><th>Expires</th></tr></thead>
        <tbody>
          {rates.map((r) => (
            <tr key={r.id}>
              <td>{r.base_currency}/{r.quote_currency}</td>
              <td>{r.rate_numerator}/{r.rate_denominator}</td>
              <td>{r.provider}</td>
              <td>{new Date(r.expires_at).toLocaleString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ItemEditor({ item, categories, close, saved, setError }: { item: CatalogItem; categories: CatalogCategory[]; close: () => void; saved: () => void; setError: (s: string | null) => void }) {
  const isNew = item.id === "";
  const [form, setForm] = useState(item);
  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const input = {
      category_id: form.category_id,
      name: form.name,
      description: form.description,
      unit: form.unit,
      charge_type: form.charge_type,
      stock_mode: form.stock_mode,
      price_usd_cents: Number(form.price_usd_cents),
      is_taxable: form.is_taxable,
      is_required_fee: form.is_required_fee,
      is_active: form.is_active,
      archived: form.archived_at ? true : undefined,
    };
    try {
      if (isNew) await adminApi.createCatalogItem(input);
      else await adminApi.updateCatalogItem(form.id, input);
      saved();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Save failed.");
    }
  }
  return (
    <div className="modal-backdrop">
      <form className="modal" onSubmit={submit}>
        <h2>{isNew ? "Add item" : "Edit item"}</h2>
        <div className="field"><label>Name</label><input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></div>
        <div className="field"><label>Category</label><select value={form.category_id} onChange={(e) => setForm({ ...form, category_id: e.target.value })}>{categories.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}</select></div>
        <div className="form-row">
          <div className="field"><label>Unit</label><select value={form.unit} onChange={(e) => setForm({ ...form, unit: e.target.value })}>{units.map((u) => <option key={u} value={u}>{u}</option>)}</select></div>
          <div className="field"><label>USD cents</label><input type="number" min="0" value={form.price_usd_cents} onChange={(e) => setForm({ ...form, price_usd_cents: Number(e.target.value) })} /></div>
        </div>
        <div className="form-row">
          <div className="field"><label>Charge type</label><select value={form.charge_type} onChange={(e) => setForm({ ...form, charge_type: e.target.value })}>{chargeTypes.map((t) => <option key={t} value={t}>{t}</option>)}</select></div>
          <div className="field"><label>Stock</label><select value={form.stock_mode} onChange={(e) => setForm({ ...form, stock_mode: e.target.value as "none" | "counted" })}><option value="none">none</option><option value="counted">counted</option></select></div>
        </div>
        <label className="checkline"><input type="checkbox" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} /> Active</label>
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={close}>Cancel</button>
          <button type="submit" className="primary">Save</button>
        </div>
      </form>
    </div>
  );
}

function blankItem(categories: CatalogCategory[]): CatalogItem {
  return {
    id: "",
    category_id: categories[0]?.id ?? "",
    category_name: categories[0]?.name ?? "",
    template_key: null,
    name: "",
    description: null,
    unit: "each",
    charge_type: "sale",
    stock_mode: "none",
    price_usd_cents: 0,
    is_taxable: false,
    is_required_fee: false,
    is_active: true,
    archived_at: null,
  };
}

async function applyDefaults(reload: () => Promise<void>, setError: (s: string | null) => void) {
  setError(null);
  try {
    await adminApi.applyCatalogDefaults();
    await reload();
  } catch (err) {
    setError((err as { message?: string })?.message ?? "Failed to apply defaults.");
  }
}

function usd(cents: number) {
  return `$${(cents / 100).toFixed(2)}`;
}

function title(s: string) {
  return s.slice(0, 1).toUpperCase() + s.slice(1);
}
