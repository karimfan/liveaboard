import { useEffect, useState } from "react";

import { adminApi, type AdminUser } from "../api";

export function Users() {
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    adminApi
      .listUsers()
      .then((res) => !cancelled && setUsers(res.users ?? []))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load users."));
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Users</h1>
          <div className="admin-page-subtitle">
            Org Admins and Site Directors in your organization.
          </div>
        </div>
        <button className="primary" disabled title="Coming next sprint">
          + Invite
        </button>
      </div>

      {error && <div className="error">{error}</div>}
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
    </>
  );
}
