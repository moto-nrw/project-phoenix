"use client";

import { LogOut, Trash2, Undo2, XCircle } from "lucide-react";
import { useCallback, useState } from "react";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { useToast } from "~/contexts/ToastContext";
import {
  cancelCareExit,
  canResumeCare,
  hasPlannedCareExit,
} from "~/lib/care-exit-api";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { Student } from "~/lib/api";
import { CareExitModal } from "./care-exit-modal";
import { CareResumeModal } from "./care-resume-modal";
import { StudentDeletionModal } from "./student-deletion-modal";

const logger = createLogger({ component: "StudentRecordActions" });

/** Die Felder der Kindakte, an denen die Datensatz-Aktionen hängen. */
type RecordStudent = Pick<
  Student,
  "id" | "care_ended" | "care_ends_on" | "care_exit_recorded"
>;

interface StudentRecordActionsProps {
  readonly student: RecordStudent;
  readonly displayName: string;
  /** Lädt die Akte neu, nachdem ein Ende eingetragen, storniert oder aufgehoben wurde. */
  readonly onChanged: () => void | Promise<void>;
  /** Das Kind gibt es nicht mehr; die Seite verlässt die Akte. */
  readonly onDeleted: () => void | Promise<void>;
}

/**
 * Das Kebab-Menü der Kindakte für die Aktionen am Datensatz (BAUARTEN-SPEC
 * Bauart 2 Regel 1): Betreuung beenden, ein geplantes Ende ändern oder
 * stornieren, nach dem Austritt wieder aufnehmen, Löschen (#2487, #3115).
 *
 * Alles hier braucht die Berechtigung „Benutzer löschen"; die Seite rendert
 * das Menü ohne sie gar nicht. Jede Aktion fragt vor dem Schreiben nach
 * (Bauart 2 Regel 6): der Klick öffnet den Dialog, die API läuft in dessen
 * Bestätigung.
 */
export function StudentRecordActions({
  student,
  displayName,
  onChanged,
  onDeleted,
}: StudentRecordActionsProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [careExitOpen, setCareExitOpen] = useState(false);
  const [cancelExitOpen, setCancelExitOpen] = useState(false);
  const [cancellingExit, setCancellingExit] = useState(false);
  const [resumeOpen, setResumeOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const studentId = String(student.id);
  const plannedExit = hasPlannedCareExit(student as Student);

  // Storniert ein noch nicht wirksames Betreuungsende (#2487). Ein bereits
  // wirksamer Austritt kann nur über „Wieder aufnehmen" zurückgenommen
  // werden, mit neuem Beginn und ausdrücklicher Prüfung.
  const cancelPlannedExit = useCallback(async () => {
    setCancellingExit(true);
    try {
      await cancelCareExit([studentId]);
      // Sagt beides: das Ende ist weg UND der Plan ist zurück. Ohne den
      // zweiten Halbsatz bliebe offen, ob die Termine neu eingetragen werden
      // müssen (#2487).
      toastSuccess(
        `Das geplante Betreuungsende von ${displayName} wurde storniert. Termine und Angebote gelten wieder.`,
      );
      setCancelExitOpen(false);
      await onChanged();
    } catch (cancelError) {
      const message =
        cancelError instanceof Error
          ? cancelError.message
          : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
      logger.error("care_exit_cancel_failed", {
        student_id: studentId,
        error: message,
      });
      toastError(message);
    } finally {
      setCancellingExit(false);
    }
  }, [displayName, onChanged, studentId, toastError, toastSuccess]);

  const items: OverflowMenuItem[] = [];
  if (student.care_ended) {
    // Wieder aufnehmen kann nur, wer einen hinterlegten Austritt zurücknimmt.
    // Lief die Betreuung mit der Anmeldephase aus, weist der Server die
    // Wiederaufnahme ab; dann steht der Eintrag gar nicht erst da (#2487).
    if (canResumeCare(student as Student)) {
      items.push({
        label: "Wieder aufnehmen",
        icon: <Undo2 className="size-4" aria-hidden />,
        onClick: () => setResumeOpen(true),
      });
    }
  } else {
    items.push({
      label: plannedExit ? "Ende ändern" : "Betreuung beenden",
      icon: <LogOut className="size-4" aria-hidden />,
      onClick: () => setCareExitOpen(true),
    });
    if (plannedExit) {
      items.push({
        label: "Ende stornieren",
        icon: <XCircle className="size-4" aria-hidden />,
        disabled: cancellingExit,
        onClick: () => setCancelExitOpen(true),
      });
    }
  }
  items.push({
    label: "Löschen",
    icon: <Trash2 className="size-4" aria-hidden />,
    destructive: true,
    onClick: () => setDeleteOpen(true),
  });

  return (
    <>
      <OverflowMenu ariaLabel="Weitere Aktionen" items={items} />

      {careExitOpen ? (
        <CareExitModal
          isOpen
          studentIds={[studentId]}
          plannedLastCareDay={
            plannedExit ? (student.care_ends_on ?? undefined) : undefined
          }
          onClose={() => setCareExitOpen(false)}
          onFinished={async () => {
            setCareExitOpen(false);
            await onChanged();
          }}
        />
      ) : null}

      <ConfirmationModal
        isOpen={cancelExitOpen}
        title="Betreuungsende stornieren?"
        confirmText="Ende stornieren"
        cancelText="Abbrechen"
        isConfirmLoading={cancellingExit}
        isDismissDisabled={cancellingExit}
        onConfirm={() => void cancelPlannedExit()}
        onClose={() => setCancelExitOpen(false)}
      >
        <p className="text-sm text-gray-700">
          Das geplante Betreuungsende von <strong>{displayName}</strong>
          {student.care_ends_on
            ? ` am ${formatDate(student.care_ends_on)}`
            : ""}{" "}
          wird storniert. Termine und Angebote gelten wieder.
        </p>
      </ConfirmationModal>

      {resumeOpen ? (
        <CareResumeModal
          isOpen
          studentId={studentId}
          displayName={displayName}
          onClose={() => setResumeOpen(false)}
          onResumed={async () => {
            setResumeOpen(false);
            await onChanged();
          }}
        />
      ) : null}

      {deleteOpen ? (
        <StudentDeletionModal
          isOpen
          studentId={studentId}
          displayName={displayName}
          careEnded={student.care_ended === true}
          onClose={() => setDeleteOpen(false)}
          onDeleted={async () => {
            setDeleteOpen(false);
            toastSuccess(`${displayName} wurde gelöscht.`);
            await onDeleted();
          }}
        />
      ) : null}
    </>
  );
}
