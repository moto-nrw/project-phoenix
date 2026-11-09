"use client";

/**
 * Schließtag des Trägers anlegen oder ändern (#4702): Grund, Zeitraum und
 * die Schulen, für die er gilt. Er wirkt wie ein Schließtag, den die Schule
 * selbst einträgt.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  createTraegerClosingDay,
  type TraegerCalendarSchool,
  type TraegerClosingDay,
  updateTraegerClosingDay,
} from "~/lib/traeger-closing-days";

const logger = createLogger({ component: "TraegerClosingDayModal" });

export function TraegerClosingDayModal({
  isOpen,
  onClose,
  onSaved,
  initial,
  schools,
  defaultDate,
}: Readonly<{
  isOpen: boolean;
  onClose: () => void;
  onSaved: (day: TraegerClosingDay) => void;
  initial: TraegerClosingDay | null;
  schools: readonly TraegerCalendarSchool[];
  /** Der vorbelegte Tag eines neuen Schließtags. */
  defaultDate: string;
}>) {
  const [reason, setReason] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [schoolIds, setSchoolIds] = useState<readonly string[]>([]);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const clearErrors = formErrors.clear;
  const latestSaveRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    if (!isOpen) return;
    setReason(initial?.reason ?? "");
    setStartDate(initial?.startDate ?? defaultDate);
    setEndDate(initial?.endDate ?? defaultDate);
    setSchoolIds(initial?.schoolIds ?? schools.map((school) => school.id));
    clearErrors();
  }, [isOpen, initial, schools, defaultDate, clearErrors]);

  const allSelected =
    schools.length > 0 && schools.every((s) => schoolIds.includes(s.id));
  const toggleSchool = (id: string, checked: boolean) =>
    setSchoolIds((current) =>
      checked ? [...current, id] : current.filter((value) => value !== id),
    );

  const handleSave = async () => {
    const fields = closingDayFieldErrors(reason, startDate, endDate, schoolIds);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    setSaving(true);
    formErrors.clear();
    const input = { reason, startDate, endDate, schoolIds };
    try {
      const saved = initial
        ? await updateTraegerClosingDay(initial.id, input)
        : await createTraegerClosingDay(input);
      onSaved(saved);
      onClose();
    } catch (err) {
      logger.error("traeger_closing_day_save_failed", {
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
  useLayoutEffect(() => {
    latestSaveRef.current = () => void handleSave();
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={initial ? "Schließtag ändern" : "Schließtag anlegen"}
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
      <div ref={formRef} className="space-y-5">
        <Input
          id="traeger-closing-day-reason"
          name="reason"
          type="text"
          label="Grund"
          required
          value={reason}
          maxLength={120}
          onChange={(event) => setReason(event.target.value)}
          error={formErrors.fieldError("reason")}
          placeholder="z. B. Brückentag oder Betriebsausflug"
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <ISODatePicker
            id="traeger-closing-day-start"
            name="start_date"
            label="Von"
            controlSize="lg"
            value={startDate}
            onChange={(value) => {
              setStartDate(value);
              if (!endDate || endDate < value) setEndDate(value);
            }}
            error={formErrors.fieldError("start_date")}
            calendarLayout="popover"
          />
          <ISODatePicker
            id="traeger-closing-day-end"
            name="end_date"
            label="Bis"
            controlSize="lg"
            value={endDate}
            min={startDate || undefined}
            defaultMonth={startDate}
            onChange={setEndDate}
            error={formErrors.fieldError("end_date")}
            calendarLayout="popover"
          />
        </div>
        <FormFieldset
          legend="Gilt für"
          required
          error={formErrors.fieldError("school_ids")}
        >
          <div className="space-y-2">
            <ChoiceTile selected={allSelected} className="px-3 py-2.5">
              <Checkbox
                checked={allSelected}
                onChange={(event) =>
                  setSchoolIds(
                    event.target.checked ? schools.map((s) => s.id) : [],
                  )
                }
              />
              <span className="text-sm font-semibold text-gray-900">
                Alle Schulen
              </span>
            </ChoiceTile>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {schools.map((school) => (
                <ChoiceTile
                  key={school.id}
                  selected={schoolIds.includes(school.id)}
                  className="px-3 py-2.5"
                >
                  <Checkbox
                    checked={schoolIds.includes(school.id)}
                    onChange={(event) =>
                      toggleSchool(school.id, event.target.checked)
                    }
                  />
                  <span className="min-w-0 text-sm wrap-anywhere text-gray-900">
                    {school.name}
                  </span>
                </ChoiceTile>
              ))}
            </div>
          </div>
        </FormFieldset>
        <p className="text-xs text-gray-500">
          An diesem Tag ist die OGS zu. Es gibt keine Ferienbetreuung, Termine
          fallen aus und Mitarbeitende haben kein Soll.
        </p>
      </div>
    </FormModal>
  );
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
export function closingDayFieldErrors(
  reason: string,
  startDate: string,
  endDate: string,
  schoolIds: readonly string[],
): Record<string, string> {
  const fields: Record<string, string> = {};
  if (!reason.trim()) fields.reason = "Bitte geben Sie einen Grund an.";
  if (!startDate) fields.start_date = "Bitte wählen Sie den ersten Tag.";
  if (!endDate) fields.end_date = "Bitte wählen Sie den letzten Tag.";
  if (startDate && endDate && endDate < startDate)
    fields.end_date = "Der letzte Tag darf nicht vor dem ersten liegen.";
  if (schoolIds.length === 0)
    fields.school_ids = "Bitte wählen Sie mindestens eine Schule.";
  return fields;
}
