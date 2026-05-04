// Hardcoded mock data for the Sprint 007 Admin UX sample.
// A future sprint replaces these with live /api/admin/* calls.

export type Boat = {
  id: string;
  slug: string;
  name: string;
  description: string;
  capacity: number;
  imageUrl: string;
  sourceUrl: string;
  lastSyncedAt: string;
  upcomingTrips: number;
  status: "active" | "archived";
};

export type Trip = {
  id: string;
  boatId: string;
  boatName: string;
  itinerary: string;
  startDate: string;
  endDate: string;
  director: string | null;
  manifestFilled: number;
  manifestCapacity: number;
  status: "planned" | "active" | "completed" | "cancelled";
  priceText: string;
  availability: "FULL" | "AVAILABLE" | "ON REQUEST" | "ONLY 1 SPACE LEFT";
};

export type CatalogItem = {
  id: string;
  name: string;
  category: string;
  priceText: string;
  active: boolean;
};

export type InventoryRow = {
  itemId: string;
  itemName: string;
  category: string;
  onHand: number;
  minThreshold: number;
};

export type User = {
  id: string;
  fullName: string;
  email: string;
  role: "Org Admin" | "Site Director";
  status: "active" | "pending invite" | "deactivated";
  invitedAt?: string;
};

export type Alert = {
  kind: "low_stock" | "no_director" | "low_manifest" | "missing_currency";
  text: string;
  href: string;
};

export type ActivityEntry = {
  text: string;
  ts: string; // relative
};

// --- boats ---

export const boats: Boat[] = [
  {
    id: "boat_1",
    slug: "gaia-love",
    name: "Gaia Love",
    description: "40m luxury liveaboard, year-round Indonesia.",
    capacity: 22,
    imageUrl: "https://img.liveaboard.com/picture_library/boat/5695/gaia-main.jpg",
    sourceUrl: "https://www.liveaboard.com/diving/indonesia/gaia-love",
    lastSyncedAt: "2 hours ago",
    upcomingTrips: 16,
    status: "active",
  },
  {
    id: "boat_2",
    slug: "blue-spirit",
    name: "Blue Spirit",
    description: "Maldives Central Atolls operation.",
    capacity: 18,
    imageUrl: "https://img.liveaboard.com/picture_library/boat/5500/SEA_SPIRIT_maldives_Main.jpg",
    sourceUrl: "https://www.liveaboard.com/diving/maldives/blue-spirit",
    lastSyncedAt: "yesterday",
    upcomingTrips: 9,
    status: "active",
  },
  {
    id: "boat_3",
    slug: "seahorse",
    name: "Seahorse",
    description: "Coral Triangle expeditions.",
    capacity: 12,
    imageUrl: "",
    sourceUrl: "",
    lastSyncedAt: "—",
    upcomingTrips: 0,
    status: "active",
  },
];

// --- trips ---

export const trips: Trip[] = [
  {
    id: "trip_1",
    boatId: "boat_1",
    boatName: "Gaia Love",
    itinerary: "Raja Ampat North & South",
    startDate: "2027-02-06",
    endDate: "2027-02-16",
    director: "Maya Patel",
    manifestFilled: 22,
    manifestCapacity: 22,
    status: "planned",
    priceText: "$6,400",
    availability: "FULL",
  },
  {
    id: "trip_2",
    boatId: "boat_1",
    boatName: "Gaia Love",
    itinerary: "Raja Ampat North & South",
    startDate: "2027-02-18",
    endDate: "2027-02-28",
    director: null,
    manifestFilled: 18,
    manifestCapacity: 22,
    status: "planned",
    priceText: "$6,400",
    availability: "FULL",
  },
  {
    id: "trip_3",
    boatId: "boat_1",
    boatName: "Gaia Love",
    itinerary: "Komodo",
    startDate: "2027-03-04",
    endDate: "2027-03-14",
    director: "Joonseok Min",
    manifestFilled: 4,
    manifestCapacity: 22,
    status: "planned",
    priceText: "$5,900",
    availability: "AVAILABLE",
  },
  {
    id: "trip_4",
    boatId: "boat_2",
    boatName: "Blue Spirit",
    itinerary: "Maldives Central Atolls",
    startDate: "2027-03-09",
    endDate: "2027-03-16",
    director: "Karim Wright",
    manifestFilled: 14,
    manifestCapacity: 18,
    status: "planned",
    priceText: "$3,200",
    availability: "AVAILABLE",
  },
  {
    id: "trip_5",
    boatId: "boat_2",
    boatName: "Blue Spirit",
    itinerary: "Maldives South",
    startDate: "2027-04-01",
    endDate: "2027-04-08",
    director: null,
    manifestFilled: 0,
    manifestCapacity: 18,
    status: "planned",
    priceText: "$3,400",
    availability: "AVAILABLE",
  },
  {
    id: "trip_6",
    boatId: "boat_1",
    boatName: "Gaia Love",
    itinerary: "Banda Sea",
    startDate: "2027-04-12",
    endDate: "2027-04-22",
    director: "Maya Patel",
    manifestFilled: 22,
    manifestCapacity: 22,
    status: "planned",
    priceText: "$7,200",
    availability: "FULL",
  },
];

// --- catalog ---

