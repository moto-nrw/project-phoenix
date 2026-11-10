"use client";

/**
 * ClosingDayModal creates or edits one OGS-Schließtag range (#1418 3b).
 * A single closed day is entered as start = end. Kept deliberately small —
 * unlike CalendarPeriodModal there is no type, cycle, or link section.
 */

import {
  type ReactNode,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
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
  onSave,
  schools,
  defaultDate = "",
  reasonMaxLength = 255,
  onDraftChange,
  consequences,
  saveLabel = "Speichern",
  saveBlocked = false,
}: ClosingDayModalProps) {
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [reason, setReason] = useState("");
  const [schoolIds, setSchoolIds] = useState<readonly string[]>([]);
  const initializedRecord = useRef<string | null>(null);
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const clearErrors = formErrors.clear;
  const latestSaveRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    if (!isOpen) {
      initializedRecord.current = null;
      return;
    }
    const record = initial?.id ?? "new";
    if (initializedRecord.current === record) return;
    initializedRecord.current = record;
    setStartDate(initial?.startDate ?? defaultDate);
    setEndDate(initial?.endDate ?? defaultDate);
    setReason(initial?.reason ?? "");
    setSchoolIds(
      initial?.schoolIds ?? schools?.map((school) => school.id) ?? [],
    );
    clearErrors();
  }, [isOpen, initial, schools, defaultDate, clearErrors]);

  // Die Seite zeigt zum Entwurf, was der Schließtag bewirkt (#6434).
  const draftChanged = useRef(onDraftChange);
  useLayoutEffect(() => {
    draftChanged.current = onDraftChange;
  });
  const schoolKey = schoolIds.join(",");
  useEffect(() => {
    if (!isOpen) return;
    draftChanged.current?.({
      startDate,
      endDate,
      schoolIds: schoolKey ? schoolKey.split(",") : [],
    });
  }, [isOpen, startDate, endDate, schoolKey]);

  const allSelected =
    (schools?.length ?? 0) > 0 &&
    (schools?.every((s) => schoolIds.includes(s.id)) ?? false);
  const toggleSchool = (id: string, checked: boolean) =>
    setSchoolIds((current) =>
      checked ? [...current, id] : current.filter((value) => value !== id),
    );

  const handleSave = async () => {
    const fields = closingDayFieldErrors(reason, startDate, endDate);
    if (schools && schoolIds.length === 0)
      fields.school_ids = "Bitte wählen Sie mindestens eine Schule.";
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    const body = {
      start_date: startDate,
      end_date: endDate,
      reason: reason.trim(),
    };
    await save.run(async (op) => {
      formErrors.clear();
      try {
        if (onSave) {
          await onSave({
            startDate,
            endDate,
            reason: reason.trim(),
            schoolIds,
          });
        } else if (initial) {
          await closingDayService.update(initial.id, body);
        } else {
          await closingDayService.create(body);
        }
        onSaved();
        // Eine späte Antwort schließt keinen inzwischen neuen Dialog (#6533).
        if (!op.isCurrent()) return;
        onClose();
        // #3594: Termine, die vor dem Schließtag geplant waren, bleiben im Plan.
        // Stehen noch welche im Zeitraum, bietet der Editor das Absagen an.
        await offerCancel(onOfferCancel, startDate, endDate);
      } catch (err) {
        logger.error("closing_day_save_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        await formErrors.show(err, {
          object: "das Speichern des Schließtags",
          retry: () => latestSaveRef.current(),
        });
      }
    });
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
      pending={save}
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
            disabled={saving || saveBlocked}
          >
            {saving ? "Speichern..." : saveLabel}
          </Button>
        </div>
      }
    >
      <div ref={formRef} className="space-y-4">
        <Input
          id="closing-day-reason"
          name="reason"
          type="text"
          label="Grund"
          value={reason}
          maxLength={reasonMaxLength}
          onChange={(e) => setReason(e.target.value)}
          error={formErrors.fieldError("reason")}
          placeholder="z. B. Pädagogischer Tag, Sommerschließung"
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <ISODatePicker
            id="closing-day-start"
            name="start_date"
            label="Von"
            controlSize="lg"
            value={startDate}
            onChange={(value) => {
              setStartDate(value);
              if (schools && (!endDate || endDate < value)) setEndDate(value);
            }}
            error={formErrors.fieldError("start_date")}
            calendarLayout="popover"
          />
          <ISODatePicker
            id="closing-day-end"
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

        {schools ? (
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
        ) : null}

        {consequences}

        <p className="text-xs text-gray-500">
          {schools
            ? "An diesem Tag ist die OGS zu. Es gibt keine Ferienbetreuung, Termine fallen aus und Mitarbeitende haben kein Soll."
            : "Für einen einzelnen Tag dasselbe Datum in beide Felder eintragen. An Schließtagen fallen Termine aus und Mitarbeitende haben kein Soll."}
        </p>
      </div>
    </FormModal>
  );
}

/** The days and schools of the draft; the reason changes no effect. */
export interface ClosingDayDraft {
  readonly startDate: string;
  readonly endDate: string;
  readonly schoolIds: readonly string[];
}

interface ClosingDayModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSaved: () => void;
  readonly initial:
    (ClosingDay & { readonly schoolIds?: readonly string[] }) | null;
  readonly schools?: readonly { readonly id: string; readonly name: string }[];
  readonly defaultDate?: string;
  readonly reasonMaxLength?: number;
  /** Persists a range outside the school's closing-day service. */
  readonly onSave?: (input: {
    startDate: string;
    endDate: string;
    reason: string;
    schoolIds: readonly string[];
  }) => Promise<void>;
  /** Reports the draft as it changes, so the page can show its effects. */
  readonly onDraftChange?: (draft: ClosingDayDraft) => void;
  /** What the draft means beyond the calendar, such as booked holiday care. */
  readonly consequences?: ReactNode;
  /** Label of the save button, e.g. when saving also tells parents. */
  readonly saveLabel?: string;
  /** Holds the save while the consequences are still loading. */
  readonly saveBlocked?: boolean;
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
