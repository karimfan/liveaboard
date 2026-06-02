import { useEffect, useState, type FormEvent } from "react";

import { adminApi, type AdminUser } from "../api";
import { api, type ApiError, type Invitation } from "../../lib/api";
import {
  Button,
  Chip,
  type Column,
  DataTable,
  Empty,
  Field,
  PageHeader,
} from "../components";

import styles from "./Users.module.css";

export function Users() {
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [invites, setInvites] = useState<Invitation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showInvite, setShowInvite] = useState(false);

  async function refresh() {
    try {
      const [u, inv] = await Promise.all([
        adminApi.listUsers(),
        api.listInvitations(),
      ]);
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

  const inviteColumns: Column<Invitation>[] = [
    { key: "name", header: "Name", cell: (inv) => inv.full_name },
    { key: "email", header: "Email", cell: (inv) => inv.email },
    { key: "phone", header: "Phone", cell: (inv) => inv.phone ?? "—" },
    { key: "role", header: "Role", cell: (inv) => inv.role.replace("_", " ") },
    {
      key: "expires",
      header: "Expires",
      cell: (inv) => new Date(inv.expires_at).toLocaleString(),
    },
    {
      key: "actions",
      header: "Actions",
      cell: (inv) => (
        <div className={styles.rowActions}>
          <Button variant="quiet" onClick={() => onResend(inv.id)}>
            Resend
          </Button>
          <Button variant="quiet" onClick={() => onRevoke(inv.id)}>
            Revoke
          </Button>
        </div>
      ),
    },
  ];

  const userColumns: Column<AdminUser>[] = [
    { key: "name", header: "Name", cell: (u) => u.full_name },
    { key: "email", header: "Email", cell: (u) => u.email },
    { key: "phone", header: "Phone", cell: (u) => u.phone ?? "—" },
    { key: "role", header: "Role", cell: (u) => u.role.replace("_", " ") },
    {
      key: "status",
      header: "Status",
      cell: (u) => (
        <Chip variant={u.is_active ? "success" : "neutral"}>
          {u.is_active ? "Active" : "Deactivated"}
        </Chip>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Users"
        subtitle="Org Admins and Cruise Directors in your organization."
        actions={
          <Button variant="primary" onClick={() => setShowInvite(true)}>
            + Invite Cruise Director
          </Button>
        }
      />

      {error && <div className={styles.error}>{error}</div>}

      {invites.length > 0 && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Pending invitations</h2>
          <DataTable
            columns={inviteColumns}
            rows={invites}
            rowKey={(inv) => inv.id}
          />
        </section>
      )}

      {!users ? (
        <div className={styles.loading}>Loading…</div>
      ) : users.length === 0 ? (
        <Empty title="No users yet" />
      ) : (
        <DataTable columns={userColumns} rows={users} rowKey={(u) => u.id} />
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
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api.invite({
        email,
        full_name: fullName,
        phone: phone.trim() || undefined,
      });
      onCreated();
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Could not send invitation.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <h2 className={styles.modalTitle}>Invite a Cruise Director</h2>
        <form className={styles.form} onSubmit={onSubmit}>
          {error && <div className={styles.error}>{error}</div>}
          <Field label="Full name" htmlFor="invname">
            <input
              id="invname"
              type="text"
              autoComplete="name"
              autoFocus
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              required
            />
          </Field>
          <Field label="Email" htmlFor="invemail">
            <input
              id="invemail"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </Field>
          <Field label="Phone (optional)" htmlFor="invphone">
            <input
              id="invphone"
              type="tel"
              autoComplete="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
            />
          </Field>
          <Field label="Role">
            <div className={styles.readonly}>Cruise Director</div>
          </Field>
          <div className={styles.modalActions}>
            <Button type="button" variant="quiet" onClick={onClose}>
              Cancel
            </Button>
            <Button variant="primary" type="submit" disabled={submitting}>
              {submitting ? "Sending…" : "Send invitation"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
