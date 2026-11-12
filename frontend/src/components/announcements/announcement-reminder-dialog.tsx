"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { TimeField } from "~/components/ui/time-field";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  berlinClockFromISO,
  berlinDateTimeISO,
  berlinDayFromISO,
  berlinWallClockProblem,
} from "~/lib/date-helpers";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { updateAnnouncementReminder } from "~/lib/parent-announcements-api";
import type { Announcement } from "~/lib/parent-announcements-api";

const logger = createLogger({ component: "AnnouncementReminderDialog" });

/**
 * Scheduled reminder (#3162). The default clock is the morning before school
 * starts; the length mirrors the backend (maxReminderTextLen).
 */
export const DEFAULT_REMINDER_TIME = "08:00";
export const MAX_REMINDER_TEXT_LENGTH = 500;
const TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$/;

export function reminderTimeError(
  day: Date | null,
  time: string,
): string | null {
  if (!day) return null;
  if (!TIME_PATTERN.test(time)) {
    return "Bitte eine Uhrzeit für die Erinnerung eingeben, z. B. 08:00.";
  }
  return berlinWallClockProblem(day, time);
}

/**
 * The reminder rules the backend enforces, said before the request: a valid
 * clock time, a moment still ahead of us, and never after the expiry (the
 * announcement would already be hidden when the reminder points at it).
 */
export function reminderError(
  day: Date | null,
  time: string,
  expiresAt: Date | null,
  now: Date = new Date(),
): string | null {
  const timeProblem = reminderTimeError(day, time);
  if (timeProblem) return timeProblem;
  if (!day) return null;
  const moment = new Date(berlinDateTimeISO(day, time));
  if (moment <= now) {
    return "Die Erinnerung muss in der Zukunft liegen.";
  }
  if (expiresAt && moment >= expiresAt) {
    return "Die Erinnerung muss vor dem Ablaufdatum liegen, sonst sehen Eltern die Mitteilung nicht mehr.";
  }
  return null;
}

interface AnnouncementReminderDialogProps {
  readonly announcement: Announcement;
  readonly onClose: () => void;
  /** Läuft nach dem erfolgreichen Speichern, vor dem Schließen (neu laden). */
  /** Reloads the announcement; never rejects (#6652). */
  readonly onSaved: RefreshAfterWrite;
}

/**
 * Die eine Änderung, die eine VERÖFFENTLICHTE Mitteilung noch zulässt
 * (#3162): die geplante Erinnerung verschieben, umformulieren oder entfernen,
 * solange sie nicht verschickt wurde. Titel, Text und Empfänger bleiben
 * gesperrt; dieser Dialog zeigt sie deshalb gar nicht erst an.
 */
