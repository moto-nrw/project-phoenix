"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { FramedList } from "~/components/ui/framed-list";
import { Modal } from "~/components/ui/modal";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import type { CareBeyondPhaseEnd } from "~/lib/enrollment-phase-api";
import { schoolClassLabel } from "~/lib/school-class-label";

interface PhaseCareBeyondEndDialogProps {
  readonly phaseName: string;
  /** Das neue Ende der Anmeldephase (YYYY-MM-DD). */
  readonly phaseEnd: string;
  readonly students: readonly CareBeyondPhaseEnd[];
  /** Darf die Person die Betreuung beenden (users:delete)? */
  readonly canEndCare: boolean;
  readonly onEndCare: () => void;
  readonly onClose: () => void;
}

/**
 * Hinweis nach dem Kürzen einer Anmeldephase (#6598): Diese Kinder werden
 * schon betreut, und ihre Betreuung läuft über das neue Ende hinaus. moto
 * ändert daran nichts; die Schule beendet die Betreuung bewusst über
 * „Betreuung beenden“.
 */
export function PhaseCareBeyondEndDialog({
  phaseName,
  phaseEnd,
  students,
  canEndCare,
  onEndCare,
  onClose,
}: PhaseCareBeyondEndDialogProps) {
  const endPassed = phaseEnd < berlinTodayISO();
  const count = students.length === 1 ? "1 Kind" : `${students.length} Kindern`;
  const footer = canEndCare ? (
    <>
      <Button type="button" variant="outline" size="md" onClick={onClose}>
        Schließen
      </Button>
      <Button type="button" variant="primary" size="md" onClick={onEndCare}>
        Betreuung beenden
      </Button>
    </>
  ) : (
    <Button type="button" variant="primary" size="md" onClick={onClose}>
      Schließen
    </Button>
  );

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="Betreuung läuft länger als die Phase"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-lg"
      footer={footer}
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {`Die Anmeldephase „${phaseName}“ endet jetzt am ${formatDate(phaseEnd)}. Bei ${count} ist die Betreuung länger eingetragen. moto beendet sie nicht von selbst. Diese Liste sehen Sie nur jetzt.`}
        </p>
        <FramedList maxHeight="md">
          {students.map((child) => (
            <li
              key={child.studentId}
              className="flex items-center justify-between gap-3 px-4 py-3 text-sm"
            >
              <span className="min-w-0">
                <span className="block truncate font-medium text-gray-900">
                  {`${child.firstName} ${child.lastName}`}
                </span>
                {child.schoolClass ? (
                  <span className="block text-xs text-gray-500">
                    {schoolClassLabel(child.schoolClass)}
                  </span>
                ) : null}
              </span>
              <span className="shrink-0 text-right text-gray-700">
                {`Betreuung bis ${formatDate(child.careEnd)}`}
              </span>
            </li>
          ))}
        </FramedList>
        {canEndCare ? (
          <p className="text-sm leading-6 text-gray-600">
            {endPassed
              ? "Soll die Betreuung mit der Phase enden? Das neue Ende ist schon vorbei. Die Betreuung kann frühestens heute enden."
              : `Soll die Betreuung mit der Phase enden? Dann beenden Sie sie zum ${formatDate(phaseEnd)}.`}
          </p>
        ) : (
          <Alert
            type="warning"
            announce="off"
            message="Die Betreuung beenden darf nur, wer Kinder verwalten darf. Notieren Sie die Namen und geben Sie sie an diese Person weiter."
          />
        )}
      </div>
    </Modal>
  );
}
