"use client";

/**
 * CalendarPeriodModal creates or edits a planning window (Schuljahr,
 * Halbjahr, Ferien, Sonstiges). The same form drives both modes; pass an
 * `initial` to switch into edit mode.
 *
 * Why this matters: without an active calendar period covering a date,
 * the materialization service silently no-ops (see materialization_service.go
 * line ~213). This modal is the only UI path to fix that today.
 */

import { useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  type CalendarPeriod,
  type CalendarPeriodWarning,
  PERIOD_TYPES,
  PERIOD_TYPE_LABELS,
  type PeriodType,
  formatPeriodUsage,
} from "~/lib/calendar-period-helpers";
import { createLogger } from "~/lib/logger";
import { timetableRequiredMark, timetableSelectClass } from "./timetable-style";

const logger = createLogger({ component: "CalendarPeriodModal" });
const FORM_SELECT_CLASS = timetableSelectClass;

interface CalendarPeriodModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (period: CalendarPeriod) => void;
  onDeleted?: (period: CalendarPeriod) => void;
  /** Pass an existing period to enter edit mode. Omit/null for create. */
  initial?: CalendarPeriod | null;
  /** Optional defaults used when creating from a visible planner week. */
  createDefaults?: Partial<FormState>;
  /**
   * Advisory reference counts for the edited period. When provided, the
   * delete confirmation names the concrete usage instead of a generic
   * warning. Deleting never blocks — all FKs are ON DELETE SET NULL.
   */
  usage?: { enrollmentPhaseCount: number; scheduleCount: number };
}

interface FormState {
  name: string;
  periodType: PeriodType;
  startDate: string;
  endDate: string;
  weekCycleLength: string; // string for input control; coerced on submit
  weekCycleAnchor: string;
  isActive: boolean;
}

function emptyForm(): FormState {
  return {
    name: "",
    periodType: "school_year",
    startDate: "",
    endDate: "",
    weekCycleLength: "1",
    weekCycleAnchor: "",
    isActive: true,
  };
}

function formFromPeriod(p: CalendarPeriod): FormState {
  return {
    name: p.name,
    periodType: p.periodType,
    startDate: p.startDate,
    endDate: p.endDate,
    weekCycleLength: String(p.weekCycleLength),
    weekCycleAnchor: p.weekCycleAnchor ?? "",
    isActive: p.isActive,
  };
}