export function AnnouncementReminderDialog({
  announcement,
  onClose,
  onSaved,
}: AnnouncementReminderDialogProps) {
  const hasReminder = Boolean(announcement.reminder_at);
  const [day, setDay] = useState<Date | null>(
    announcement.reminder_at
      ? berlinDayFromISO(announcement.reminder_at)
      : null,
  );
  const [time, setTime] = useState(
    announcement.reminder_at
      ? berlinClockFromISO(announcement.reminder_at)
      : DEFAULT_REMINDER_TIME,
  );
  const [text, setText] = useState(announcement.reminder_text ?? "");
  // Welche Aktion läuft, für die Beschriftung; die Sperre hält `save` (#6533).
  const [pending, setPending] = useState<"save" | "remove" | null>(null);
  const save = useFormModalPending();
  // Removing is a state change the parents never see coming, so it asks once
  // (BAUARTEN-SPEC Bauart 2 Regel 6): the click opens the question, the
  // request runs from its confirmation.
  const [confirmRemove, setConfirmRemove] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  // Fehler beim Entfernen bleiben im offenen Bestätigungsdialog.
  const removeErrors = useApiFormError();
  const latestSubmitRef = useRef<(remove: boolean) => void>(() => undefined);

  const expiresAt = announcement.expires_at
    ? new Date(announcement.expires_at)
    : null;

  const submit = async (remove: boolean) => {
    if (!remove && !day) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", {
        reminder_at: "Bitte wählen Sie einen Tag für die Erinnerung.",
      });
      return;
    }
    if (!remove) {
      const problem = reminderError(day, time, expiresAt);
      if (problem) {
        formErrors.invalid(problem, { reminder_at: problem });
        return;
      }
    }
    formErrors.clear();
    removeErrors.clear();
    await save.run(async (op) => {
      setPending(remove ? "remove" : "save");
      try {
        await updateAnnouncementReminder(announcement.id, {
          reminder_at: remove || !day ? null : berlinDateTimeISO(day, time),
          reminder_text: remove || !text.trim() ? null : text.trim(),
        });
      } catch (err) {
        logger.error("announcement_reminder_update_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        await (remove ? removeErrors : formErrors).show(err, {
          object: remove
            ? "das Entfernen der Erinnerung"
            : "das Speichern der Erinnerung",
          retry: () => latestSubmitRef.current(remove),
        });
        setPending(null);
        return;
      }
      setPending(null);
      await onSaved();
      // Eine späte Antwort schließt keinen inzwischen neuen Dialog (#6533).
      if (!op.isCurrent()) return;
      setConfirmRemove(false);
      onClose();
    });
  };
  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = (remove) => void submit(remove);
  });
  const reminderAtError = formErrors.fieldError("reminder_at");

  return (
    <>
      <FormModal
        isOpen
        onClose={onClose}
        title={hasReminder ? "Erinnerung ändern" : "Erinnerung planen"}
        size="md"
        pending={save}
        suspended={confirmRemove}
        error={formErrors.error}
        footer={
          // On a phone Abbrechen and Speichern share one full-width row and
          // „Erinnerung entfernen“ sits full width below them (#5758).
          <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            {hasReminder && (
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                onClick={() => setConfirmRemove(true)}
                disabled={pending !== null}
                className="w-full sm:mr-auto sm:w-auto"
              >
                {pending === "remove"
                  ? "Wird entfernt…"
                  : "Erinnerung entfernen"}
              </Button>
            )}
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={onClose}
                disabled={pending !== null}
                className="flex-1 sm:flex-none"
              >
                Abbrechen
              </Button>
              <Button
                type="button"
                size="md"
                onClick={() => void submit(false)}
                disabled={pending !== null}
                className="flex-1 sm:flex-none"
              >
                {pending === "save" ? "Wird gespeichert…" : "Speichern"}
              </Button>
            </div>
          </div>
        }
      >
        <div ref={formRef} className="space-y-4">
          <p className="text-sm text-gray-600">
            moto schickt „{announcement.title}“ zu diesem Zeitpunkt noch einmal
            an alle Empfänger, auch wenn sie schon{" "}
            {announcement.delivery_mode === "declaration"
              ? "geantwortet"
              : "gelesen oder bestätigt"}{" "}
            haben. Titel, Text und Empfänger bleiben unverändert.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <FieldLabel
                htmlFor="announcement-reminder-at"
                invalid={Boolean(reminderAtError)}
              >
                Erinnern am
              </FieldLabel>
              <DatePicker
                id="announcement-reminder-at"
                name="reminder_at"
                value={day}
                onChange={setDay}
                placeholder="Tag wählen"
                invalid={Boolean(reminderAtError)}
                ariaDescribedBy={
                  reminderAtError ? "announcement-reminder-at-error" : undefined
                }
              />
              {reminderAtError ? (
                <p
                  id="announcement-reminder-at-error"
                  className="text-moto-red-strong mt-1 text-xs"
                >
                  {reminderAtError}
                </p>
              ) : null}
            </div>
            <TimeField
              label="Uhrzeit"
              value={time}
              onChange={setTime}
              hint="Uhrzeit im Format 08:00"
              placeholder="08:00"
              required
              invalid={Boolean(reminderAtError)}
            />
          </div>
          <Textarea
            id="announcement-reminder-dialog-text"
            name="reminder_text"
            label="Erinnerungstext (optional)"
            hint={
              announcement.delivery_mode === "letter"
                ? "Ohne eigenen Text schickt moto den Text des Elternbriefs noch einmal."
                : "Die E-Mail hat nur Titel und Link. Den Text sehen Eltern im Eltern-Portal."
            }
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={3}
            maxLength={MAX_REMINDER_TEXT_LENGTH}
            placeholder="Kurz das Wichtigste, z. B. „Morgen endet die Betreuung um 13:00 Uhr.“"
            className="resize-y px-4 py-3 text-base"
          />
        </div>
      </FormModal>
      <ConfirmDeleteModal
        isOpen={confirmRemove}
        title="Erinnerung entfernen"
        description={
          <>
            Die Eltern bekommen dann keine Erinnerung zu
            <span className="font-medium text-gray-900">
              {" "}
              „{announcement.title}“
            </span>
            . Die Mitteilung selbst bleibt, wie sie ist.
          </>
        }
        gate={{ mode: "twoStep" }}
        confirmLabel="Erinnerung entfernen"
        loadingLabel="Wird entfernt…"
        onConfirm={() => submit(true)}
        onClose={() => {
          setConfirmRemove(false);
          removeErrors.clear();
        }}
        loading={pending === "remove"}
        error={removeErrors.error}
      />
    </>
  );
}
