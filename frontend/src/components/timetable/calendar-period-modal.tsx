"use client";

/**
 * CalendarPeriodModal — create or edit a Kalenderperiode (Schuljahr,
 * Halbjahr, Ferien, Sonstiges). The same form drives both modes; pass an
 * `initial` to switch into edit mode.
 *
 * Why this matters: without an active calendar period covering a date,
 * the materialization service silently no-ops (see materialization_service.go
 * line ~213). This modal is the only UI path to fix that today.
 */

import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
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
        "Bei mehrwöchigem Rhythmus ist ein Anker-Datum erforderlich.",
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
          ? `Periode "${result.name}" aktualisiert`
          : `Periode "${result.name}" angelegt`,
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
          : "Periode konnte nicht gespeichert werden";
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
      toastSuccess(`Periode "${initial.name}" gelöscht`);
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
          : "Periode konnte nicht gelöscht werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEdit ? "Kalenderperiode bearbeiten" : "Kalenderperiode anlegen"}
      footer={
        <div className="flex w-full items-center justify-between gap-2">
          <div>
            {isEdit && (
              <Button
                type="button"
                variant="outline_danger"
                size="sm"
                onClick={() => void handleDelete()}
                isLoading={deleting}
                loadingText="Lösche …"
                disabled={submitting}
              >
                {deleteConfirm ? "Löschen bestätigen" : "Löschen"}
              </Button>
            )}
          </div>
          <div className="flex items-center justify-end gap-2">
            {deleteConfirm && !deleting && (
              <button
                type="button"
                className="text-xs font-semibold text-slate-500 hover:text-slate-700"
                onClick={() => setDeleteConfirm(false)}
              >
                Löschen abbrechen
              </button>
            )}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onClose}
              disabled={submitting || deleting}
            >
              Abbrechen
            </Button>
            <Button
              type="submit"
              form="calendar-period-form"
              variant="primary"
              size="sm"
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
        <Field label="Name" htmlFor="name" required>
          <Input
            id="name"
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
            placeholder="z. B. Schuljahr 2025/2026"
            maxLength={255}
            required
            autoFocus
          />
        </Field>

        <Field label="Typ" htmlFor="period_type" required>
          <select
            id="period_type"
            value={form.periodType}
            onChange={(e) => update("periodType", e.target.value as PeriodType)}
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
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
              onChange={(e) => update("startDate", e.target.value)}
              required
            />
          </Field>
          <Field label="Enddatum" htmlFor="end_date" required>
            <Input
              id="end_date"
              type="date"
              value={form.endDate}
              onChange={(e) => update("endDate", e.target.value)}
              required
            />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Wochenzyklus (1 = jede Woche)" htmlFor="cycle_length">
            <Input
              id="cycle_length"
              type="number"
              min={1}
              max={4}
              value={form.weekCycleLength}
              onChange={(e) => update("weekCycleLength", e.target.value)}
            />
          </Field>
          <Field
            label="Anker-Datum"
            htmlFor="cycle_anchor"
            required={cycleLength > 1}
          >
            <Input
              id="cycle_anchor"
              type="date"
              value={form.weekCycleAnchor}
              onChange={(e) => update("weekCycleAnchor", e.target.value)}
              disabled={cycleLength <= 1}
            />
          </Field>
        </div>

        <label className="flex cursor-pointer items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm">
          <input
            type="checkbox"
            checked={form.isActive}
            onChange={(e) => update("isActive", e.target.checked)}
            className="h-4 w-4"
          />
          <span className="font-semibold text-slate-700">
            Periode ist aktiv
          </span>
          <span className="text-xs text-slate-500">
            Nur aktive Perioden materialisieren Templates
          </span>
        </label>

        {validationError && (
          <div
            role="alert"
            className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]"
          >
            {validationError}
          </div>
        )}
      </form>
    </Modal>
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
      <span className="text-xs font-semibold text-slate-700">
        {label}
        {required && <span className="ml-0.5 text-[#FF3130]">*</span>}
      </span>
      {children}
    </label>
  );
}
