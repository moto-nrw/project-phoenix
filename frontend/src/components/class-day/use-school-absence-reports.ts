"use client";

// Nachträgliche Meldung der Schulverwaltung (#4791): Ein Kind fehlt am
// Vormittag, und niemand hat es gemeldet. Rufen die Eltern an, trägt die
// Schulverwaltung „krank“ oder „entschuldigt“ für heute ein. Die OGS sieht den
// Status wie jeden anderen. Nur die Schulverwaltung nimmt so eine Meldung
// zurück.
//
// Der Hook hält nur den Ablauf: welches Kind gerade gespeichert wird, die
// Rückfrage vor dem Zurücknehmen und den Fehler dazu (über der Liste, wie
// bei der Anwesenheit am Vormittag). Die Liste lädt danach neu; das Kind
// verschwindet sichtbar aus „Fehlen ohne Meldung“.

import { useState } from "react";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type {
  AbsenceReportStatus,
  SchoolDayChild,
} from "~/lib/school-class-day-api";

const logger = createLogger({ component: "SchoolAbsenceReports" });

export interface AbsenceReportTarget {
  readonly schoolClass: string;
  readonly child: SchoolDayChild;
}

interface Options {
  readonly dateISO: string;
  readonly report?: (
    schoolClass: string,
    date: string,
    studentId: string,
    status: AbsenceReportStatus,
  ) => Promise<void>;
  readonly withdraw?: (
    schoolClass: string,
    date: string,
    studentId: string,
  ) => Promise<void>;
  /** Lädt die Tagesübersicht nach einer Änderung neu. */
  readonly onChanged: () => Promise<unknown>;
}

function childKey(target: AbsenceReportTarget): string {
  return `${target.schoolClass}:${target.child.student_id ?? ""}`;
}

export function useSchoolAbsenceReports({
  dateISO,
  report,
  withdraw,
  onChanged,
}: Options) {
  const errors = useApiFormError();
  const [pending, setPending] = useState<string | null>(null);
  const [withdrawTarget, setWithdrawTarget] =
    useState<AbsenceReportTarget | null>(null);

  // Ein Schritt mit Fehleranzeige: speichern, die Liste neu laden, bei
  // einem Fehler die Meldung über der Liste mit „Erneut versuchen“.
  const run = async (
    target: AbsenceReportTarget,
    action: (studentId: string) => Promise<void>,
    event: string,
    retry: () => void,
  ): Promise<boolean> => {
    const studentId = target.child.student_id;
    if (!studentId) return false;
    errors.clear();
    setPending(childKey(target));
    try {
      await action(studentId);
      await onChanged();
      return true;
    } catch (cause) {
      logger.error(event, {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void errors.show(cause, { object: "die Meldung", retry });
      // Die Liste kann veraltet sein, etwa wenn die Lehrkraft das Kind
      // inzwischen als da eingetragen hat: neu laden, damit es verschwindet.
      // Scheitert auch das, bleibt die Meldung oben der sichtbare Fehler.
      void onChanged().catch(() => undefined);
      return false;
    } finally {
      setPending(null);
    }
  };

  const enter = async (
    target: AbsenceReportTarget,
    status: AbsenceReportStatus,
  ) => {
    if (!report) return;
    await run(
      target,
      (studentId) => report(target.schoolClass, dateISO, studentId, status),
      "absence_report_failed",
      () => void enter(target, status),
    );
  };

  const confirmWithdraw = async () => {
    const target = withdrawTarget;
    if (!withdraw || !target) return;
    const done = await run(
      target,
      (studentId) => withdraw(target.schoolClass, dateISO, studentId),
      "absence_report_withdraw_failed",
      () => void confirmWithdraw(),
    );
    if (done) setWithdrawTarget(null);
  };

  return {
    /** Der Fehler der letzten Aktion, für `FormErrorAlert`. */
    error: errors.error,
    /** Ob die Seite die Aktionen überhaupt anbieten kann. */
    available: report !== undefined && withdraw !== undefined,
    isPending: (target: AbsenceReportTarget) => pending === childKey(target),
    enter,
    withdrawTarget,
    askWithdraw: setWithdrawTarget,
    closeWithdraw: () => setWithdrawTarget(null),
    withdrawing:
      withdrawTarget !== null && pending === childKey(withdrawTarget),
    confirmWithdraw,
  };
}
