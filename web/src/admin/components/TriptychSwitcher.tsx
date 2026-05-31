import { useEffect, useState } from "react";

import { LAYOUTS, MOTIONS, PALETTES, useDesignMode } from "../design";
import { useDevFlags } from "../../lib/devFlags";

import styles from "./TriptychSwitcher.module.css";

// Sprint 025 — floating bottom-right design-mode switcher.
// Renders ONLY when useDevFlags().ui_redesign_switcher is
// true (dev mode). Production builds see an empty render.
// Three segmented controls for palette / layout / motion, plus
// a "Copy ?triptych=" share-link button.
export function TriptychSwitcher() {
  const flags = useDevFlags();
  const { mode, setPalette, setLayout, setMotion } = useDesignMode();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const t = window.setTimeout(() => setCopied(false), 1200);
    return () => window.clearTimeout(t);
  }, [copied]);

  if (!flags.ui_redesign_switcher) return null;

  function copyShare() {
    const url = new URL(window.location.href);
    url.searchParams.set("triptych", `${mode.palette},${mode.layout},${mode.motion}`);
    void navigator.clipboard.writeText(url.toString()).then(() => setCopied(true));
  }

  return (
    <aside className={styles.dock} aria-label="Triptych design switcher">
      <Row label="Palette">
        {PALETTES.map((p) => (
          <Pill key={p} active={p === mode.palette} onClick={() => setPalette(p)}>
            {p}
          </Pill>
        ))}
      </Row>
      <Row label="Layout">
        {LAYOUTS.map((l) => (
          <Pill key={l} active={l === mode.layout} onClick={() => setLayout(l)}>
            {l}
          </Pill>
        ))}
      </Row>
      <Row label="Motion">
        {MOTIONS.map((m) => (
          <Pill key={m} active={m === mode.motion} onClick={() => setMotion(m)}>
            {m}
          </Pill>
        ))}
      </Row>
      <button type="button" className={styles.copy} onClick={copyShare}>
        {copied ? "Copied!" : "Copy ?triptych= link"}
      </button>
    </aside>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className={styles.row}>
      <span className={styles.rowLabel}>{label}</span>
      <div className={styles.pills}>{children}</div>
    </div>
  );
}

function Pill({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={active ? `${styles.pill} ${styles.active}` : styles.pill}
      aria-pressed={active}
    >
      {children}
    </button>
  );
}
