"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";

/** The two status filters a child without check-in falls into. */
export type MissingCheckinStatus = "schule" | "abwesend";

export interface MissingCheckinStudent {
  readonly id: string;
  readonly name: string;
  readonly status: MissingCheckinStatus;
}

// Spelled as the page's „Status“ filter spells its options.
const STATUS_FILTER_LABELS: Record<MissingCheckinStatus, string> = {
  schule: "Schule",
  abwesend: "Zuhause",
};

const MAX_NAMED = 6;

function namesLine(students: readonly MissingCheckinStudent[]): string {
  const named = students.slice(0, MAX_NAMED).map((student) => student.name);
  const rest = students.length - named.length;
  return rest > 0
    ? `${named.join(", ")} und ${rest} weitere`
    : named.join(", ");
}

/**
 * Shown after a bulk Anmelden (#4762): which children of the list on screen
 * are still not checked in, with one button per status filter that lists
 * them. Children who are sick, excused or not expected today are left out:
 * nobody waits for them.
 */
export function SchoolCheckinMissingHint({
  students,
  onShowStatus,
  onDismiss,
}: Readonly<{
  students: readonly MissingCheckinStudent[];
  onShowStatus: (status: MissingCheckinStatus) => void;
  onDismiss: () => void;
}>) {
  if (students.length === 0) return null;
  const statuses = (["schule", "abwesend"] as const).filter((status) =>
    students.some((student) => student.status === status),
  );

  return (
    <Alert
      type="info"
      className="mb-3"
      title={`Noch nicht angemeldet: ${students.length} ${students.length === 1 ? "Kind" : "Kinder"}`}
      message={namesLine(students)}
      actionLayout="stacked"
      action={
        <div className="flex flex-wrap gap-2">
          {statuses.map((status) => (
            <Button
              key={status}
              type="button"
              variant="secondary"
              size="compact"
              onClick={() => onShowStatus(status)}
            >
              Nur „{STATUS_FILTER_LABELS[status]}“ zeigen
            </Button>
          ))}
          <Button
            type="button"
            variant="ghost"
            size="compact"
            className="shadow-none"
            onClick={onDismiss}
          >
            Ausblenden
          </Button>
        </div>
      }
    />
  );
}
