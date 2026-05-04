import { NavLink, Outlet } from "react-router-dom";

import { organization } from "./mock";

const navItems: { to: string; label: string; end?: boolean }[] = [
  { to: "/admin", label: "Overview", end: true },
  { to: "/admin/organization", label: "Organization" },
  { to: "/admin/fleet", label: "Fleet" },
  { to: "/admin/catalog", label: "Catalog" },
  { to: "/admin/trips", label: "Trips" },
  { to: "/admin/users", label: "Users" },
  { to: "/admin/reports", label: "Reports" },
];

/**
 * AdminShell is the persistent chrome for the Sprint 007 Option A
 * "Control Tower" admin experience. The matched child route renders
 * inside <Outlet />.
 *
 * All data displayed here is hardcoded mock data (web/src/admin/mock.ts).
 * A future implementation sprint replaces these with /api/admin/* calls.
 */
export function AdminShell() {
  return (
    <div className="admin">
      <aside className="admin-sidebar">
        <div className="admin-sidebar__brand">Liveaboard</div>
        <nav className="admin-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                "admin-nav__link" + (isActive ? " is-active" : "")
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="admin-sidebar__footer">
          <div className="admin-sidebar__org">{organization.name}</div>
          <div className="admin-sidebar__user">owner@acme.test</div>
        </div>
      </aside>
      <main className="admin-main">
        <div className="mockup-banner">
          MOCKUP — Sprint 007 Option A. Data is hardcoded; navigation is real.
        </div>
        <Outlet />
      </main>
    </div>
  );
}
