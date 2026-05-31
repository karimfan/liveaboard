// Sprint 025 Triptych — canonical role-aware NAV data.
//
// Every layout (rail / spaces / canvas) consumes this same
// structure. Role filtering happens in AdminShell ONCE on the
// raw NAV; filtered output is handed to the layout renderer.
// This guarantees role gating cannot drift between layouts.
//
// `Today` is the sidebar label for the index route /admin.
// The file (Overview.tsx) and route (/admin) stay stable so
// Sprint 023's onboarding auto-show
// (location.pathname === "/admin") continues to fire.

export type NavItem = {
  to: string;
  label: string;
  end?: boolean;
  adminOnly?: boolean;
  glyph?: string; // single-letter mark for the rail layout
  children?: NavItem[];
};

export type NavSpaceKey = "operations" | "configuration" | "insights";

export type NavSpace = {
  key: NavSpaceKey;
  label: string;
  items: NavItem[];
};

export const NAV: NavSpace[] = [
  {
    key: "operations",
    label: "Operations",
    items: [
      { to: "/admin", label: "Today", end: true, glyph: "T" },
      {
        to: "/admin/trips",
        label: "Trips",
        glyph: "R",
        children: [{ to: "/admin/import", label: "Import", adminOnly: true }],
      },
      { to: "/admin/inventory", label: "Inventory", adminOnly: true, glyph: "I" },
    ],
  },
  {
    key: "configuration",
    label: "Configuration",
    items: [
      {
        to: "/admin/organization",
        label: "Organization",
        adminOnly: true,
        glyph: "O",
        children: [
          { to: "/admin/organization/payments", label: "Payments", adminOnly: true },
          { to: "/admin/organization/pricing", label: "Pricing", adminOnly: true },
        ],
      },
      { to: "/admin/fleet", label: "Fleet", adminOnly: true, glyph: "F" },
      { to: "/admin/users", label: "Users", adminOnly: true, glyph: "U" },
    ],
  },
  {
    key: "insights",
    label: "Insights",
    items: [
      { to: "/admin/reports", label: "Reports", adminOnly: true, glyph: "P" },
      { to: "/admin/audit", label: "Audit", glyph: "A" },
    ],
  },
];

// filterNavForRole drops adminOnly items (and their children)
// when the viewer isn't an org_admin. Returns spaces that have
// at least one surviving item; empty spaces disappear.
export function filterNavForRole(nav: NavSpace[], isAdmin: boolean): NavSpace[] {
  return nav
    .map((space) => ({
      ...space,
      items: space.items
        .filter((item) => !item.adminOnly || isAdmin)
        .map((item) => ({
          ...item,
          children: (item.children ?? []).filter((c) => !c.adminOnly || isAdmin),
        })),
    }))
    .filter((space) => space.items.length > 0);
}

// flattenNav returns every NavItem (and its children) in order.
// CommandBar uses this for the ⌘K route launcher.
export function flattenNav(nav: NavSpace[]): NavItem[] {
  const out: NavItem[] = [];
  for (const space of nav) {
    for (const item of space.items) {
      out.push(item);
      for (const child of item.children ?? []) {
        out.push(child);
      }
    }
  }
  return out;
}
