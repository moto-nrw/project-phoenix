"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, Plus, StickyNote } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { EditActions } from "~/components/ui/edit-actions";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import {
  completesOneDigitHour,
  normalizeTimeInput,
} from "~/components/ui/time-field";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
} from "~/lib/arrival-schedule-helpers";
import {
  formatDateISO,
  type PickupScheduleFormData,
  type WeekdayNoteEntry,
} from "~/lib/pickup-schedule-helpers";
import type {
  PickupAdjustmentPreview,
  PickupAdjustmentResolution,
  PickupAdjustmentSelection,
} from "~/lib/pickup-schedule-api";
import type {
  CareDaysSource,
  CareTimePresets,
  SchoolPeriod,
} from "~/lib/student-arrival-api";
import { PickupAdjustmentDecision } from "./pickup-adjustment-decision";
import { SchoolPeriodSelect } from "./school-period-select";

/**
 * Der Wochenplan eines Kindes: die festen Ankunfts- und Abholzeiten je
 * Wochentag samt der Notizen, die jede Woche gelten. Es gibt EIN Raster dafür
 * (#3119): der Reiter „Betreuungszeiten“ wechselt damit in den
 * Bearbeiten-Zustand (`CareWeeklyPlanEditForm`, Bauart 2 Regeln 3 und 4), und
 * der Anlege-Assistent zeigt dasselbe Raster in seiner `FormModal`-Hülle
 * (`care-weekly-plan-modal.tsx`), weil es das Kind dort noch nicht gibt.
 * Ausnahmen für einen einzelnen Tag laufen weiter über
 * `care-plan-editor-modal.tsx`.
 */

export interface CarePlanWeeklySubmit {
  readonly arrivalSchedules: ArrivalScheduleFormEntry[];
  readonly pickupSchedules: PickupScheduleFormData[];
  /**
   * Notes of weekdays without a pickup time (#3369). They cannot ride on the
   * pickup row: that row's existence marks the child as expected. Absent
   * means: leave the stored weekday notes untouched.
   */
  readonly weekdayNotes?: WeekdayNoteEntry[];
}

export interface CarePlanWeeklyAdjustment {
  readonly resolution: PickupAdjustmentResolution;
  readonly preview: PickupAdjustmentPreview;
  readonly selections?: PickupAdjustmentSelection[];
  readonly effectiveFrom?: string;
  readonly reason?: string;
  /** false refreshes the offering consequences; true performs the write. */
  readonly confirm: boolean;
  readonly completeWithdrawalConfirmed?: boolean;
}

export interface WeeklyRow {
  readonly weekday: number;
  /** The child is in care that weekday (#2414). */
  readonly arrivalInCare: boolean;
  /** The child's OWN arrival time. Empty means the class time applies. */
  readonly arrivalTime: string;
  /** The class time that applies when the child has none. Read-only. */
  readonly arrivalClassTime: string;
  readonly arrivalNotes: string;
  readonly pickupTime: string;
  readonly pickupNotes: string;
}

const NO_WEEKDAY_NOTES: WeekdayNoteEntry[] = [];

type WeeklyRowField =
  "arrivalTime" | "pickupTime" | "arrivalNotes" | "pickupNotes";

const TIME_PATTERN = /^([01]?\d|2[0-3]):[0-5]\d$/;

// --- pure helpers -----------------------------------------------------------

function buildWeeklyRows(
  arrival: readonly ArrivalScheduleFormEntry[],
  pickup: readonly PickupScheduleFormData[],
  weekdayNotes: readonly WeekdayNoteEntry[],
): WeeklyRow[] {
  return WEEKDAYS.map((day) => {
    const arrivalEntry = arrival.find(
      (schedule) => schedule.weekday === day.value,
    );
    const pickupEntry = pickup.find(
      (schedule) => schedule.weekday === day.value,
    );
    return {
      weekday: day.value,
      // A row exists = care day. Its time is the child's own only when it did
      // not come from the class timetable (#2414).
      arrivalInCare: arrivalEntry?.inCare ?? false,
      arrivalTime: arrivalEntry?.expected_arrival ?? "",
      arrivalClassTime: arrivalEntry?.classTime ?? "",
      arrivalNotes: arrivalEntry?.notes ?? "",
      pickupTime: pickupEntry?.pickupTime ?? "",
      // ONE note field per day: the pickup row carries it while the day has a
      // pickup time, the weekday note while it has none (#3369).
      pickupNotes:
        (pickupEntry?.notes ? pickupEntry.notes : undefined) ??
        weekdayNotes.find((note) => note.weekday === day.value)?.content ??
        "",
    };
  });
}

