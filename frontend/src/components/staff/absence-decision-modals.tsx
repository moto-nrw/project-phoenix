"use client";

import { useState } from "react";

import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { useToast } from "~/contexts/ToastContext";
import { absenceRowLabel, formatAbsenceRange } from "~/lib/absence-helpers";
import { staffAbsenceService, type StaffAbsenceRow } from "~/lib/staff-api";

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
  errorMessage,
  onClose,
  onDone,
}: {
  readonly absence: StaffAbsenceRow;
  readonly title: string;
  readonly noteLabel: string;
  readonly placeholder: string;
  readonly submitLabel: string;
  readonly submitVariant: "danger" | "primary";
  readonly onSubmitNote: (note: string) => Promise<void>;
  readonly successMessage: string;
  readonly errorMessage: string;
  readonly onClose: () => void;
  readonly onDone: () => void | Promise<void>;
}) {
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const handleSubmit = async () => {
    if (note.trim().length < 3) {
      toast.error("Bitte gib eine kurze Begründung ein.");
      return;
    }
    setSubmitting(true);
    try {
      await onSubmitNote(note.trim());
      toast.success(successMessage);
      await onDone();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : errorMessage);
    } finally {
      setSubmitting(false);
    }
  };

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
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={4}
          maxLength={500}
          placeholder={placeholder}
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
  readonly absence: StaffAbsenceRow;
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
      successMessage="Antrag abgelehnt."
      errorMessage="Ablehnung fehlgeschlagen."
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
  readonly absence: StaffAbsenceRow;
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
      successMessage="Rückfrage gesendet."
      errorMessage="Rückfrage fehlgeschlagen."
      onClose={onClose}
      onDone={onQuestioned}
    />
  );
}
