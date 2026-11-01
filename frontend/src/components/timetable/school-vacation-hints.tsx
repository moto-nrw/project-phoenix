"use client";

/**
 * Ferienhinweis (#5345) mit der Sammelabsage (#3594) auf derselben Seite:
 * Betreuungsplan und „Schuljahr und Kalender“. Die Startseite nutzt
 * `SchoolVacationHintsCore` ohne Dialog, siehe dort.
 */

import { useState } from "react";

import { BulkCancelAppointmentsModal } from "~/components/timetable/bulk-cancel-appointments-modal";
import {
  SchoolVacationHintsCore,
  type SchoolVacationHintsCoreProps,
} from "~/components/timetable/school-vacation-hints-core";
import type { SchoolVacationHint } from "~/lib/school-vacations-api";

export { vacationHintMessage } from "~/components/timetable/school-vacation-hints-core";

interface SchoolVacationHintsProps extends Omit<
  SchoolVacationHintsCoreProps,
  "onCancel"
> {
  /** Nach „Termine absagen“, z. B. um Zählungen der Seite neu zu laden. */
  onAppointmentsCancelled?: () => void;
}

export function SchoolVacationHints({
  onAppointmentsCancelled,
  ...props
}: Readonly<SchoolVacationHintsProps>) {
  const [cancelling, setCancelling] = useState<SchoolVacationHint | null>(null);
  return (
    <>
      <SchoolVacationHintsCore {...props} onCancel={setCancelling} />
      {/* Die Sammelabsage lädt Plan und Ferienhinweis selbst neu. */}
      {cancelling ? (
        <BulkCancelAppointmentsModal
          isOpen
          initialFrom={cancelling.cancelFrom}
          initialTo={cancelling.endDate}
          onClose={() => setCancelling(null)}
          onCancelled={() => onAppointmentsCancelled?.()}
        />
      ) : null}
    </>
  );
}
