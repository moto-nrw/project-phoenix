"use client";

/**
 * ClosingDayModal creates or edits one OGS-Schließtag range (#1418 3b).
 * A single closed day is entered as start = end. Kept deliberately small —
 * unlike CalendarPeriodModal there is no type, cycle, or link section.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { closingDayService } from "~/lib/closing-day-api";
import { type ClosingDay } from "~/lib/closing-day-helpers";
import { createLogger } from "~/lib/logger";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "ClosingDayModal" });

export function ClosingDayModal({
  isOpen,
  onClose,
  onSaved,
  initial,
  onOfferCancel,
}: ClosingDayModalProps) {
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const clearErrors = formErrors.clear;
  const latestSaveRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    if (!isOpen) return;
    setStartDate(initial?.startDate ?? "");
    setEndDate(initial?.endDate ?? "");
    setReason(initial?.reason ?? "");
    clearErrors();
  }, [isOpen, initial, clearErrors]);

  const handleSave = async () => {
    const fields = closingDayFieldErrors(reason, startDate, endDate);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    setSaving(true);
    formErrors.clear();
    const body = {
      start_date: startDate,
      end_date: endDate,
      reason: reason.trim(),
    };
    try {
      if (initial) {
        await closingDayService.update(initial.id, body);
      } else {
        await closingDayService.create(body);
      }
      onSaved();
      onClose();
      // #3594: Termine, die vor dem Schließtag geplant waren, bleiben im Plan.
      // Stehen noch welche im Zeitraum, bietet der Editor das Absagen an.
      await offerCancel(onOfferCancel, startDate, endDate);
    } catch (err) {
      logger.error("closing_day_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "das Speichern des Schließtags",
        retry: () => latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };
  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSaveRef.current = () => void handleSave();
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={initial ? "Schließtag bearbeiten" : "Schließtag anlegen"}
      size="md"
      error={formErrors.error}
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void handleSave()}
            disabled={saving}
          >
            {saving ? "Speichern..." : "Speichern"}
          </Button>
        </div>
      }
    >
      <div ref={formRef} className="space-y-4">
        <div>
          <label
            htmlFor="closing-day-reason"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Grund
          </label>
          <Input
            id="closing-day-reason"
            name="reason"
            type="text"
            value={reason}
            maxLength={255}
            onChange={(e) => setReason(e.target.value)}
            error={formErrors.fieldError("reason")}
            placeholder="z. B. Pädagogischer Tag, Sommerschließung"
          />
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label
              htmlFor="closing-day-start"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Von
            </label>
            <ISODatePicker
              id="closing-day-start"
              name="start_date"
              controlSize="lg"
              value={startDate}
              onChange={setStartDate}
              error={formErrors.fieldError("start_date")}
              calendarLayout="popover"
            />
          </div>
          <div>
            <label
              htmlFor="closing-day-end"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Bis
            </label>
            <ISODatePicker
              id="closing-day-end"
              name="end_date"
              controlSize="lg"
              value={endDate}
              min={startDate || undefined}
              defaultMonth={startDate}
              onChange={setEndDate}
              error={formErrors.fieldError("end_date")}
              calendarLayout="popover"
            />
          </div>
        </div>

        <p className="text-xs text-gray-500">
          Für einen einzelnen Tag dasselbe Datum in beide Felder eintragen. An
          Schließtagen fallen Termine aus und Mitarbeitende haben kein Soll.
        </p>
      </div>
    </FormModal>
  );
}

interface ClosingDayModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSaved: () => void;
  readonly initial: ClosingDay | null;
  /** Opens „Termine absagen“ for the saved range when appointments remain. */
  readonly onOfferCancel?: (range: {
    startDate: string;
    endDate: string;
  }) => void;
}

// #3594: Stehen im gespeicherten Zeitraum noch geplante Termine, bietet moto
// das Absagen an. Ein Fehler beim Zählen blockiert das Speichern nicht.
async function offerCancel(
  onOfferCancel: ClosingDayModalProps["onOfferCancel"],
  from: string,
  to: string,
) {
  if (!onOfferCancel) return;
  try {
    const preview = await timetableService.bulkCancel(from, to, true);
    if (preview.count > 0) onOfferCancel({ startDate: from, endDate: to });
  } catch (err) {
    logger.warn("closing_day_cancel_preview_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
    // Bewusst still ohne Recht zum Absagen: der Dialog könnte nichts tun.
    if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
      return;
    }
    // Sonst ist offen, ob noch Termine im Zeitraum liegen. Der Dialog
    // „Termine absagen“ zählt selbst neu und zeigt einen Fehler dort mit
    // Wiederholen; still zu bleiben ließe geplante Termine stehen.
    onOfferCancel({ startDate: from, endDate: to });
  }
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
function closingDayFieldErrors(
  reason: string,
  startDate: string,
  endDate: string,
): Record<string, string> {
  const fields: Record<string, string> = {};
  if (!reason.trim()) fields.reason = "Bitte geben Sie einen Grund an.";
  if (!startDate) fields.start_date = "Bitte wählen Sie den ersten Tag.";
  if (!endDate) fields.end_date = "Bitte wählen Sie den letzten Tag.";
  if (startDate && endDate && endDate < startDate)
    fields.end_date = "Der letzte Tag darf nicht vor dem ersten liegen.";
  return fields;
}
