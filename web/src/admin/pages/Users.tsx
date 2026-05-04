import { users } from "../mock";

export function Users() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Users</h1>
          <div className="admin-page-subtitle">
            Org Admins, Site Directors, and pending invitations.
          </div>
        </div>
        <button className="primary">+ Invite</button>
      </div>

      <div className="filter-bar">
        <select defaultValue="active">
          <option value="active">Active</option>
          <option value="pending">Pending invites</option>
          <option value="deactivated">Deactivated</option>
          <option value="all">All</option>
        </select>
        <input type="search" placeholder="Search name or email..." />
        <div className="filter-bar__spacer" />
      </div>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Role</th>
            <th>Status</th>
            <th>Notes</th>
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.id} className="is-clickable">
              <td>{u.fullName}</td>
              <td>{u.email}</td>
              <td>{u.role}</td>
              <td>
                {u.status === "active" ? (
                  <span className="chip chip--active">Active</span>
                ) : u.status === "pending invite" ? (
                  <span className="chip chip--warn">Pending</span>
                ) : (
                  <span className="chip chip--archived">Deactivated</span>
                )}
              </td>
              <td className="muted">
                {u.status === "pending invite" ? `Invited ${u.invitedAt}` : ""}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
