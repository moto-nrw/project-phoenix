"use client";

import { useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Modal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { absenceRowLabel, formatAbsenceRange } from "~/lib/absence-helpers";
import { createLogger } from "~/lib/logger";
import {
  staffAbsenceService,
  type StaffAbsenceRequestRow,
  type StaffAbsenceRow,
} from "~/lib/staff-api";

const logger = createLogger({ component: "AbsenceDecisionModals" });

type AbsenceDecisionRow = Omit<
  StaffAbsenceRow,
  "id" | "staff_id" | "approved_by"
> & {
  id: string | number;
};

// Shared note-entry modal for the deny and Rückfrage decisions (#1419).
// Extracted from abwesenheiten-tab.tsx so the /staff inbox reuses the exact
// same flow. Both actions require a note of at least 3 characters.
function AbsenceNoteModal({
  absence,
  title,
  noteLabel,
  placeholder,
  submitLabel,
  submitVariant,
  onSubmitNote,
  successMessage,
  logEvent,
  onClose,
  onDone,
}: {
  readonly absence: AbsenceDecisionRow | StaffAbsenceRequestRow;
  readonly title: string;
  readonly noteLabel: string;
  readonly placeholder: string;
  readonly submitLabel: string;
  readonly submitVariant: "danger" | "primary";
  readonly onSubmitNote: (note: string) => Promise<void>;
  readonly successMessage: string;
  readonly logEvent: string;
  readonly onClose: () => void;
  readonly onDone: () => void | Promise<void>;
}) {
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const formErrors = useApiFormError();
  const toast = useToast();
  // „Wiederholen“ sendet die aktuelle Notiz, nicht die vom Fehlerzeitpunkt.
  const latestSubmitRef = useRef<() => Promise<void>>(async () => undefined);

  const handleSubmit = async () => {
    formErrors.clear();
    if (note.trim().length < 3) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", {
        decision_note: "Bitte geben Sie eine kurze Begründung ein.",
      });
      return;
    }
    setSubmitting(true);
    try {
      await onSubmitNote(note.trim());
      toast.success(successMessage);
      await onDone();
    } catch (err) {
      logger.error(logEvent, {
        absence_id: String(absence.id),
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "die Anfrage",
        retry: () => void latestSubmitRef.current(),
      });
    } finally {
      setSubmitting(false);
    }
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = handleSubmit;
  });

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title={title}
      footer={
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant={submitVariant}
            size="md"
            onClick={handleSubmit}
            disabled={submitting}
          >
            {submitting ? "…" : submitLabel}
          </Button>
        </div>
      }
    >
      <div className="space-y-3">
        <FormErrorAlert message={formErrors.error} />
        <p className="text-sm text-gray-700">
          Antrag {formatAbsenceRange(absence.date_start, absence.date_end)} (
          {absenceRowLabel(absence)})
        </p>
        <label
          htmlFor="decision-note"
          className="block text-xs font-semibold tracking-wider text-gray-500 uppercase"
        >
          {noteLabel}
        </label>
        <Textarea
          id="decision-note"
          name="decision_note"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={4}
          maxLength={500}
          placeholder={placeholder}
          error={formErrors.fieldError("decision_note")}
        />
        <p className="text-right text-xs text-gray-400">{note.length}/500</p>
      </div>
    </Modal>
  );
}

export function DenyAbsenceModal({
  absence,
  onClose,
  onDenied,
}: {
  readonly absence: AbsenceDecisionRow | StaffAbsenceRequestRow;
  readonly onClose: () => void;
  readonly onDenied: () => void | Promise<void>;
}) {
  return (
    <AbsenceNoteModal
      absence={absence}
      title="Antrag ablehnen"
      noteLabel="Begründung"
      placeholder="Wird der Mitarbeiterin im Antrag angezeigt und bei aktivierten Benachrichtigungen zusätzlich per E-Mail mitgeteilt."
      submitLabel="Ablehnen"
      submitVariant="danger"
      onSubmitNote={(note) => staffAbsenceService.deny(absence.id, note)}
      successMessage="Der Antrag ist abgelehnt."
      logEvent="absence_deny_failed"
      onClose={onClose}
      onDone={onDenied}
    />
  );
}

export function QuestionAbsenceModal({
  absence,
  onClose,
  onQuestioned,
}: {
  readonly absence: AbsenceDecisionRow | StaffAbsenceRequestRow;
  readonly onClose: () => void;
  readonly onQuestioned: () => void | Promise<void>;
}) {
  return (
    <AbsenceNoteModal
      absence={absence}
      title="Rückfrage stellen"
      noteLabel="Rückfrage"
      placeholder="Wird der Mitarbeiterin im Antrag angezeigt. Sie kann ihre Antwort ergänzen und den Antrag erneut einreichen; bei aktivierten Benachrichtigungen erhält sie zusätzlich eine E-Mail."
      submitLabel="Rückfrage senden"
      submitVariant="primary"
      onSubmitNote={(note) => staffAbsenceService.question(absence.id, note)}
      successMessage="Die Rückfrage ist gesendet."
      logEvent="absence_question_failed"
      onClose={onClose}
      onDone={onQuestioned}
    />
  );
}
