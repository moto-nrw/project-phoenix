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
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
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
import { timetableRequiredMark } from "./timetable-style";

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
  /**
   * Advisory reference counts for the edited period. When provided, the
   * delete confirmation names the concrete usage instead of a generic
   * warning. Deleting never blocks — all FKs are ON DELETE SET NULL.
   */
  usage?: {
    enrollmentPhaseCount: number;
    activityGroupCount: number;
    scheduleCount: number;
    studentEnrollmentCount: number;
    supervisorCount: number;
    activityInstanceCount: number;
  };
  /**
   * Enables the "Verknüpfte Anmeldephasen" section (edit mode only) so the
   * link can be managed from the period side too. The FK lives on the
   * phase, so toggling writes through the phase API — onToggle persists
   * immediately, independent of the period form's save button. Kept as a
   * prop so the timetable page (which shares this modal) stays unchanged.
   */
  phaseLink?: {
    phases: LinkablePhase[];
    onToggle: (phase: LinkablePhase, link: boolean) => Promise<void>;
  };
}

/**
 * Structural subset of an enrollment phase — deliberately not imported
 * from the enrollment lib to keep this shared timetable component
 * decoupled from that domain.
 */
export interface LinkablePhase {
  id: string;
  name: string;
  calendar_period_id?: string | null;
  is_active: boolean;
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
  phaseLink,
}: CalendarPeriodModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [togglingPhaseId, setTogglingPhaseId] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [saveWarnings, setSaveWarnings] = useState<CalendarPeriodWarning[]>([]);
  // Once a save succeeded in this modal session, further submits must update
  // that period — otherwise a corrected re-submit after an advisory warning
  // would create a duplicate in create mode.
  const [savedPeriod, setSavedPeriod] = useState<CalendarPeriod | null>(null);

  const isEdit = Boolean(initial);
  const persisted = initial ?? savedPeriod;

  // Most period links are nullable and get unlinked on delete. The server can
  // still reject the mutation when unpinning would invalidate a care-offering
  // link or create duplicate active unscoped roster assignments, so the
  // warning must not promise success.
  const usageText = usage
    ? formatPeriodUsage(
        usage.enrollmentPhaseCount,
        usage.scheduleCount,
        " und ",
        {
          activityGroupCount: usage.activityGroupCount,
          studentEnrollmentCount: usage.studentEnrollmentCount,
          supervisorCount: usage.supervisorCount,
          activityInstanceCount: usage.activityInstanceCount,
        },
      )
    : "";
  const groupDeleteImpact =
    (usage?.activityGroupCount ?? 0) > 0
      ? " Aktivitätsvorlagen verlieren dabei ihre Zeitraumfestlegung."
      : "";
  const deleteWarning = usageText
    ? `Dieser Zeitraum wird von ${usageText} verwendet. Beim Löschen werden diese Verknüpfungen entfernt.${groupDeleteImpact} Die Anmeldephasen, Vorlagen und Termine selbst bleiben erhalten.`
    : "Beim Löschen werden bestehende Verknüpfungen zu Anmeldephasen, Aktivitätsvorlagen und Regelterminen entfernt.";
  const deleteConflictHint =
    "Der Server verweigert das Löschen, wenn dadurch eine Betreuungsangebot-Verknüpfung ungültig würde. Auch doppelte aktive Kinder- oder Personalzuordnungen ohne Zeitraum können das Löschen verhindern.";

  useEffect(() => {
    if (!isOpen) return;
    setForm(
      initial ? formFromPeriod(initial) : { ...emptyForm(), ...createDefaults },
    );
    setValidationError(null);
    setDeleteConfirm(false);
    setSaveWarnings([]);
    setSavedPeriod(null);
  }, [isOpen, initial, createDefaults]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
    // Editing after a warning-bearing save dismisses the warning state so the
    // footer returns to Abbrechen + Speichern and the correction can be saved.
    setSaveWarnings([]);
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
      // Already saved and untouched since (any edit clears the warnings);
      // Enter mirrors the "Schließen" primary action.
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

      const { period, warnings } = persisted
        ? await calendarPeriodService.update(persisted.id, body)
        : await calendarPeriodService.create(body);

      toastSuccess(
        persisted
          ? `Kalenderzeitraum "${period.name}" aktualisiert`
          : `Kalenderzeitraum "${period.name}" angelegt`,
      );
      setSavedPeriod(period);
      onSaved(period);
      if (warnings.length === 0) {
        onClose();
      } else {
        // Advisory only: the save succeeded. Keep the modal open so the
        // user reads the overlap warning, then closes it explicitly (or
        // edits the form, which re-enables saving).
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

  const handlePhaseToggle = async (phase: LinkablePhase, link: boolean) => {
    if (!phaseLink) return;
    setTogglingPhaseId(phase.id);
    try {
      // Error handling (toast + reload) lives in the caller — the modal
      // only tracks the busy state so checkboxes can't race each other.
      await phaseLink.onToggle(phase, link);
    } finally {
      setTogglingPhaseId(null);
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
        persisted ? "Kalenderzeitraum bearbeiten" : "Kalenderzeitraum anlegen"
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
                  <p className="text-xs text-[#CC2626]">
                    {deleteWarning} {deleteConflictHint}
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
                {persisted ? "Speichern" : "Anlegen"}
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
          <CustomSelect
            id="period_type"
            ariaLabel="Art"
            value={form.periodType}
            options={PERIOD_TYPES.map((t) => ({
              value: t,
              label: PERIOD_TYPE_LABELS[t],
            }))}
            onChange={(next) => update("periodType", next as PeriodType)}
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Startdatum" htmlFor="start_date" required>
            <ISODatePicker
              id="start_date"
              value={form.startDate}
              onChange={(next) => update("startDate", next)}
              calendarLayout="popover"
            />
          </Field>
          <Field label="Enddatum" htmlFor="end_date" required>
            <ISODatePicker
              id="end_date"
              value={form.endDate}
              onChange={(next) => update("endDate", next)}
              calendarLayout="popover"
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
            <ISODatePicker
              id="cycle_anchor"
              value={form.weekCycleAnchor}
              onChange={(next) => update("weekCycleAnchor", next)}
              disabled={cycleLength <= 1}
              calendarLayout="popover"
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

        {isEdit && initial && phaseLink && phaseLink.phases.length > 0 && (
          <fieldset className="rounded-xl border border-gray-200 p-3">
            <legend className="px-1 text-xs font-semibold text-gray-700">
              Verknüpfte Anmeldephasen
            </legend>
            <div className="flex flex-col">
              {phaseLink.phases.map((phase) => {
                const linked = phase.calendar_period_id === initial.id;
                const linkedElsewhere = !linked && !!phase.calendar_period_id;
                return (
                  <label
                    key={phase.id}
                    className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm transition-colors hover:bg-gray-50"
                  >
                    <Checkbox
                      checked={linked}
                      disabled={togglingPhaseId !== null}
                      onChange={(e) =>
                        void handlePhaseToggle(phase, e.target.checked)
                      }
                    />
                    <span className="min-w-0 flex-1 truncate text-gray-800">
                      {phase.name}
                    </span>
                    {linkedElsewhere && (
                      <span className="shrink-0 text-xs text-[#F78C10]">
                        Mit anderem Zeitraum verknüpft
                      </span>
                    )}
                    {!phase.is_active && (
                      <span className="shrink-0 text-xs text-gray-400">
                        Inaktiv
                      </span>
                    )}
                  </label>
                );
              })}
            </div>
          </fieldset>
        )}

        {saveWarnings.map((warning) => (
          <Alert
            key={`${warning.code}-${warning.overlappingPeriodIds.join("-")}`}
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
