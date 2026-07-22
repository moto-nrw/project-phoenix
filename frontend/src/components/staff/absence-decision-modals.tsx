"use client";

import { useState } from "react";

import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { ABSENCE_TYPE_LABEL, formatAbsenceRange } from "~/lib/absence-helpers";
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
  submitClassName,
  focusClassName,
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
  readonly submitClassName: string;
  readonly focusClassName: string;
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
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={submitting}
            className={`rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50 ${submitClassName}`}
          >
            {submitting ? "…" : submitLabel}
          </button>
        </div>
      }
    >
      <div className="space-y-3">
        <p className="text-sm text-gray-700">
          Antrag {formatAbsenceRange(absence.date_start, absence.date_end)} (
          {ABSENCE_TYPE_LABEL[absence.absence_type] ?? absence.absence_type})
        </p>
        <label
          htmlFor="decision-note"
          className="block text-xs font-semibold tracking-wider text-gray-500 uppercase"
        >
          {noteLabel}
        </label>
        <textarea
          id="decision-note"
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={4}
          maxLength={500}
          placeholder={placeholder}
          className={`w-full resize-none rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 placeholder:text-gray-400 focus:outline-none ${focusClassName}`}
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
      placeholder="Wird der Mitarbeiterin per E-Mail mitgeteilt."
      submitLabel="Ablehnen"
      submitClassName="bg-red-600 hover:bg-red-700"
      focusClassName="focus:border-red-500"
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
      placeholder="Wird der Mitarbeiterin per E-Mail mitgeteilt. Sie kann ihre Antwort ergänzen und den Antrag erneut einreichen."
      submitLabel="Rückfrage senden"
      submitClassName="bg-[#7C3AED] hover:bg-[#6d28d9]"
      focusClassName="focus:border-[#7C3AED]"
      onSubmitNote={(note) => staffAbsenceService.question(absence.id, note)}
      successMessage="Rückfrage gesendet."
      errorMessage="Rückfrage fehlgeschlagen."
      onClose={onClose}
      onDone={onQuestioned}
    />
  );
}
