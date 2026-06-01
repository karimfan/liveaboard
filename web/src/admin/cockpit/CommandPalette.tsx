import { CommandBar } from "../components";

import type { CockpitCommand } from "./CockpitApi";

export function CommandPalette({
  commands,
  open,
  onClose,
}: {
  commands: CockpitCommand[];
  open: boolean;
  onClose: () => void;
}) {
  return (
    <CommandBar
      open={open}
      onClose={onClose}
      items={commands.map((c) => ({ to: c.route, label: c.label }))}
    />
  );
}
