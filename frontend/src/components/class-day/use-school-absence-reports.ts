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

  const enter = async (
    target: AbsenceReportTarget,
    status: AbsenceReportStatus,
  ) => {
    const studentId = target.child.student_id;
    if (!report || !studentId) return;
    errors.clear();
    setPending(childKey(target));
    try {
      await report(target.schoolClass, dateISO, studentId, status);
      await onChanged();
    } catch (cause) {
      logger.error("absence_report_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void errors.show(cause, {
        object: "die Meldung",
        retry: () => void enter(target, status),
      });
    } finally {
      setPending(null);
    }
  };

  const confirmWithdraw = async () => {
    const target = withdrawTarget;
    const studentId = target?.child.student_id;
    if (!withdraw || !target || !studentId) return;
    errors.clear();
    setPending(childKey(target));
    try {
      await withdraw(target.schoolClass, dateISO, studentId);
      await onChanged();
      setWithdrawTarget(null);
    } catch (cause) {
      logger.error("absence_report_withdraw_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void errors.show(cause, {
        object: "die Meldung",
        retry: () => void confirmWithdraw(),
      });
    } finally {
      setPending(null);
    }
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