export const catalogItems: CatalogItem[] = [
  { id: "item_1", name: "T-shirt XL",        category: "Apparel",     priceText: "$25",   active: true },
  { id: "item_2", name: "T-shirt M",         category: "Apparel",     priceText: "$25",   active: true },
  { id: "item_3", name: "Mask defog 30ml",   category: "Consumables", priceText: "$12",   active: true },
  { id: "item_4", name: "Aluminum tank",     category: "Equipment",   priceText: "$15",   active: true },
  { id: "item_5", name: "Nitrox fill",       category: "Services",    priceText: "$10",   active: true },
  { id: "item_6", name: "Beer (local)",      category: "Beverages",   priceText: "$5",    active: true },
  { id: "item_7", name: "Wine (bottle)",     category: "Beverages",   priceText: "$45",   active: true },
  { id: "item_8", name: "Dive computer rental", category: "Equipment", priceText: "$30/day", active: true },
  { id: "item_9", name: "Laundry (per kg)",  category: "Services",    priceText: "$8",    active: true },
];

// --- per-boat inventory ---

export const inventoryByBoat: Record<string, InventoryRow[]> = {
  boat_1: [
    { itemId: "item_1", itemName: "T-shirt XL",          category: "Apparel",     onHand: 10,  minThreshold: 5 },
    { itemId: "item_2", itemName: "T-shirt M",           category: "Apparel",     onHand: 4,   minThreshold: 5 },
    { itemId: "item_3", itemName: "Mask defog 30ml",     category: "Consumables", onHand: 27,  minThreshold: 20 },
    { itemId: "item_4", itemName: "Aluminum tank",       category: "Equipment",   onHand: 8,   minThreshold: 12 },
    { itemId: "item_5", itemName: "Nitrox fill",         category: "Services",    onHand: 999, minThreshold: 0 },
    { itemId: "item_6", itemName: "Beer (local)",        category: "Beverages",   onHand: 96,  minThreshold: 48 },
    { itemId: "item_7", itemName: "Wine (bottle)",       category: "Beverages",   onHand: 22,  minThreshold: 10 },
  ],
  boat_2: [
    { itemId: "item_1", itemName: "T-shirt XL",          category: "Apparel",     onHand: 40,  minThreshold: 5 },
    { itemId: "item_2", itemName: "T-shirt M",           category: "Apparel",     onHand: 35,  minThreshold: 5 },
    { itemId: "item_3", itemName: "Mask defog 30ml",     category: "Consumables", onHand: 12,  minThreshold: 20 },
    { itemId: "item_4", itemName: "Aluminum tank",       category: "Equipment",   onHand: 14,  minThreshold: 12 },
    { itemId: "item_6", itemName: "Beer (local)",        category: "Beverages",   onHand: 30,  minThreshold: 48 },
  ],
  boat_3: [],
};

// --- users ---

export const users: User[] = [
  { id: "u_1", fullName: "Karim Fanous",       email: "owner@acme.test",  role: "Org Admin",     status: "active" },
  { id: "u_2", fullName: "Maya Patel",         email: "maya@acme.test",   role: "Site Director", status: "active" },
  { id: "u_3", fullName: "Joonseok Min",       email: "jm@acme.test",     role: "Site Director", status: "active" },
  { id: "u_4", fullName: "Karim Wright",       email: "kw@acme.test",     role: "Site Director", status: "active" },
  { id: "u_5", fullName: "(pending)",          email: "newhire@acme.test", role: "Site Director", status: "pending invite", invitedAt: "3 days ago" },
];

// --- org ---

export const organization = {
  name: "Acme Diving",
  currency: "USD",
  createdAt: "May 2026",
};

// --- alerts (Overview) ---

export const trippsNeedingAttention: Trip[] = trips.filter(
  (t) => t.director === null || (t.manifestFilled / t.manifestCapacity < 0.5 && t.status === "planned"),
);

export const lowStock: { boatId: string; boatName: string; lowCount: number }[] = (() => {
  const out: { boatId: string; boatName: string; lowCount: number }[] = [];
  for (const b of boats) {
    const rows = inventoryByBoat[b.id] ?? [];
    const lows = rows.filter((r) => r.onHand < r.minThreshold).length;
    if (lows > 0) out.push({ boatId: b.id, boatName: b.name, lowCount: lows });
  }
  return out;
})();

export const recentActivity: ActivityEntry[] = [
  { text: "Boat “Seahorse” added to fleet.",           ts: "2 hours ago" },
  { text: "Item “T-shirt M” price set to $25.",        ts: "4 hours ago" },
  { text: "Director “Maya Patel” assigned to a trip.", ts: "yesterday" },
  { text: "Pending invite sent to newhire@acme.test.",           ts: "3 days ago" },
];

// --- setup completeness ---

export type SetupStep = {
  label: string;
  done: boolean;
  hint?: string;
  href?: string;
};

export const setupSteps: SetupStep[] = [
  { label: "Set organization currency",  done: true,  hint: "USD", href: "/admin/organization" },
  { label: "Add or import a boat",       done: true,  hint: `${boats.length} boats`, href: "/admin/fleet" },
  { label: "Seed your catalog",          done: true,  hint: `${catalogItems.length} items`, href: "/admin/catalog" },
  { label: "Set per-boat inventory",     done: false, hint: "1 boat with no stock entries", href: "/admin/fleet/seahorse" },
  { label: "Invite a Site Director",     done: true,  hint: `${users.filter((u) => u.role === "Site Director" && u.status === "active").length} active`, href: "/admin/users" },
  { label: "Create your first trip",     done: true,  hint: `${trips.length} trips planned`, href: "/admin/trips" },
];

export const setupCompletenessPct = Math.round(
  (setupSteps.filter((s) => s.done).length / setupSteps.length) * 100,
);
