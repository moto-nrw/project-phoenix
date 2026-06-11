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

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { FormModal } from "~/components/ui/form-modal";
import { renderModalErrorAlert } from "~/components/ui/modal-utils";
import { useToast } from "~/contexts/ToastContext";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  type CalendarPeriod,
  PERIOD_TYPES,
  PERIOD_TYPE_LABELS,
  type PeriodType,
} from "~/lib/calendar-period-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CalendarPeriodModal" });
const FORM_SELECT_CLASS =
  "moto-select block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500";

interface CalendarPeriodModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (period: CalendarPeriod) => void;
  onDeleted?: (period: CalendarPeriod) => void;
  /** Pass an existing period to enter edit mode. Omit/null for create. */
  initial?: CalendarPeriod | null;
  /** Optional defaults used when creating from a visible planner week. */
  createDefaults?: Partial<FormState>;
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
}: CalendarPeriodModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  const isEdit = Boolean(initial);

  useEffect(() => {
    if (!isOpen) return;
    setForm(
      initial ? formFromPeriod(initial) : { ...emptyForm(), ...createDefaults },
    );
    setValidationError(null);
    setDeleteConfirm(false);
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
    if (!canSubmit) return;

    if (form.endDate <= form.startDate) {
      setValidationError("Enddatum muss nach dem Startdatum liegen.");
      return;
    }
    if (cycleLength > 1 && form.weekCycleAnchor === "") {
      setValidationError(
        "Bei einem Rhythmus über mehrere Wochen ist ein Startdatum für den Rhythmus erforderlich.",
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

      const result = isEdit
        ? await calendarPeriodService.update(initial!.id, body)
        : await calendarPeriodService.create(body);

      toastSuccess(
        isEdit
          ? `Planungszeitraum "${result.name}" aktualisiert`
          : `Planungszeitraum "${result.name}" angelegt`,
      );
      onSaved(result);
      onClose();
    } catch (err) {
      logger.error("period_save_failed", {
        mode: isEdit ? "edit" : "create",
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Planungszeitraum konnte nicht gespeichert werden";
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
      toastSuccess(`Planungszeitraum "${initial.name}" gelöscht`);
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
          : "Planungszeitraum konnte nicht gelöscht werden";
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
        isEdit ? "Planungszeitraum bearbeiten" : "Planungszeitraum anlegen"
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
                  <p className="text-xs text-[#FF3130]">
                    Löschen klappt nur, wenn dieser Zeitraum nicht mehr von
                    Regelterminen oder einzelnen Terminen verwendet wird.
                  </p>
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
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={onClose}
              disabled={submitting || deleting}
            >
              Abbrechen
            </Button>
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
          <Field
            label="Rhythmus in Wochen (1 = jede Woche)"
            htmlFor="cycle_length"
          >
            <Input
              id="cycle_length"
              type="number"
              min={1}
              max={4}
              value={form.weekCycleLength}
              controlSize="compact"
              onChange={(e) => update("weekCycleLength", e.target.value)}
            />
          </Field>
          <Field
            label="Start für Rhythmus"
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
          className="flex cursor-pointer items-center gap-2 rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-sm"
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

        {validationError && renderModalErrorAlert({ message: validationError })}
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
    <label htmlFor={htmlFor} className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-gray-700">
        {label}
        {required && <span className="ml-0.5 text-[#FF3130]">*</span>}
      </span>
      {children}
    </label>
  );
}
