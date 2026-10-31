import { StatusBadge } from "~/components/ui/status-badge";
import { formatWeekLabel, parseISODate } from "~/lib/date-helpers";
import type {
  WeekProtocol,
  WeekProtocolBlock,
  WeekProtocolStatus,
} from "~/lib/week-protocols-api";

// Die Wörter des Wochenprotokolls (#4769), geteilt von Liste und
// Protokollseite. Die Stände und Blockzustände sagt das PDF genauso.

const STATUS: Record<
  WeekProtocolStatus,
  { label: string; tone: "gray" | "orange" | "green" }
> = {
  collecting: { label: "Woche läuft", tone: "gray" },
  draft: { label: "Entwurf", tone: "orange" },
  closed: { label: "Abgeschlossen", tone: "green" },
};

export function WeekProtocolStatusBadge({
  protocol,
}: Readonly<{ protocol: Pick<WeekProtocol, "status"> }>) {
  const status = STATUS[protocol.status];
  return <StatusBadge label={status.label} tone={status.tone} />;
}

/** „KW 36 · 06.09.–12.09.2027“, wie der Wochenkopf im Kalender. */
export function weekProtocolWeekLabel(protocol: {
  readonly weekStart: string;
  readonly weekEnd: string;
}): string {
  return formatWeekLabel(
    parseISODate(protocol.weekStart),
    parseISODate(protocol.weekEnd),
  );
}

const BLOCK_STATUS: Record<
  string,
  { label: string; tone: "gray" | "orange" | "green" | "red" }
> = {
  completed: { label: "Gelaufen", tone: "green" },
  cancelled: { label: "Ausgefallen", tone: "red" },
  active: { label: "Nicht beendet", tone: "orange" },
};

/** Der Stand eines Blocks am Abend seines Tages, wie im PDF. */
export function WeekProtocolBlockBadge({
  block,
}: Readonly<{ block: Pick<WeekProtocolBlock, "status"> }>) {
  const status = BLOCK_STATUS[block.status] ?? {
    label: "Nicht gestartet",
    tone: "gray" as const,
  };
  return <StatusBadge label={status.label} tone={status.tone} />;
}
