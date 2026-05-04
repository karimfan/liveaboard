import { organization } from "../mock";

export function Organization() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Organization</h1>
          <div className="admin-page-subtitle">
            Org profile, currency, and defaults.
          </div>
        </div>
      </div>

      <div className="admin-card" style={{ maxWidth: 560 }}>
        <h2 className="admin-card__title">Profile</h2>
        <div className="field">
          <label htmlFor="org-name">Organization name</label>
          <input id="org-name" type="text" defaultValue={organization.name} />
        </div>
        <div className="field">
          <label htmlFor="org-currency">Default currency</label>
          <select id="org-currency" defaultValue={organization.currency}>
            <option value="USD">USD — US dollar</option>
            <option value="EUR">EUR — Euro</option>
            <option value="AUD">AUD — Australian dollar</option>
            <option value="GBP">GBP — British pound</option>
          </select>
          <div className="muted" style={{ marginTop: "var(--sp-xs)" }}>
            Multi-currency support is on the roadmap; today an org has one default.
          </div>
        </div>
        <div className="field">
          <label>Created</label>
          <div className="muted">{organization.createdAt}</div>
        </div>
        <button className="primary" style={{ marginTop: "var(--sp-sm)" }}>
          Save
        </button>
      </div>
    </>
  );
}