export function CalendarPeriodModal({
  isOpen,
  onClose,
  onSaved,
  onDeleted,
  initial,
  createDefaults,
  usage,
}: CalendarPeriodModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [saveWarnings, setSaveWarnings] = useState<CalendarPeriodWarning[]>([]);

  const isEdit = Boolean(initial);

  // Deletion never blocks (all FKs are ON DELETE SET NULL) — the warning
  // names what gets unlinked so the admin knows the blast radius.
  const usageText = usage
    ? formatPeriodUsage(
        usage.enrollmentPhaseCount,
        usage.scheduleCount,
        " und ",
      )
    : "";
  const deleteWarning = usageText
    ? `Dieser Zeitraum wird von ${usageText} verwendet. Beim Löschen werden diese Verknüpfungen entfernt, die Anmeldephasen und Termine selbst bleiben erhalten.`
    : "Beim Löschen werden bestehende Verknüpfungen zu Anmeldephasen und Regelterminen entfernt.";

  useEffect(() => {
    if (!isOpen) return;
    setForm(
      initial ? formFromPeriod(initial) : { ...emptyForm(), ...createDefaults },
    );
    setValidationError(null);
    setDeleteConfirm(false);
    setSaveWarnings([]);
  }, [isOpen, initial, createDefaults]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
  };

  const cycleLength = useMemo(() => {
    const n = Number.parseInt(form.weekCycleLength, 10);
    return Number.isFinite(n) && n >= 1 ? n : 1;
  }, [form.weekCycleLength]);

  const canSubmit =
    form.name.trim().length > 0 &&
    form.startDate !== "" &&
    form.endDate !== "" &&
    !submitting;

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (saveWarnings.length > 0) {
      // Already saved; Enter mirrors the "Schließen" primary action so a
      // re-submit cannot create a duplicate period.
      onClose();
      return;
    }
    if (!canSubmit) return;

    if (form.endDate <= form.startDate) {
      setValidationError("Enddatum muss nach dem Startdatum liegen.");
      return;
    }
    if (cycleLength > 1 && form.weekCycleAnchor === "") {
      setValidationError(
        "Bei einer Wiederholung über mehrere Wochen ist das Startdatum der Wiederholung erforderlich.",
      );
      return;
    }

    setSubmitting(true);
    try {
      const body = {
        name: form.name.trim(),
        period_type: form.periodType,
        start_date: form.startDate,
        end_date: form.endDate,
        week_cycle_length: cycleLength,
        is_active: form.isActive,
        ...(form.weekCycleAnchor
          ? { week_cycle_anchor: form.weekCycleAnchor }
          : {}),
      };

      const { period, warnings } = isEdit
        ? await calendarPeriodService.update(initial!.id, body)
        : await calendarPeriodService.create(body);

      toastSuccess(
        isEdit
          ? `Kalenderzeitraum "${period.name}" aktualisiert`
          : `Kalenderzeitraum "${period.name}" angelegt`,
      );
      onSaved(period);
      if (warnings.length === 0) {
        onClose();
      } else {
        // Advisory only: the save succeeded. Keep the modal open so the
        // user reads the overlap warning, then closes it explicitly.
        setSaveWarnings(warnings);
      }
    } catch (err) {
      logger.error("period_save_failed", {
        mode: isEdit ? "edit" : "create",
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Kalenderzeitraum konnte nicht gespeichert werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!initial) return;
    if (!deleteConfirm) {
      setDeleteConfirm(true);
      return;
    }
    setDeleting(true);
    try {
      await calendarPeriodService.delete(initial.id);
      toastSuccess(`Kalenderzeitraum "${initial.name}" gelöscht`);
      onDeleted?.(initial);
      onClose();
    } catch (err) {
      logger.error("period_delete_failed", {
        period_id: initial.id,
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Kalenderzeitraum konnte nicht gelöscht werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      size="md"
      title={
        isEdit ? "Kalenderzeitraum bearbeiten" : "Kalenderzeitraum anlegen"
      }
      footer={
        <div className="flex w-full items-center justify-between gap-2">
          <div>
            {isEdit && (
              <div className="flex max-w-sm flex-col gap-1">
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
                  onClick={() => void handleDelete()}
                  isLoading={deleting}
                  loadingText="Lösche …"
                  disabled={submitting}
                >
                  {deleteConfirm ? "Löschen bestätigen" : "Löschen"}
                </Button>
                {deleteConfirm && !deleting && (
                  <p className="text-xs text-[#CC2626]">{deleteWarning}</p>
                )}
              </div>
            )}
          </div>
          <div className="flex items-center justify-end gap-2">
            {deleteConfirm && !deleting && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => setDeleteConfirm(false)}
              >
                Löschen abbrechen
              </Button>
            )}
            {saveWarnings.length === 0 && (
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={onClose}
                disabled={submitting || deleting}
              >
                Abbrechen
              </Button>
            )}
            {saveWarnings.length > 0 ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={onClose}
              >
                Schließen
              </Button>
            ) : (
              <Button
                type="submit"
                form="calendar-period-form"
                variant="primary"
                size="md"
                isLoading={submitting}
                loadingText="Speichere …"
                disabled={!canSubmit || deleting}
              >
                {isEdit ? "Speichern" : "Anlegen"}
              </Button>
            )}
          </div>
        </div>
      }
    >
      <form
        id="calendar-period-form"
        onSubmit={(e) => void handleSubmit(e)}
        className="flex flex-col gap-4"
      >
        <Field label="Bezeichnung" htmlFor="name" required>
          <Input
            id="name"
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
            placeholder="z. B. Schuljahr 2025/2026"
            maxLength={255}
            controlSize="compact"
            required
            autoFocus
          />
        </Field>

        <Field label="Art" htmlFor="period_type" required>
          <select
            id="period_type"
            value={form.periodType}
            onChange={(e) => update("periodType", e.target.value as PeriodType)}
            className={FORM_SELECT_CLASS}
          >
            {PERIOD_TYPES.map((t) => (
              <option key={t} value={t}>
                {PERIOD_TYPE_LABELS[t]}
              </option>
            ))}
          </select>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Startdatum" htmlFor="start_date" required>
            <Input
              id="start_date"
              type="date"
              value={form.startDate}
              controlSize="compact"
              onChange={(e) => update("startDate", e.target.value)}
              required
            />
          </Field>
          <Field label="Enddatum" htmlFor="end_date" required>
            <Input
              id="end_date"
              type="date"
              value={form.endDate}
              controlSize="compact"
              onChange={(e) => update("endDate", e.target.value)}
              required
            />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Wiederholung in Wochen" htmlFor="cycle_length">
            <Input
              id="cycle_length"
              type="number"
              min={1}
              max={4}
              value={form.weekCycleLength}
              controlSize="compact"
              onChange={(e) => update("weekCycleLength", e.target.value)}
            />
            <span className="text-xs font-normal text-gray-500">
              1 = jede Woche, 2 = alle 2 Wochen
            </span>
          </Field>
          <Field
            label="Startdatum der Wiederholung"
            htmlFor="cycle_anchor"
            required={cycleLength > 1}
          >
            <Input
              id="cycle_anchor"
              type="date"
              value={form.weekCycleAnchor}
              controlSize="compact"
              onChange={(e) => update("weekCycleAnchor", e.target.value)}
              disabled={cycleLength <= 1}
            />
          </Field>
        </div>

        <label
          htmlFor="period_active"
          className="flex cursor-pointer items-center gap-2 rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm transition-colors hover:bg-gray-50"
        >
          <Checkbox
            id="period_active"
            checked={form.isActive}
            onChange={(e) => update("isActive", e.target.checked)}
          />
          <span className="font-semibold text-gray-700">
            Zeitraum im Plan verwenden
          </span>
          <span className="text-xs text-gray-500">
            Nur aktive Zeiträume legen Termine aus Regelterminen an
          </span>
        </label>

        {saveWarnings.map((warning, index) => (
          <Alert
            key={`${warning.code}-${index}`}
            type="warning"
            message={warning.message}
          />
        ))}

        {validationError && <Alert type="error" message={validationError} />}
      </form>
    </FormModal>
  );
}

interface FieldProps {
  label: string;
  htmlFor: string;
  required?: boolean;
  children: React.ReactNode;
}

function Field({ label, htmlFor, required = false, children }: FieldProps) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={htmlFor} className="text-xs font-semibold text-gray-700">
        {label}
        {required && <span className={timetableRequiredMark}>*</span>}
      </label>
      {children}
    </div>
  );
}
