import type { CockpitActivity } from "../CockpitApi";

import styles from "../Cockpit.module.css";

export function ActivityStream({ activity }: { activity: CockpitActivity[] }) {
  return (
    <section className={styles.activityStream}>
      <div className={styles.panelHeader}>
        <span>Activity</span>
        <strong>{activity.length}</strong>
      </div>
      {activity.length === 0 ? (
        <p className={styles.quiet}>No activity yet.</p>
      ) : (
        <ol>
          {activity.map((event) => (
            <li key={event.id}>
              <strong>{event.action.replaceAll("_", " ")}</strong>
              <span>{event.entity.replaceAll("_", " ")}</span>
              <time>{new Date(event.created_at).toLocaleString()}</time>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
