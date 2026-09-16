"use client";

import { ConfirmationModal } from "~/components/ui/modal";
import type { TimetableRoster } from "~/lib/timetable-operations-types";

interface CompleteInstanceModalProps {
  readonly isOpen: boolean;
  readonly roster: TimetableRoster | null;
  readonly isCompleting: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
}

/**
 * Asks before a running block ends and names the children still present.
 * One per roster: the page shows it for its own session, and every block of a
 * released room shows its own (#3281).
 */
export function CompleteInstanceModal({
  isOpen,
  roster,
  isCompleting,
  onClose,
  onConfirm,
}: CompleteInstanceModalProps) {
  const present = roster?.rows.filter((row) => row.currentlyPresent) ?? [];
  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Aktivität wirklich beenden?"
      confirmText="Aktivität beenden"
      isConfirmLoading={isCompleting}
      isDismissDisabled={isCompleting}
    >
      <div className="space-y-3 text-sm text-gray-700">
        <p>
          <strong>{roster?.instance.title}</strong> endet laut Plan um{" "}
          {roster?.instance.endTime} Uhr.
        </p>
        <p>Aktuell anwesend: {present.length}</p>
        {present.length > 0 ? (
          <ul className="list-disc space-y-1 pl-5">
            {present.map((row) => (
              <li key={row.studentId}>{row.studentName}</li>
            ))}
          </ul>
        ) : null}
      </div>
    </ConfirmationModal>
  );
}
