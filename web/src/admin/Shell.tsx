import { useEffect } from "react";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router-dom";

import { adminApi } from "./api";
import { SpacesNav } from "./components";
import { filterNavForRole, NAV } from "./nav";
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
        // Non-fatal.
      });
    return () => {
      cancelled = true;
    };
  }, [isAdmin, location.pathname, navigate]);
}

/**
 * AdminShell is the persistent chrome for the admin / Cruise
 * Director surface. Sprint 027 collapses Triptych to one
 * production shell. Role filtering happens once on the canonical
 * NAV; the cockpit and remaining pages share the same chrome.
 */
export function AdminShell() {
  const me = useMe();
  const { submitting, error, signOut } = useSignOut();

  // Hooks must run on every render in the same order — call
  // this BEFORE any early return. The hook itself gates on
  // isAdmin so a pre-load `false` is a safe no-op.
  const isAdmin = me.loaded && me.me?.role === "org_admin";
  useOnboardingAutoShow(isAdmin);

  if (!me.loaded) {
    return null;
  }
  if (me.error || !me.me) {
    return <Navigate to="/login" replace />;
  }

  const filteredNav = filterNavForRole(NAV, isAdmin);
  const footer = (
    <>
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
    </>
  );

  return (
    <div className="admin admin--cockpit">
      <SpacesNav nav={filteredNav} brand="Liveaboard" footer={footer} />
      <main className="admin-main">
        <Outlet />
      </main>
    </div>
  );
}

/**
 * RequireAdmin guards routes that only Org Admins should see. A
 * Cruise Director hitting one of these URLs gets bounced to
 * /admin (their Overview). The API itself ALSO 403s these
 * requests — this is a UX nicety, not the security boundary.
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
