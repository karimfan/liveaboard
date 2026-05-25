import { useEffect } from "react";
import { NavLink, Outlet, useLocation, Navigate, useNavigate } from "react-router-dom";

import { adminApi } from "./api";
import { useMe } from "./useMe";
import { UserMenu, useSignOut } from "./UserMenu";

// First-run auto-show: when a new Org Admin lands on /admin and their
// onboarding wizard is neither complete nor dismissed, navigate them
// to /admin/onboarding. One-shot per session — sessionStorage guards
// against re-redirecting after the admin clicks back.
function useOnboardingAutoShow(isAdmin: boolean) {
  const navigate = useNavigate();
  const location = useLocation();
  useEffect(() => {
    if (!isAdmin) return;
    if (location.pathname !== "/admin") return;
    if (sessionStorage.getItem("onboarding_auto_shown") === "1") return;
    let cancelled = false;
    adminApi
      .onboarding()
      .then((state) => {
        if (cancelled) return;
        sessionStorage.setItem("onboarding_auto_shown", "1");
        if (!state.dismissed_at && !state.onboarding_complete) {
          navigate("/admin/onboarding", { replace: true });
        }
      })
      .catch(() => {
        // Non-fatal: if the fetch fails (e.g. transient), don't pin
        // the flag, so the next visit will retry.
      });
    return () => {
      cancelled = true;
    };
  }, [isAdmin, location.pathname, navigate]);
}

type NavItem = {
  to: string;
  label: string;
  end?: boolean;
  adminOnly?: boolean;
  children?: NavItem[];
};

const navItems: NavItem[] = [
  { to: "/admin", label: "Overview", end: true },
  {
    to: "/admin/organization",
    label: "Organization",
    adminOnly: true,
    children: [
      { to: "/admin/organization/payments", label: "Payments", adminOnly: true },
      { to: "/admin/organization/pricing", label: "Pricing", adminOnly: true },
    ],
  },
  { to: "/admin/audit", label: "Audit" },
  {
    to: "/admin/fleet",
    label: "Fleet",
    adminOnly: true,
    children: [
      { to: "/admin/inventory", label: "Inventory", adminOnly: true },
    ],
  },
  {
    to: "/admin/trips",
    label: "Trips",
    children: [
      { to: "/admin/import", label: "Import", adminOnly: true },
    ],
  },
  { to: "/admin/users", label: "Users", adminOnly: true },
  { to: "/admin/reports", label: "Reports", adminOnly: true },
];

/**
 * AdminShell is the persistent chrome for the admin / Cruise Director
 * surface. The matched child route renders inside <Outlet />. Sidebar
 * items adjust by role: an Org Admin sees all 7; a Cruise Director sees
 * Overview + Trips only.
 */
export function AdminShell() {
  const me = useMe();
  // Logout state is owned at the Shell level so the popover item and
  // the standalone footer button share the same `submitting` and
  // `error`. Both call the same `signOut()` and the disabled state
  // stays consistent.
  const { submitting, error, signOut } = useSignOut();

  // Hooks must run on every render in the same order — call this
  // BEFORE any early return. The hook itself gates on isAdmin so a
  // pre-load `false` is a safe no-op.
  const isAdmin = me.loaded && me.me?.role === "org_admin";
  useOnboardingAutoShow(isAdmin);

  if (!me.loaded) {
    return null; // brief flash; preferable to a spinner for Sprint 008
  }
  if (me.error || !me.me) {
    return <Navigate to="/login" replace />;
  }
  // Filter the parents the role can see; for each parent that
  // survives, also filter its children. A child whose adminOnly
  // would hide it disappears even if the parent stays visible.
  const visible = navItems
    .filter((n) => !n.adminOnly || isAdmin)
    .map((n) => ({
      ...n,
      children: (n.children ?? []).filter((c) => !c.adminOnly || isAdmin),
    }));

  return (
    <div className="admin">
      <aside className="admin-sidebar">
        <div className="admin-sidebar__brand">Liveaboard</div>
        <nav className="admin-nav">
          {visible.map((item) => (
            <div key={item.to} className="admin-nav__group">
              <NavLink
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  "admin-nav__link" + (isActive ? " is-active" : "")
                }
              >
                {item.label}
              </NavLink>
              {item.children.map((child) => (
                <NavLink
                  key={child.to}
                  to={child.to}
                  className={({ isActive }) =>
                    "admin-nav__link admin-nav__link--child" +
                    (isActive ? " is-active" : "")
                  }
                >
                  {child.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="admin-sidebar__footer">
          <UserMenu
            me={me.me}
            signingOut={submitting}
            signOutError={error}
            onSignOut={signOut}
          />
          <button
            type="button"
            className="admin-sidebar__signout"
            onClick={signOut}
            disabled={submitting}
          >
            {submitting ? "Signing out…" : "Sign out"}
          </button>
        </div>
      </aside>
      <main className="admin-main">
        <Outlet />
      </main>
    </div>
  );
}

/**
 * RequireAdmin guards routes that only Org Admins should see. A Cruise
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