export function validateWeeklyRows(
  rows: readonly WeeklyRow[],
  /**
   * The detail page stores a note without pickup time as a weekday note
   * (#3369). The create assistant has no child yet to attach one to, so there
   * the note still needs its pickup time.
   */
  notesWithoutPickup = false,
): string | null {
  for (const row of rows) {
    const day = WEEKDAYS.find((weekday) => weekday.value === row.weekday);
    if (row.arrivalTime && !TIME_PATTERN.test(row.arrivalTime)) {
      return `Ungültige Ankunftszeit für ${day?.label ?? "diesen Tag"}.`;
    }
    if (row.arrivalNotes.trim() && !row.arrivalInCare) {
      return `Eine Ankunftsnotiz für ${day?.label ?? "diesen Tag"} braucht einen Betreuungstag.`;
    }
    if (row.pickupTime && !TIME_PATTERN.test(row.pickupTime)) {
      return `Ungültige Abholzeit für ${day?.label ?? "diesen Tag"}.`;
    }
    if (
      !notesWithoutPickup &&
      row.pickupNotes.trim() &&
      !row.pickupTime.trim()
    ) {
      return `Eine Abholnotiz für ${day?.label ?? "diesen Tag"} benötigt eine Abholzeit.`;
    }
  }
  return null;
}

/**
 * Which previously scheduled times an emptied field would delete. The weekly
 * write REPLACES the plan, so clearing a field really does remove that day —
 * this is what the warning and the confirmation are built from, so a mistyped
 * Friday cannot vanish silently.
 */
function collectWeeklyRemovals(
  rows: readonly WeeklyRow[],
  arrival: readonly ArrivalScheduleFormEntry[],
  pickup: readonly PickupScheduleFormData[],
): string[] {
  const removals: string[] = [];
  for (const row of rows) {
    const label =
      WEEKDAYS.find((day) => day.value === row.weekday)?.label ?? "Tag";
    // "Had" means it was a care day — the template carries an entry for every
    // weekday, care day or not (#2414).
    const hadArrival = arrival.some(
      (entry) => entry.weekday === row.weekday && entry.inCare,
    );
    const hadPickup = pickup.some(
      (entry) => entry.weekday === row.weekday && entry.pickupTime,
    );
    if (hadArrival && !row.arrivalInCare) {
      removals.push(`${label} Ankunft`);
    }
    if (hadPickup && !row.pickupTime.trim()) {
      removals.push(`${label} Abholung`);
    }
  }
  return removals;
}

export function toWeeklySubmit(
  rows: readonly WeeklyRow[],
  pickupNeedsCareDay = false,
): CarePlanWeeklySubmit {
  const keepsPickup = (row: WeeklyRow) =>
    row.pickupTime.trim() !== "" && (!pickupNeedsCareDay || row.arrivalInCare);
  return {
    arrivalSchedules: rows
      .filter((row) => row.arrivalInCare)
      .map((row) => ({
        weekday: row.weekday,
        inCare: true,
        // Empty = the class timetable supplies the time (#2414). Sending the
        // inherited value back would freeze it as a per-child deviation.
        expected_arrival: row.arrivalTime.trim(),
        notes: row.arrivalNotes.trim() ? row.arrivalNotes : null,
      })),
    pickupSchedules: rows.filter(keepsPickup).map((row) => ({
      weekday: row.weekday,
      pickupTime: row.pickupTime,
      notes: row.pickupNotes.trim() ? row.pickupNotes : undefined,
    })),
    weekdayNotes: rows
      .filter((row) => !keepsPickup(row) && row.pickupNotes.trim() !== "")
      .map((row) => ({
        weekday: row.weekday,
        content: row.pickupNotes.trim(),
      })),
  };
}

