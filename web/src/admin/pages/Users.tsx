import { useEffect, useState, type FormEvent } from "react";

import { adminApi, type AdminUser } from "../api";
import { api, type ApiError, type Invitation } from "../../lib/api";

export function Users() {
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [invites, setInvites] = useState<Invitation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showInvite, setShowInvite] = useState(false);

  async function refresh() {
    try {
      const [u, inv] = await Promise.all([adminApi.listUsers(), api.listInvitations()]);
      setUsers(u.users ?? []);
      setInvites(inv.invitations ?? []);
    } catch (e) {
      const apiErr = e as ApiError;
      setError(apiErr?.message ?? "Failed to load users.");
    }
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function onResend(id: string) {
    try {
      await api.resendInvitation(id);
      await refresh();
    } catch (e) {
      const apiErr = e as ApiError;
      setError(apiErr?.message ?? "Could not resend invitation.");
    }
  }

  async function onRevoke(id: string) {
    if (!confirm("Revoke this invitation?")) return;
    try {
      await api.revokeInvitation(id);
      await refresh();
    } catch (e) {
      const apiErr = e as ApiError;
      setError(apiErr?.message ?? "Could not revoke invitation.");
    }
  }

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Users</h1>
          <div className="admin-page-subtitle">
            Org Admins and Site Directors in your organization.
          </div>
        </div>
        <button className="primary" onClick={() => setShowInvite(true)}>
          + Invite
        </button>
      </div>

      {error && <div className="error">{error}</div>}

      {invites.length > 0 && (
        <section style={{ marginBottom: "var(--sp-lg)" }}>
          <h2>Pending invitations</h2>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Email</th>
                <th>Role</th>
                <th>Expires</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {invites.map((inv) => (
                <tr key={inv.id}>
                  <td>{inv.email}</td>
                  <td>{inv.role.replace("_", " ")}</td>
                  <td>{new Date(inv.expires_at).toLocaleString()}</td>
                  <td>
                    <button className="ghost" onClick={() => onResend(inv.id)}>
                      Resend
                    </button>
                    <button className="ghost" onClick={() => onRevoke(inv.id)}>
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}

      {!users ? (
        <div className="muted">Loading…</div>
      ) : users.length === 0 ? (
        <div className="empty-state">
          <h3>No users yet</h3>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Role</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.full_name}</td>
                <td>{u.email}</td>
                <td>{u.role.replace("_", " ")}</td>
                <td>
                  {u.is_active ? (
                    <span className="chip chip--active">Active</span>
                  ) : (
                    <span className="chip chip--archived">Deactivated</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {showInvite && (
        <InviteModal
          onClose={() => setShowInvite(false)}
          onCreated={() => {
            setShowInvite(false);
            void refresh();
          }}
        />
      )}
    </>
  );
}

function InviteModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api.invite(email, "site_director");
      onCreated();
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Could not send invitation.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Invite a Site Director</h2>
        <form onSubmit={onSubmit}>
          {error && <div className="error">{error}</div>}
          <div className="field">
            <label htmlFor="invemail">Email</label>
            <input
              id="invemail"
              type="email"
              autoFocus
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </div>
          <div style={{ display: "flex", gap: "var(--sp-sm)", justifyContent: "flex-end" }}>
            <button type="button" className="ghost" onClick={onClose}>
              Cancel
            </button>
            <button className="primary" type="submit" disabled={submitting}>
              {submitting ? "Sending…" : "Send invitation"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
