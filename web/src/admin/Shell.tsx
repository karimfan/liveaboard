import { NavLink, Outlet, useLocation, Navigate } from "react-router-dom";

import { useMe } from "./useMe";

type NavItem = { to: string; label: string; end?: boolean; adminOnly?: boolean };

const navItems: NavItem[] = [
  { to: "/admin", label: "Overview", end: true },
  { to: "/admin/organization", label: "Organization", adminOnly: true },
  { to: "/admin/fleet", label: "Fleet", adminOnly: true },
  { to: "/admin/catalog", label: "Catalog", adminOnly: true },
  { to: "/admin/trips", label: "Trips" },
  { to: "/admin/users", label: "Users", adminOnly: true },
  { to: "/admin/reports", label: "Reports", adminOnly: true },
];

/**
 * AdminShell is the persistent chrome for the admin / Site Director
 * surface. The matched child route renders inside <Outlet />. Sidebar
 * items adjust by role: an Org Admin sees all 7; a Site Director sees
 * Overview + Trips only.
 */
export function AdminShell() {
  const me = useMe();

  if (!me.loaded) {
    return null; // brief flash; preferable to a spinner for Sprint 008
  }
  if (me.error || !me.me) {
    return <Navigate to="/login" replace />;
  }

  const visible = navItems.filter((n) => !n.adminOnly || me.me!.role === "org_admin");

  return (
    <div className="admin">
      <aside className="admin-sidebar">
        <div className="admin-sidebar__brand">Liveaboard</div>
        <nav className="admin-nav">
          {visible.map((item) => (
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
          <div className="admin-sidebar__org">{me.me.full_name}</div>
          <div className="admin-sidebar__user">
            {me.me.email} · {me.me.role.replace("_", " ")}
          </div>
        </div>
      </aside>
      <main className="admin-main">
        <Outlet />
      </main>
    </div>
  );
}

/**
 * RequireAdmin guards routes that only Org Admins should see. A Site
 * Director hitting one of these URLs gets bounced to /admin (their
 * Overview). The API itself ALSO 403s these requests — this is a UX
 * nicety, not the security boundary.
 */
export function RequireAdmin({ children }: { children: React.ReactNode }) {
  const me = useMe();
  const location = useLocation();
  if (!me.loaded) return null;
  if (!me.me || me.me.role !== "org_admin") {
    return <Navigate to="/admin" replace state={{ from: location.pathname }} />;
  }
  return <>{children}</>;
}
