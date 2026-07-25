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

const logger = createLogger({ component: "ClosingDayModal" });

export function ClosingDayModal({
  isOpen,
  onClose,
  onSaved,
  initial,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSaved: () => void;
  readonly initial: ClosingDay | null;
}) {
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
            className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]"
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
              value={endDate}
              min={startDate || undefined}
              onChange={setEndDate}
              calendarLayout="popover"
            />
          </div>
        </div>

        <p className="text-xs text-gray-500">
          Für einen einzelnen Schließtag dasselbe Datum in beide Felder
          eintragen. An Schließtagen gilt für alle Mitarbeitenden Soll = 0,
          genau wie an gesetzlichen Feiertagen.
        </p>
      </div>
    </FormModal>
  );
}
