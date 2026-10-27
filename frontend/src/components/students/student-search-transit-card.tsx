"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";
import { TransitAssignmentCard } from "~/components/rooms/transit-assignment-card";
import type { Student } from "~/lib/api";
import { hasPermission } from "~/lib/auth-utils";
import { isTransitLocation } from "~/lib/location-helper";
import { useAttendanceWebEnabled, usePresenceMode } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";

const TRANSIT_PATH = "/rooms/unterwegs";

/**
 * Die Karte „Unterwegs“ auf „Alle Kinder“ (#5026): Wer nach Unterwegs filtert
 * oder gruppiert, kommt von hier zur Zuweisung, statt vor einer Liste ohne
 * Aktion zu stehen. Sie erscheint nur, wo die Zuweisung auch gelingen kann:
 * mit `visits:update` (dasselbe Recht wie POST
 * /api/active/visits/transit/assign), mit Räumen (nicht im Binärmodus) und
 * mit Anwesenheit über die App.
 */
export function StudentSearchTransitCard({
  students,
  show,
  fromParam,
}: {
  /** Die sichtbaren Kinder der Liste; gezählt werden die unterwegs. */
  readonly students: readonly Student[];
  /** Die Liste filtert oder gruppiert gerade nach Unterwegs (nur heute). */
  readonly show: boolean;
  /** Der bereits kodierte Rückweg zu „Alle Kinder“ mit ihren Filtern. */
  readonly fromParam: string;
}) {
  const { data: session } = useSession();
  const presenceMode = usePresenceMode();
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const tenantPath = useTenantAwarePath();
  const transitCount = useMemo(
    () =>
      students.filter((student) => isTransitLocation(student.current_location))
        .length,
    [students],
  );

  if (
    !show ||
    transitCount === 0 ||
    presenceMode === "binary" ||
    !attendanceWebEnabled ||
    !hasPermission(session, "visits:update")
  ) {
    return null;
  }

  return (
    <TransitAssignmentCard
      count={transitCount}
      href={tenantPath(`${TRANSIT_PATH}?from=${fromParam}`)}
      className="mb-3"
    />
  );
}
