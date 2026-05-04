import { Link, NavLink, Outlet, useParams } from "react-router-dom";

import { boats, inventoryByBoat, trips } from "../mock";

export function BoatDetail() {
  const { slug } = useParams();
  const boat = boats.find((b) => b.slug === slug);

  if (!boat) {
    return (
      <div className="empty-state">
        <h3>Boat not found</h3>
        <p>
          <Link to="/admin/fleet">Back to fleet</Link>
        </p>
      </div>
    );
  }

  return (
    <>
      <div className="admin-breadcrumb">
        <Link to="/admin/fleet">Fleet</Link> · {boat.name}
      </div>

      <div className="boat-detail-header">
        {boat.imageUrl ? (
          <img className="boat-detail-header__image" src={boat.imageUrl} alt={boat.name} />
        ) : (
          <div className="boat-detail-header__image" />
        )}
        <div>
          <h1 className="boat-detail-header__name">{boat.name}</h1>
          <div className="boat-detail-header__source">
            {boat.sourceUrl ? (
              <a href={boat.sourceUrl} target="_blank" rel="noreferrer">
                {boat.sourceUrl.replace(/^https?:\/\//, "")}
              </a>
            ) : (
              "(no source linkage)"
            )}
            {" · last synced "}
            {boat.lastSyncedAt}
          </div>
          <div className="boat-detail-header__stats">
            <div>
              <div className="boat-stat__label">Capacity</div>
              <div className="boat-stat__value">{boat.capacity}</div>
            </div>
            <div>
              <div className="boat-stat__label">Upcoming trips</div>
              <div className="boat-stat__value">{boat.upcomingTrips}</div>
            </div>
            <div>
              <div className="boat-stat__label">Status</div>
              <div className="boat-stat__value">{boat.status}</div>
            </div>
          </div>
        </div>
      </div>

      <nav className="tabs">
        <NavLink
          end
          to={`/admin/fleet/${boat.slug}`}
          className={({ isActive }) => "tabs__link" + (isActive ? " is-active" : "")}
        >
          Trips
        </NavLink>
        <NavLink
          to={`/admin/fleet/${boat.slug}/inventory`}
          className={({ isActive }) => "tabs__link" + (isActive ? " is-active" : "")}
        >
          Inventory
        </NavLink>
        <NavLink
          to={`/admin/fleet/${boat.slug}/notes`}
          className={({ isActive }) => "tabs__link" + (isActive ? " is-active" : "")}
        >
          Notes
        </NavLink>
      </nav>

      <Outlet context={{ boat, inventory: inventoryByBoat[boat.id] ?? [], trips: trips.filter((t) => t.boatId === boat.id) }} />
    </>
  );
}
