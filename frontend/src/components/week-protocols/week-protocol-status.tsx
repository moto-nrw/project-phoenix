import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
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

/** „06.09. bis 12.09.2027“ */
export function weekProtocolWeekLabel(protocol: {
  readonly weekStart: string;
  readonly weekEnd: string;
}): string {
  return `${formatDate(protocol.weekStart).slice(0, 6)} bis ${formatDate(protocol.weekEnd)}`;
}

/** Der Stand eines Blocks am Abend seines Tages, wie im PDF. */
export function weekProtocolBlockStatus(block: WeekProtocolBlock): string {
  switch (block.status) {
    case "completed":
      return "Gelaufen";
    case "cancelled":
      return block.cancelReason
        ? `Ausgefallen: ${block.cancelReason}`
        : "Ausgefallen";
    case "active":
      return "Nicht beendet";
    default:
      return "Nicht gestartet";
  }
}