function offeringRemovesAllCareDays(
  preview: PickupAdjustmentPreview,
  selections: PickupAdjustmentSelection[],
) {
  if (!preview.offering_catalog) return false;
  const selected = new Map(
    selections.map((selection) => [selection.offering_id, selection]),
  );
  return !preview.offering_catalog.items.some(
    (item) =>
      item.counts_as_care &&
      (item.days_of_week_mode === "fixed"
        ? selected.has(item.offering_id) && item.available_days.length > 0
        : (selected.get(item.offering_id)?.selected_days.length ?? 0) > 0),
  );
}

// --- draft state ------------------------------------------------------------

export interface WeeklyPlanDraft {
  readonly rows: WeeklyRow[];
  readonly expandedWeekdays: Set<number>;
  readonly setField: (
    weekday: number,
    field: WeeklyRowField,
    value: string,
  ) => void;
  readonly toggleCare: (weekday: number, inCare: boolean) => void;
  readonly toggleNotes: (weekday: number) => void;
}

/**
 * The rows of one editing session. Seeded ONCE, when the editing component
 * mounts: a later change of the schedule props (a remote refresh, a re-render
 * with new references) must not discard what the person has typed. Whoever
 * needs a fresh draft mounts the editor anew.
 */
export function useWeeklyPlanDraft(
  initialArrival: readonly ArrivalScheduleFormEntry[],
  initialPickup: readonly PickupScheduleFormData[],
  initialWeekdayNotes: readonly WeekdayNoteEntry[] = [],
): WeeklyPlanDraft {
  const [rows, setRows] = useState<WeeklyRow[]>(() =>
    buildWeeklyRows(initialArrival, initialPickup, initialWeekdayNotes),
  );
  const [expandedWeekdays, setExpandedWeekdays] = useState<Set<number>>(
    () =>
      new Set(
        rows
          .filter((row) => row.arrivalNotes || row.pickupNotes)
          .map((row) => row.weekday),
      ),
  );

  const setField = (weekday: number, field: WeeklyRowField, value: string) =>
    setRows((current) =>
      current.map((row) =>
        row.weekday === weekday ? { ...row, [field]: value } : row,
      ),
    );

  const toggleCare = (weekday: number, inCare: boolean) =>
    setRows((current) =>
      current.map((row) =>
        row.weekday === weekday
          ? {
              ...row,
              arrivalInCare: inCare,
              arrivalTime: inCare ? row.arrivalTime : "",
              arrivalNotes: inCare ? row.arrivalNotes : "",
            }
          : row,
      ),
    );

  const toggleNotes = (weekday: number) =>
    setExpandedWeekdays((current) => {
      const next = new Set(current);
      if (next.has(weekday)) {
        next.delete(weekday);
      } else {
        next.add(weekday);
      }
      return next;
    });

  return { rows, expandedWeekdays, setField, toggleCare, toggleNotes };
}

// --- the grid ---------------------------------------------------------------

const NO_SCHOOL_PERIODS: readonly SchoolPeriod[] = [];
const NO_CARE_TIME_PRESETS: CareTimePresets = { arrival: "", pickup: "" };

const GRID_COLUMNS =
  "sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)]";

