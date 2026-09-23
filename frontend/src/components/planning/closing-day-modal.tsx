"use client";

/**
 * ClosingDayModal creates or edits one OGS-Schließtag range (#1418 3b).
 * A single closed day is entered as start = end. Kept deliberately small —
 * unlike CalendarPeriodModal there is no type, cycle, or link section.
 */

import { useEffect, useState } from "react";

import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setStartDate(initial?.startDate ?? "");
    setEndDate(initial?.endDate ?? "");
    setReason(initial?.reason ?? "");
    setError(null);
  }, [isOpen, initial]);

  const validate = (): string | null => {
    if (!reason.trim()) return "Bitte einen Grund angeben.";
    if (!startDate) return "Bitte ein Startdatum wählen.";
    if (!endDate) return "Bitte ein Enddatum wählen.";
    if (endDate < startDate)
      return "Das Enddatum darf nicht vor dem Startdatum liegen.";
    return null;
  };

  const handleSave = async () => {
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    setError(null);
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
      const message =
        err instanceof Error
          ? err.message
          : "Schließtag konnte nicht gespeichert werden";
      logger.error("closing_day_save_failed", { error: message });
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={initial ? "Schließtag bearbeiten" : "Schließtag anlegen"}
      size="md"
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
      <div className="space-y-4">
        {error && (
          <div
            className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border p-3 text-sm"
            role="alert"
          >
            {error}
          </div>
        )}

        <div>
          <label
            htmlFor="closing-day-reason"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Grund
          </label>
          <Input
            id="closing-day-reason"
            type="text"
            value={reason}
            maxLength={255}
            onChange={(e) => setReason(e.target.value)}
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
              controlSize="lg"
              value={startDate}
              onChange={setStartDate}
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
              controlSize="lg"
              value={endDate}
              min={startDate || undefined}
              onChange={setEndDate}
              calendarLayout="popover"
            />
          </div>
        </div>

        <p className="text-xs text-gray-500">
          Für einen einzelnen Schließtag dasselbe Datum in beide Felder
          eintragen. Regeltermine fallen an Schließtagen aus. Ausnahme: Serien,
          die auch an Schließtagen geplant sind, z. B. die Ferienbetreuung.
          Mitarbeitende haben an Schließtagen keine Sollstunden, außer es ist
          eine Sonderarbeitszeit eingetragen.
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
  }
}
