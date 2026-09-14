"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useFormError } from "~/components/ui/form-error";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { TimeField } from "~/components/ui/time-field";
import {
  berlinClockFromISO,
  berlinDateTimeISO,
  berlinDayFromISO,
} from "~/lib/date-helpers";
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
  if (!day) return null;
  if (!TIME_PATTERN.test(time)) {
    return "Bitte eine Uhrzeit für die Erinnerung eingeben, z. B. 08:00.";
  }
  let moment: Date;
  try {
    moment = new Date(berlinDateTimeISO(day, time));
  } catch (error) {
    if (error instanceof RangeError) {
      if (error.message === "ambiguous Berlin wall-clock time") {
        return "Diese Uhrzeit kommt an diesem Tag in Berlin zweimal vor. Bitte eine andere Uhrzeit wählen.";
      }
      return "Diese Uhrzeit gibt es an diesem Tag in Berlin nicht. Bitte eine andere Uhrzeit wählen.";
    }
    throw error;
  }
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
  readonly onSaved: () => Promise<void> | void;
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
  const [pending, setPending] = useState<"save" | "remove" | null>(null);
  // Removing is a state change the parents never see coming, so it asks once
  // (BAUARTEN-SPEC Bauart 2 Regel 6): the click opens the question, the
  // request runs from its confirmation.
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [removeError, setRemoveError] = useState("");
  const [error, setError] = useFormError();

  const expiresAt = announcement.expires_at
    ? new Date(announcement.expires_at)
    : null;

  const submit = async (remove: boolean) => {
    if (!remove && !day) {
      setError("Bitte einen Tag für die Erinnerung wählen.");
      return;
    }
    if (!remove) {
      const problem = reminderError(day, time, expiresAt);
      if (problem) {
        setError(problem);
        return;
      }
    }
    setPending(remove ? "remove" : "save");
    setError("");
    setRemoveError("");
    try {
      await updateAnnouncementReminder(announcement.id, {
        reminder_at: remove || !day ? null : berlinDateTimeISO(day, time),
        reminder_text: remove || !text.trim() ? null : text.trim(),
      });
      await onSaved();
      setConfirmRemove(false);
      onClose();
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Erinnerung konnte nicht gespeichert werden";
      if (remove) {
        setRemoveError(message);
      } else {
        setError(message);
      }
      logger.error("announcement_reminder_update_failed", { error: message });
    } finally {
      setPending(null);
    }
  };

  return (
    <>
      <FormModal
        isOpen
        onClose={onClose}
        title={hasReminder ? "Erinnerung ändern" : "Erinnerung planen"}
        size="md"
        closeDisabled={pending !== null}
        suspended={confirmRemove}
        error={error}
        footer={
          <div className="flex flex-wrap justify-end gap-2">
            {hasReminder && (
              <Button
                type="button"
                variant="secondary"
                size="md"
                onClick={() => setConfirmRemove(true)}
                disabled={pending !== null}
                className="mr-auto"
              >
                {pending === "remove"
                  ? "Wird entfernt…"
                  : "Erinnerung entfernen"}
              </Button>
            )}
            <Button
              type="button"
              variant="secondary"
              size="md"
              onClick={onClose}
              disabled={pending !== null}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              size="md"
              onClick={() => void submit(false)}
              disabled={pending !== null}
            >
              {pending === "save" ? "Wird gespeichert…" : "Speichern"}
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            moto schickt „{announcement.title}“ zu diesem Zeitpunkt noch einmal
            an alle Empfänger, auch wenn sie schon gelesen oder bestätigt haben.
            Titel, Text und Empfänger bleiben unverändert.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <span className="mb-1.5 block text-sm font-medium text-gray-700">
                Erinnern am
              </span>
              <DatePicker
                value={day}
                onChange={setDay}
                placeholder="Tag wählen"
              />
            </div>
            <TimeField
              label="Uhrzeit"
              value={time}
              onChange={setTime}
              hint="Uhrzeit im Format 08:00"
              placeholder="08:00"
              required
            />
          </div>
          <div>
            <label
              htmlFor="announcement-reminder-dialog-text"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Erinnerungstext (optional)
            </label>
            <textarea
              id="announcement-reminder-dialog-text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={3}
              maxLength={MAX_REMINDER_TEXT_LENGTH}
              placeholder="Kurz das Wichtigste, z. B. „Morgen endet die Betreuung um 13:00 Uhr.“"
              className="block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
            />
            <p className="mt-1.5 text-xs text-gray-500">
              Ohne eigenen Text schickt moto den Text der Mitteilung noch
              einmal.
            </p>
          </div>
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
          setRemoveError("");
        }}
        loading={pending === "remove"}
        error={removeError}
      />
    </>
  );
}