export function CareWeeklyPlanGrid({
  draft,
  careDaysSource,
  removals,
  disabled = false,
  pickupNeedsCareDay = false,
  schoolPeriods = NO_SCHOOL_PERIODS,
  timePresets = NO_CARE_TIME_PRESETS,
  notesWithoutPickup = false,
}: {
  readonly draft: WeeklyPlanDraft;
  readonly careDaysSource: CareDaysSource;
  /** Lessons the arrival can be picked by (#3372); none hides the choice. */
  readonly schoolPeriods?: readonly SchoolPeriod[];
  /** The school's usual times, offered for one click into an empty field. */
  readonly timePresets?: CareTimePresets;
  /** Times the save would delete; shown as a warning under the grid. */
  readonly removals: readonly string[];
  /** Every field is locked while a save is running. */
  readonly disabled?: boolean;
  /** In bookings mode a pickup time is only possible on a booked care day. */
  readonly pickupNeedsCareDay?: boolean;
  /** A day without pickup time can still carry a note (#3369). */
  readonly notesWithoutPickup?: boolean;
}) {
  const { rows, expandedWeekdays, setField, toggleCare, toggleNotes } = draft;
  return (
    <div className="space-y-3">
      <div className="moto-content-surface overflow-hidden rounded-xl border shadow-sm sm:rounded-2xl">
        <div
          className={`hidden gap-3 border-b border-gray-100 bg-gray-50 px-4 py-3 text-xs font-semibold tracking-wide text-gray-500 uppercase sm:grid ${GRID_COLUMNS}`}
        >
          <span>Betreuungstag</span>
          <span>Ankunft</span>
          <span>Abholung</span>
        </div>
        <div className="divide-y divide-gray-100">
          {WEEKDAYS.map((day) => {
            const row = rows.find((entry) => entry.weekday === day.value);
            const inCare = row?.arrivalInCare ?? false;
            const pickupLocked =
              disabled ||
              (pickupNeedsCareDay && careDaysSource === "bookings" && !inCare);
            // Without a pickup time the note belongs to the day, not to an
            // Abholung the child does not have (#3369).
            const noteIsForDay =
              notesWithoutPickup &&
              (pickupLocked || !(row?.pickupTime ?? "").trim());
            // The usual time is offered only where it becomes the child's
            // time: never over the class time, whose empty field already
            // means a time (#2414), and never on a day that is no care day,
            // whose pickup the save drops (#3371).
            const arrivalPreset =
              inCare && !row?.arrivalTime && !row?.arrivalClassTime
                ? timePresets.arrival
                : "";
            const pickupPreset =
              inCare && !pickupLocked && !row?.pickupTime
                ? timePresets.pickup
                : "";
            return (
              <div
                key={day.value}
                className={`grid gap-3 px-3 py-4 sm:items-start sm:px-4 ${GRID_COLUMNS}`}
              >
                <div>
                  <label
                    htmlFor={`weekly-care-${day.value}`}
                    className="flex min-h-6 cursor-pointer items-center gap-2 text-sm font-semibold text-gray-900 has-[:disabled]:cursor-not-allowed sm:min-h-10"
                  >
                    <Checkbox
                      id={`weekly-care-${day.value}`}
                      checked={inCare}
                      disabled={disabled || careDaysSource === "bookings"}
                      onChange={(event) =>
                        toggleCare(day.value, event.target.checked)
                      }
                    />
                    <span>{day.label}</span>
                  </label>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    className="mt-1 gap-1 px-0 text-xs text-gray-500 hover:text-gray-800"
                    aria-expanded={expandedWeekdays.has(day.value)}
                    onClick={() => toggleNotes(day.value)}
                  >
                    <StickyNote className="h-3.5 w-3.5" aria-hidden="true" />
                    Notizen
                    <ChevronDown
                      className={`h-3.5 w-3.5 transition-transform ${
                        expandedWeekdays.has(day.value) ? "rotate-180" : ""
                      }`}
                      aria-hidden="true"
                    />
                  </Button>
                </div>
                <div>
                  <WeeklyTimeField
                    id={`weekly-arrival-${day.value}`}
                    label="Ankunft"
                    value={row?.arrivalTime ?? ""}
                    disabled={disabled || !inCare}
                    onChange={(value) =>
                      setField(day.value, "arrivalTime", value)
                    }
                  />
                  <TimePresetButton
                    time={arrivalPreset}
                    ariaLabel={`${day.label}: Ankunft ${arrivalPreset} Uhr eintragen`}
                    disabled={disabled}
                    onApply={() =>
                      setField(day.value, "arrivalTime", arrivalPreset)
                    }
                  />
                  <div className="mt-1 empty:hidden">
                    <SchoolPeriodSelect
                      id={`weekly-arrival-period-${day.value}`}
                      periods={schoolPeriods}
                      time={row?.arrivalTime ?? ""}
                      ariaLabel={`${day.label}: Ankunft nach Schulstunde`}
                      disabled={disabled || !inCare}
                      onSelect={(endTime) =>
                        setField(day.value, "arrivalTime", endTime)
                      }
                    />
                  </div>
                  {inCare && row?.arrivalClassTime ? (
                    <div className="mt-1 flex flex-wrap items-center justify-between gap-1">
                      <p className="text-xs text-gray-500">
                        Ohne Eintrag gilt {row.arrivalClassTime} Uhr aus der
                        Klasse.
                      </p>
                      {row.arrivalTime ? (
                        <Button
                          type="button"
                          variant="ghost"
                          size="compact"
                          disabled={disabled}
                          onClick={() => setField(day.value, "arrivalTime", "")}
                        >
                          Klassenzeit nutzen
                        </Button>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <div>
                  <WeeklyTimeField
                    id={`weekly-pickup-${day.value}`}
                    label="Abholung"
                    value={row?.pickupTime ?? ""}
                    disabled={pickupLocked}
                    onChange={(value) =>
                      setField(day.value, "pickupTime", value)
                    }
                  />
                  <TimePresetButton
                    time={pickupPreset}
                    ariaLabel={`${day.label}: Abholung ${pickupPreset} Uhr eintragen`}
                    disabled={disabled}
                    onApply={() =>
                      setField(day.value, "pickupTime", pickupPreset)
                    }
                  />
                </div>
                {expandedWeekdays.has(day.value) ? (
                  <div className={`grid gap-3 sm:col-span-3 ${GRID_COLUMNS}`}>
                    <div className="hidden sm:block" />
                    <WeeklyNoteField
                      id={`weekly-arrival-notes-${day.value}`}
                      label="Ankunftsnotiz (jede Woche)"
                      value={row?.arrivalNotes ?? ""}
                      disabled={disabled || !inCare}
                      onChange={(value) =>
                        setField(day.value, "arrivalNotes", value)
                      }
                    />
                    <WeeklyNoteField
                      id={`weekly-pickup-notes-${day.value}`}
                      label={
                        noteIsForDay
                          ? "Notiz zum Tag (jede Woche)"
                          : "Abholnotiz (jede Woche)"
                      }
                      value={row?.pickupNotes ?? ""}
                      disabled={notesWithoutPickup ? disabled : pickupLocked}
                      onChange={(value) =>
                        setField(day.value, "pickupNotes", value)
                      }
                    />
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>

      {removals.length > 0 ? (
        <div className="border-moto-orange/25 bg-moto-orange/10 text-moto-orange-strong rounded-xl border px-4 py-3 text-sm">
          Wird beim Speichern entfernt: {removals.join(", ")}.
        </div>
      ) : null}
    </div>
  );
}

function WeeklyTimeField({
  id,
  label,
  value,
  disabled,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly disabled: boolean;
  readonly onChange: (value: string) => void;
}) {
  const completedOneDigitHour = useRef<string | null>(null);

  return (
    <div>
      {/* The column header names the field on wide screens; the label is
          only shown where the rows stack. It stays for screen readers, which
          do not associate the column header with the field. */}
      <label
        htmlFor={id}
        className="mb-1 block text-xs font-medium text-gray-500 sm:sr-only"
      >
        {label}
      </label>
      {/* A text field with the digit mask instead of the native time input
          (#3371): Safari fills an empty native field with the current time in
          grey, which reads like a stored value and has to be overwritten hour
          and minute apart. Here "1600" becomes 16:00 in one go. */}
      <Input
        id={id}
        type="text"
        inputMode="numeric"
        autoComplete="off"
        placeholder="HH:MM"
        maxLength={6}
        controlSize="compact"
        className="tabular-nums"
        value={value}
        disabled={disabled}
        onChange={(event) => {
          const nextValue = normalizeTimeInput(
            event.target.value,
            value,
            completedOneDigitHour.current === value,
          );
          completedOneDigitHour.current = completesOneDigitHour(
            event.target.value,
          )
            ? nextValue
            : null;
          onChange(nextValue);
        }}
      />
    </div>
  );
}

/**
 * Copies the school's usual time into the empty field above it (#3371).
 * Nothing is stored until the plan is saved. Renders nothing without a time.
 */
function TimePresetButton({
  time,
  ariaLabel,
  disabled,
  onApply,
}: {
  readonly time: string;
  readonly ariaLabel: string;
  readonly disabled: boolean;
  readonly onApply: () => void;
}) {
  if (!time) return null;
  return (
    <Button
      type="button"
      variant="surface"
      size="compact"
      className="mt-1.5"
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={onApply}
    >
      <Plus className="h-3.5 w-3.5" aria-hidden="true" />
      {time} Uhr eintragen
    </Button>
  );
}

function WeeklyNoteField({
  id,
  label,
  value,
  disabled,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly disabled: boolean;
  readonly onChange: (value: string) => void;
}) {
  return (
    <div>
      <label
        htmlFor={id}
        className="mb-1 block text-xs font-medium text-gray-500"
      >
        {label}
      </label>
      <Input
        id={id}
        type="text"
        controlSize="compact"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        maxLength={500}
        placeholder="Optional"
      />
    </div>
  );
}

// --- the edit state of the Betreuungszeiten section --------------------------

interface CareWeeklyPlanEditFormProps {
  readonly careDaysSource: CareDaysSource;
  readonly schoolPeriods?: readonly SchoolPeriod[];
  readonly timePresets?: CareTimePresets;
  readonly weeklyArrival: ArrivalScheduleFormEntry[];
  readonly weeklyPickup: PickupScheduleFormData[];
  /** The notes of weekdays without a pickup time (#3369). */
  readonly weeklyNotes?: WeekdayNoteEntry[];
  readonly onSubmitWeekly: (
    payload: CarePlanWeeklySubmit,
    adjustment?: CarePlanWeeklyAdjustment,
  ) => Promise<PickupAdjustmentPreview | void>;
  /** Leaves the edit state without saving. */
  readonly onCancel: () => void;
  /** The plan is written; the section returns to its display. */
  readonly onSaved: () => void;
}

/**
 * The Bearbeiten-Zustand of the „Betreuungszeiten“ section: the weekly grid
 * in place of the day cards, errors in the alert on top (Regel 5), one
 * `EditActions` below (Regel 4). Two questions open dialogs out of the save,
 * because each has its own flow: the confirmation before an emptied field
 * deletes a time, and the offering decision when the new pickup times no
 * longer match the booked offering (#2290).
 */
export function CareWeeklyPlanEditForm({
  careDaysSource,
  schoolPeriods,
  timePresets,
  weeklyArrival,
  weeklyPickup,
  weeklyNotes = NO_WEEKDAY_NOTES,
  onSubmitWeekly,
  onCancel,
  onSaved,
}: CareWeeklyPlanEditFormProps) {
  const toast = useToast();
  const draft = useWeeklyPlanDraft(weeklyArrival, weeklyPickup, weeklyNotes);
  const pickupNeedsCareDay = careDaysSource === "bookings";
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useFormError();
  const [showRemovalConfirm, setShowRemovalConfirm] = useState(false);
  const [weeklyAdjustment, setWeeklyAdjustment] =
    useState<PickupAdjustmentPreview | null>(null);
  const [offeringSelections, setOfferingSelections] = useState<
    PickupAdjustmentSelection[]
  >([]);
  const [offeringEffectiveFrom, setOfferingEffectiveFrom] = useState("");
  const [offeringReason, setOfferingReason] = useState("");
  const [offeringConfirmed, setOfferingConfirmed] = useState(false);
  const [selectedOfferingId, setSelectedOfferingId] = useState<string | null>(
    null,
  );
  const offeringPreviewRequestId = useRef(0);
  const weeklyExceptionPreview = useRef<PickupAdjustmentPreview | null>(null);
  const decisionHeadingRef = useRef<HTMLHeadingElement>(null);
  const decisionWasVisible = useRef(false);

  useEffect(() => {
    if (weeklyAdjustment && !decisionWasVisible.current) {
      decisionHeadingRef.current?.focus();
    }
    decisionWasVisible.current = weeklyAdjustment !== null;
  }, [weeklyAdjustment]);

  const weeklyRemovals = collectWeeklyRemovals(
    draft.rows,
    weeklyArrival,
    weeklyPickup,
  );

  const clearDecision = () => {
    offeringPreviewRequestId.current++;
    weeklyExceptionPreview.current = null;
    setWeeklyAdjustment(null);
    setOfferingSelections([]);
    setSelectedOfferingId(null);
    setOfferingConfirmed(false);
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (weeklyAdjustment || isSubmitting) return;
    setError(null);
    const invalid = validateWeeklyRows(draft.rows, true);
    if (invalid) {
      setError(invalid);
      return;
    }
    if (weeklyRemovals.length > 0) {
      setShowRemovalConfirm(true);
      return;
    }
    void performSave();
  };

  const performSave = async () => {
    setError(null);
    setIsSubmitting(true);
    try {
      const adjustment = await onSubmitWeekly(
        toWeeklySubmit(draft.rows, pickupNeedsCareDay),
      );
      if (adjustment) {
        weeklyExceptionPreview.current = adjustment;
        setWeeklyAdjustment(adjustment);
        setOfferingEffectiveFrom(adjustment.effective_from);
        setOfferingConfirmed(false);
        return;
      }
      toast.success("Wochenplan wurde gespeichert");
      onSaved();
    } catch (err) {
      const raw =
        err instanceof Error
          ? err.message
          : "Änderung konnte nicht gespeichert werden";
      // The backend refuses to let an account without a staff profile overwrite
      // a parent-set time. Surface that as a readable reason, not a raw 403.
      setError(
        raw.includes("staff_profile_required")
          ? "Diese Zeit wurde von den Eltern gesetzt und kann nur von Mitarbeitenden mit Personalprofil geändert werden."
          : raw,
      );
      clearDecision();
    } finally {
      setShowRemovalConfirm(false);
      setIsSubmitting(false);
    }
  };

  const saveWeeklyException = async () => {
    const preview = weeklyExceptionPreview.current;
    if (!preview) return;
    setError(null);
    setIsSubmitting(true);
    try {
      await onSubmitWeekly(toWeeklySubmit(draft.rows, pickupNeedsCareDay), {
        resolution: "exception",
        preview,
        reason: offeringReason,
        confirm: true,
      });
      toast.success("Dauerhafte Ausnahme wurde gespeichert");
      onSaved();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Dauerhafte Ausnahme konnte nicht gespeichert werden",
      );
      clearDecision();
    } finally {
      setIsSubmitting(false);
    }
  };

  const previewMatchingOffering = async (
    offeringId: string,
    effectiveFrom = offeringEffectiveFrom,
  ) => {
    if (!weeklyAdjustment?.offering_catalog) return;
    const match = weeklyAdjustment.matching_offerings.find(
      (item) => item.offering_id === offeringId,
    );
    const target = weeklyAdjustment.offering_catalog.items.find(
      (item) => item.offering_id === offeringId,
    );
    if (!match || !target) return;
    const selections = match.selections;
    const requestId = ++offeringPreviewRequestId.current;
    setSelectedOfferingId(null);
    setOfferingSelections([]);
    setOfferingConfirmed(false);

    setError(null);
    setIsSubmitting(true);
    try {
      const preview = await onSubmitWeekly(
        toWeeklySubmit(draft.rows, pickupNeedsCareDay),
        {
          resolution: "offering",
          preview: weeklyAdjustment,
          selections,
          effectiveFrom,
          reason: offeringReason,
          confirm: false,
        },
      );
      if (!preview) {
        throw new Error("Die Angebotsänderung konnte nicht geprüft werden.");
      }
      if (requestId !== offeringPreviewRequestId.current) return;
      if (
        !preview.matching_offerings.some(
          (item) => item.offering_id === offeringId,
        )
      ) {
        setWeeklyAdjustment(preview);
        return;
      }
      const refreshedTarget = preview.offering_catalog?.items.find(
        (item) => item.offering_id === offeringId,
      );
      if (
        refreshedTarget &&
        !refreshedTarget.selected &&
        refreshedTarget.capacity !== undefined &&
        refreshedTarget.free_slots === 0
      ) {
        throw new Error("Dieses Angebot hat keinen freien Platz mehr.");
      }
      setOfferingSelections(selections);
      setWeeklyAdjustment(preview);
      setSelectedOfferingId(offeringId);
      setOfferingConfirmed(false);
    } catch (err) {
      if (requestId !== offeringPreviewRequestId.current) return;
      setError(
        err instanceof Error
          ? err.message
          : "Angebot konnte nicht geprüft werden",
      );
      setOfferingSelections([]);
      setSelectedOfferingId(null);
      setOfferingConfirmed(false);
    } finally {
      setIsSubmitting(false);
    }
  };

  const saveMatchingOffering = async () => {
    if (!weeklyAdjustment || !offeringConfirmed) return;
    setError(null);
    setIsSubmitting(true);
    try {
      await onSubmitWeekly(toWeeklySubmit(draft.rows, pickupNeedsCareDay), {
        resolution: "offering",
        preview: weeklyAdjustment,
        selections: offeringSelections,
        effectiveFrom: offeringEffectiveFrom,
        reason: offeringReason,
        confirm: true,
        completeWithdrawalConfirmed: offeringRemovesAllCareDays(
          weeklyAdjustment,
          offeringSelections,
        ),
      });
      toast.success("Angebot und Wochenplan wurden geändert");
      onSaved();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Angebot konnte nicht geändert werden",
      );
      clearDecision();
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <>
      {/* noValidate: a half-cleared <input type="time"> (e.g. backspacing the
          hour of "15:00") reports validity.badInput, which makes the browser
          refuse the submit — the Save button then does nothing beyond a native
          bubble on an off-screen field. Such a field reads back as "", which
          is the removal the warning below already announces. */}
      <form
        noValidate
        onSubmit={handleSubmit}
        className="space-y-4"
        aria-label="Wochenplan bearbeiten"
      >
        {/* While the decision dialog is open its own error slot carries the
            message; the alert here speaks once the person is back on the
            grid. */}
        <FormErrorAlert message={weeklyAdjustment ? null : error} />
        <p className="text-sm leading-6 text-gray-600">
          Gilt ab sofort für alle kommenden Wochen. Bereits eingetragene
          Ausnahmen bleiben bestehen.
        </p>
        {careDaysSource === "bookings" ? (
          <p className="text-sm font-medium text-gray-700">
            Die Betreuungstage kommen aus den Buchungen. Ändern Sie die Tage bei
            den Buchungen.
          </p>
        ) : null}
        <CareWeeklyPlanGrid
          draft={draft}
          careDaysSource={careDaysSource}
          removals={weeklyRemovals}
          disabled={isSubmitting}
          pickupNeedsCareDay
          schoolPeriods={schoolPeriods}
          timePresets={timePresets}
          notesWithoutPickup
        />
        <EditActions onCancel={onCancel} saving={isSubmitting} />
      </form>

      <ConfirmationModal
        isOpen={showRemovalConfirm}
        onClose={() => setShowRemovalConfirm(false)}
        onConfirm={() => void performSave()}
        title="Zeiten entfernen?"
        confirmText="Trotzdem speichern"
        cancelText="Zurück"
        isConfirmLoading={isSubmitting}
        confirmVariant="danger"
      >
        <div className="space-y-2 text-sm leading-6 text-gray-600">
          <p>
            Diese Zeiten sind im Wochenplan hinterlegt und werden durch das
            Speichern entfernt:
          </p>
          <ul className="list-inside list-disc font-semibold text-gray-800">
            {weeklyRemovals.map((removal) => (
              <li key={removal}>{removal}</li>
            ))}
          </ul>
        </div>
      </ConfirmationModal>

      {/* The offering decision has its own flow (choose an offering, check the
          consequences, confirm) and two conclusions, so it stays a dialog
          opened out of the save rather than a second edit state. */}
      <FormModal
        isOpen={weeklyAdjustment !== null}
        onClose={clearDecision}
        title="Wochenplan speichern"
        size="xl"
        error={error}
        closeDisabled={isSubmitting}
        footer={
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={clearDecision}
            disabled={isSubmitting}
          >
            Zurück zum Wochenplan
          </Button>
        }
      >
        {weeklyAdjustment ? (
          <PickupAdjustmentDecision
            preview={weeklyAdjustment}
            headingRef={decisionHeadingRef}
            reason={offeringReason}
            selectedOfferingId={selectedOfferingId}
            effectiveFrom={offeringEffectiveFrom}
            canSaveException={
              offeringEffectiveFrom <= formatDateISO(new Date())
            }
            confirmed={offeringConfirmed}
            busy={isSubmitting}
            onReasonChange={setOfferingReason}
            onSelectOffering={(offeringId) =>
              void previewMatchingOffering(offeringId)
            }
            onEffectiveFromChange={(value) => {
              setOfferingEffectiveFrom(value);
              setOfferingConfirmed(false);
              if (selectedOfferingId) {
                void previewMatchingOffering(selectedOfferingId, value);
              }
            }}
            onConfirmedChange={setOfferingConfirmed}
            onSaveOffering={() => void saveMatchingOffering()}
            onSaveException={() => void saveWeeklyException()}
          />
        ) : null}
      </FormModal>
    </>
  );
}
