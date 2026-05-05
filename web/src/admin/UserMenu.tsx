import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { api, type ApiError } from "../lib/api";
import type { Me } from "./useMe";

// UserMenu is the sidebar-footer trigger + popover. Clicking the name
// opens a small menu with Profile + Sign out. A separate always-visible
// Sign out button (rendered in Shell.tsx alongside this component)
// shares the same logout handler via the onSignOut prop, so the
// `submitting` state disables both paths in lockstep.
//
// On a 2xx logout response the SPA navigates to /login with
// replace:true so the back button can't restore /admin/*. On failure
// the user stays in the shell and an inline error appears in the menu.

export type UserMenuProps = {
  me: Me;
  signingOut: boolean;
  signOutError: string | null;
  onSignOut: () => Promise<void>;
};

export function UserMenu({ me, signingOut, signOutError, onSignOut }: UserMenuProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [open, setOpen] = useState(false);

  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);

  // Close on route change so navigation never leaves a hanging menu.
  useEffect(() => {
    setOpen(false);
  }, [location.pathname]);

  // Outside-click + Escape — only attached while open.
  useEffect(() => {
    if (!open) return;
    const onMouseDown = (e: MouseEvent) => {
      const t = e.target as Node | null;
      if (
        t &&
        !triggerRef.current?.contains(t) &&
        !menuRef.current?.contains(t)
      ) {
        setOpen(false);
      }
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    document.addEventListener("mousedown", onMouseDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const goProfile = () => {
    setOpen(false);
    navigate("/admin/account");
  };

  const handleSignOut = async () => {
    await onSignOut();
    // Menu closes implicitly on success (route change). On failure we
    // leave it open so the user sees the inline error.
  };

  return (
    <div className="user-menu-wrap">
      <button
        ref={triggerRef}
        type="button"
        className="admin-sidebar__trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        disabled={signingOut}
      >
        <span className="admin-sidebar__trigger-name">
          {me.full_name}
          <span className="admin-sidebar__chevron" aria-hidden>▾</span>
        </span>
        <span className="admin-sidebar__trigger-meta">
          {me.email} · {me.role.replace("_", " ")}
        </span>
      </button>

      {open && (
        <div ref={menuRef} className="user-menu" role="menu">
          <button
            type="button"
            className="user-menu__item"
            role="menuitem"
            onClick={goProfile}
            disabled={signingOut}
          >
            Profile
          </button>
          <button
            type="button"
            className="user-menu__item"
            role="menuitem"
            onClick={handleSignOut}
            disabled={signingOut}
          >
            {signingOut ? "Signing out…" : "Sign out"}
          </button>
          {signOutError && (
            <div className="user-menu__error" role="alert">
              {signOutError}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// useSignOut is the shared logout state machine. Shell.tsx instantiates
// it once and passes the result to UserMenu and the standalone Sign
// out button so both routes share `submitting` and `error`.
export function useSignOut() {
  const navigate = useNavigate();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const signOut = async () => {
    if (submitting) return;
    setError(null);
    setSubmitting(true);
    try {
      await api.logout();
      navigate("/login", { replace: true });
    } catch (e) {
      const apiErr = e as ApiError;
      setError(apiErr?.message ?? "Could not sign out. Please try again.");
      setSubmitting(false);
    }
  };

  return { submitting, error, signOut };
}
