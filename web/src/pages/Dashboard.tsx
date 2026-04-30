import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import { api } from "../lib/api";

type Org = {
  id: string;
  name: string;
  currency: string | null;
  created_at: string;
  stats: { boats: number; active_trips: number; total_guests: number };
};

type Me = {
  email: string;
  full_name: string;
  role: string;
};

export function Dashboard() {
  const navigate = useNavigate();
  const [org, setOrg] = useState<Org | null>(null);
  const [me, setMe] = useState<Me | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.organization(), api.me()])
      .then(([o, u]) => {
        setOrg(o);
        setMe(u);
      })
      .catch(() => setError("Failed to load dashboard."));
  }, []);

  async function logout() {
    await api.logout();
    navigate("/login");
  }

  if (error) return <div className="dashboard"><div className="error">{error}</div></div>;
  if (!org || !me) return <div className="dashboard">Loading…</div>;

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <div>
          <div className="brand">{org.name}</div>
          <div className="muted">{me.full_name} · {me.role.replace("_", " ")}</div>
        </div>
        <button className="ghost" onClick={logout}>Log out</button>
      </div>

      <div className="stats">
        <div className="stat-card">
          <div className="stat-label">Boats</div>
          <div className="stat-value">{org.stats.boats}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Active trips</div>
          <div className="stat-value">{org.stats.active_trips}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Guests onboard</div>
          <div className="stat-value">{org.stats.total_guests}</div>
        </div>
      </div>

      <p className="muted" style={{ marginTop: 32 }}>
        Fleet, catalog, and trip management arrive in upcoming sprints.
      </p>
    </div>
  );
}
