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
import {
  Button,
  Chip,
  type Column,
  DataTable,
  Field,
  PageHeader,
  Tabs,
} from "../components";

import styles from "./Inventory.module.css";

type Tab = "items" | "categories" | "boats" | "fx";

const chargeTypes = [
  "sale",
  "rental",
  "service",
  "fee",
  "gratuity",
  "deposit",
  "damage",
  "included",
];
const units = [
  "each",
  "can",
  "bottle",
  "glass",
  "fill",
  "day",
  "week",
  "trip",
  "session",
  "item",
  "bag",
  "night",
  "person",
];

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
      setError(
        (e as { message?: string })?.message ?? "Failed to load inventory.",
      );
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
      `${i.name} ${i.category_name} ${i.unit} ${i.charge_type}`
        .toLowerCase()
        .includes(q),
    );
  }, [items, search]);

  return (
    <>
      <PageHeader
        title="Inventory"
        subtitle="Catalog items, USD prices, per-boat stock, and checkout rates."
        actions={
          <Button
            variant="secondary"
            onClick={() => void applyDefaults(load, setError)}
          >
            Apply missing defaults
          </Button>
        }
      />

      <div className={styles.tabs}>
        <Tabs
          items={(["items", "categories", "boats", "fx"] as Tab[]).map((t) => ({
            key: t,
            label: t === "fx" ? "FX Rates" : title(t),
          }))}
          active={tab}
          onSelect={(k) => setTab(k as Tab)}
        />
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {loading ? (
        <div className={styles.loading}>Loading…</div>
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
            <CategoriesTab
              categories={categories}
              reload={load}
              setError={setError}
            />
          )}
          {tab === "boats" && <BoatStockTab boats={boats} summary={summary} />}
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
      <div className={styles.toolbar}>
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          type="search"
          placeholder="Search items..."
        />
        <div className={styles.spacer} />
        <Button
          variant="primary"
          onClick={() => setEditing(blankItem(categories))}
        >
          + Add item
        </Button>
      </div>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Category</th>
            <th>Item</th>
            <th>Unit</th>
            <th>Type</th>
            <th>Stock</th>
            <th className={styles.num}>USD</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {items.map((i) => (
            <tr
              key={i.id}
              onClick={() => setEditing(i)}
              className={styles.clickRow}
            >
              <td>{i.category_name}</td>
              <td>{i.name}</td>
              <td>{i.unit}</td>
              <td>{i.charge_type}</td>
              <td>{i.stock_mode}</td>
              <td className={styles.num}>{usd(i.price_usd_cents)}</td>
              <td>
                <Chip
                  variant={i.is_active && !i.archived_at ? "success" : "error"}
                >
                  {i.archived_at
                    ? "Archived"
                    : i.is_active
                      ? "Active"
                      : "Inactive"}
                </Chip>
              </td>
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

function CategoriesTab({
  categories,
  reload,
  setError,
}: {
  categories: CatalogCategory[];
  reload: () => Promise<void>;
  setError: (s: string | null) => void;
}) {
  const [name, setName] = useState("");
  async function add(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await adminApi.createCatalogCategory({
        name,
        sort_order: categories.length * 10 + 10,
      });
      setName("");
      await reload();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to create category.",
      );
    }
  }
  const columns: Column<CatalogCategory>[] = [
    { key: "name", header: "Name", cell: (c) => c.name },
    { key: "items", header: "Items", cell: (c) => c.item_count },
    { key: "sort", header: "Sort", cell: (c) => c.sort_order },
    {
      key: "status",
      header: "Status",
      cell: (c) => (
        <Chip variant={c.archived_at ? "neutral" : "success"}>
          {c.archived_at ? "Archived" : "Active"}
        </Chip>
      ),
    },
  ];
  return (
    <div className={styles.grid}>
      <form className={styles.formGrid} onSubmit={add}>
        <h2 className={styles.modalTitle}>New category</h2>
        <Field label="Name">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </Field>
        <div>
          <Button variant="primary" type="submit">
            Add category
          </Button>
        </div>
      </form>
      <DataTable columns={columns} rows={categories} rowKey={(c) => c.id} />
    </div>
  );
}

function BoatStockTab({
  boats,
  summary,
}: {
  boats: Boat[];
  summary: InventoryBoatSummary[];
}) {
  const byBoat = new Map(summary.map((s) => [s.boat_id, s]));
  const columns: Column<Boat>[] = [
    { key: "boat", header: "Boat", cell: (b) => b.name },
    {
      key: "low",
      header: "Low stock",
      align: "right",
      tabular: true,
      cell: (b) => byBoat.get(b.id)?.low_stock_count ?? 0,
    },
    {
      key: "out",
      header: "Out",
      align: "right",
      tabular: true,
      cell: (b) => byBoat.get(b.id)?.out_stock_count ?? 0,
    },
    {
      key: "actions",
      header: "",
      cell: (b) => (
        <Link to={`/admin/fleet/${b.id}/inventory`}>Open stock</Link>
      ),
    },
  ];
  return <DataTable columns={columns} rows={boats} rowKey={(b) => b.id} />;
}

function FXTab({
  rates,
  reload,
  setError,
}: {
  rates: FXRate[];
  reload: () => Promise<void>;
  setError: (s: string | null) => void;
}) {
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
  const columns: Column<FXRate>[] = [
    {
      key: "pair",
      header: "Pair",
      cell: (r) => `${r.base_currency}/${r.quote_currency}`,
    },
    {
      key: "rate",
      header: "Rate",
      cell: (r) => `${r.rate_numerator}/${r.rate_denominator}`,
    },
    { key: "provider", header: "Provider", cell: (r) => r.provider },
    {
      key: "expires",
      header: "Expires",
      cell: (r) => new Date(r.expires_at).toLocaleString(),
    },
  ];
  return (
    <div className={styles.grid}>
      <form className={styles.formGrid} onSubmit={add}>
        <h2 className={styles.modalTitle}>Manual USD rate</h2>
        <Field label="Target currency">
          <input
            value={quoteCurrency}
            onChange={(e) => setQuoteCurrency(e.target.value.toUpperCase())}
            maxLength={3}
          />
        </Field>
        <div className={styles.formRow}>
          <Field label="Numerator">
            <input
              type="number"
              min="1"
              value={num}
              onChange={(e) => setNum(e.target.value)}
            />
          </Field>
          <Field label="Denominator">
            <input
              type="number"
              min="1"
              value={den}
              onChange={(e) => setDen(e.target.value)}
            />
          </Field>
        </div>
        <div>
          <Button variant="primary" type="submit">
            Add rate
          </Button>
        </div>
      </form>
      <DataTable columns={columns} rows={rates} rowKey={(r) => r.id} />
    </div>
  );
}

function ItemEditor({
  item,
  categories,
  close,
  saved,
  setError,
}: {
  item: CatalogItem;
  categories: CatalogCategory[];
  close: () => void;
  saved: () => void;
  setError: (s: string | null) => void;
}) {
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
    <div className={styles.backdrop}>
      <form className={styles.modal} onSubmit={submit}>
        <h2 className={styles.modalTitle}>
          {isNew ? "Add item" : "Edit item"}
        </h2>
        <Field label="Name">
          <input
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            required
          />
        </Field>
        <Field label="Category">
          <select
            value={form.category_id}
            onChange={(e) => setForm({ ...form, category_id: e.target.value })}
          >
            {categories.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        </Field>
        <div className={styles.formRow}>
          <Field label="Unit">
            <select
              value={form.unit}
              onChange={(e) => setForm({ ...form, unit: e.target.value })}
            >
              {units.map((u) => (
                <option key={u} value={u}>
                  {u}
                </option>
              ))}
            </select>
          </Field>
          <Field label="USD cents">
            <input
              type="number"
              min="0"
              value={form.price_usd_cents}
              onChange={(e) =>
                setForm({ ...form, price_usd_cents: Number(e.target.value) })
              }
            />
          </Field>
        </div>
        <div className={styles.formRow}>
          <Field label="Charge type">
            <select
              value={form.charge_type}
              onChange={(e) =>
                setForm({ ...form, charge_type: e.target.value })
              }
            >
              {chargeTypes.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Stock">
            <select
              value={form.stock_mode}
              onChange={(e) =>
                setForm({
                  ...form,
                  stock_mode: e.target.value as "none" | "counted",
                })
              }
            >
              <option value="none">none</option>
              <option value="counted">counted</option>
            </select>
          </Field>
        </div>
        <label className={styles.checkline}>
          <input
            type="checkbox"
            checked={form.is_active}
            onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
          />{" "}
          Active
        </label>
        <div className={styles.modalActions}>
          <Button type="button" variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit" variant="primary">
            Save
          </Button>
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

async function applyDefaults(
  reload: () => Promise<void>,
  setError: (s: string | null) => void,
) {
  setError(null);
  try {
    await adminApi.applyCatalogDefaults();
    await reload();
  } catch (err) {
    setError(
      (err as { message?: string })?.message ?? "Failed to apply defaults.",
    );
  }
}

function usd(cents: number) {
  return `$${(cents / 100).toFixed(2)}`;
}

function title(s: string) {
  return s.slice(0, 1).toUpperCase() + s.slice(1);
}
