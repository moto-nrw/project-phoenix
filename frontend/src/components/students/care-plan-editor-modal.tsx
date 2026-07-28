"use client";

import { useEffect, useState } from "react";
import {
  CalendarDays,
  ChevronDown,
  Clock,
  Loader2,
  StickyNote,
  Users,
} from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalDayData,
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";
import type {
  DayData as PickupDayData,
  PickupScheduleFormData,
} from "~/lib/pickup-schedule-helpers";
import { formatPickupTime } from "~/lib/pickup-schedule-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";

/**
 * The care plan is edited through exactly two doors, and each one owns one kind
 * of thing (issue #893):
 *
 *   day card         -> an EXCEPTION for that date, with a Grund
 *   "Wochenplan"     -> the recurring times of the week, with Notizen
 *
 * That split is the whole fix: before, two identical-looking pencils both led
 * to dialogs that mixed exceptions and notes, and a note gave no hint whether
 * it applied once or every week.
 */
type ArrivalMode = "regular" | "time" | "absent";
type PickupMode = "regular" | "time" | "none";

/** One leg (arrival or pickup) of a day exception. */
export type CareLegSubmit =
  | { kind: "regular" }
  | { kind: "time"; time: string; reason: string | null }
  | { kind: "none"; reason: string | null };

export interface CareExceptionSubmit {
  /** The single ISO day this exception applies to. */
  readonly date: string;
  /** null = this leg was not touched and must stay exactly as it is. */
  readonly arrival: CareLegSubmit | null;
  readonly pickup: CareLegSubmit | null;
}

export interface CarePlanWeeklySubmit {
  readonly arrivalSchedules: ArrivalScheduleFormEntry[];
  readonly pickupSchedules: PickupScheduleFormData[];
}

interface CarePlanEditorModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** The day the editor was opened from. null = the weekly plan. */
  readonly date: Date | null;
  readonly arrivalDay: ArrivalDayData | null;
  readonly pickupDay: PickupDayData | null;
  readonly weeklyArrival: ArrivalScheduleFormEntry[];
  readonly weeklyPickup: PickupScheduleFormData[];
  readonly onSubmitException: (payload: CareExceptionSubmit) => Promise<void>;
  readonly onSubmitWeekly: (payload: CarePlanWeeklySubmit) => Promise<void>;
}

const TIME_PATTERN = /^([01]?\d|2[0-3]):[0-5]\d$/;

interface WeeklyRow {
  readonly weekday: number;
  readonly arrivalTime: string;
  readonly arrivalNotes: string;
  readonly pickupTime: string;
  readonly pickupNotes: string;
}

export function CarePlanEditorModal({
  isOpen,
  onClose,
  date,
  arrivalDay,
  pickupDay,
  weeklyArrival,
  weeklyPickup,
  onSubmitException,
  onSubmitWeekly,
}: CarePlanEditorModalProps) {
  const toast = useToast();
  const isException =
    date !== null && arrivalDay !== null && pickupDay !== null;

  const [arrivalMode, setArrivalMode] = useState<ArrivalMode>("regular");
  const [arrivalTime, setArrivalTime] = useState("");
  const [arrivalReason, setArrivalReason] = useState("");
  const [pickupMode, setPickupMode] = useState<PickupMode>("regular");
  const [pickupTime, setPickupTime] = useState("");
  const [pickupReason, setPickupReason] = useState("");
  const [weeklyRows, setWeeklyRows] = useState<WeeklyRow[]>([]);
  const [expandedWeekdays, setExpandedWeekdays] = useState<Set<number>>(
    () => new Set(),
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showRemovalConfirm, setShowRemovalConfirm] = useState(false);
  const [showParentConfirm, setShowParentConfirm] = useState(false);
  // The form steps aside while the confirmation is up so the two dialogs never
  // stack. Kept out of `isOpen` on purpose: the reset effect keys on `isOpen`,
  // so toggling this preserves what the user typed.
  const [formVisible, setFormVisible] = useState(true);

  useEffect(() => {
    if (!isOpen) return;

    setError(null);
    setShowRemovalConfirm(false);
    setFormVisible(true);

    const arrivalInit = arrivalInitialState(arrivalDay);
    setArrivalMode(arrivalInit.mode);
    setArrivalTime(arrivalInit.time);
    setArrivalReason(arrivalInit.reason);

    const pickupInit = pickupInitialState(pickupDay);
    setPickupMode(pickupInit.mode);
    setPickupTime(pickupInit.time);
    setPickupReason(pickupInit.reason);

    const rows = buildWeeklyRows(weeklyArrival, weeklyPickup);
    setWeeklyRows(rows);
    setExpandedWeekdays(
      new Set(
        rows
          .filter((row) => row.arrivalNotes || row.pickupNotes)
          .map((row) => row.weekday),
      ),
    );
  }, [isOpen, arrivalDay, pickupDay, weeklyArrival, weeklyPickup]);

  if (!isOpen) return null;

  const arrivalInit = arrivalInitialState(arrivalDay);
  const pickupInit = pickupInitialState(pickupDay);
  // An untouched leg is NOT re-saved: re-saving routes through the staff
  // override path, which reclaims a guardian-authored row and drops the
  // parent's editability.
  const arrivalChanged = legChanged(
    { mode: arrivalMode, time: arrivalTime, reason: arrivalReason },
    arrivalInit,
  );
  const pickupChanged = legChanged(
    { mode: pickupMode, time: pickupTime, reason: pickupReason },
    pickupInit,
  );

  const weeklyRemovals = collectWeeklyRemovals(
    weeklyRows,
    weeklyArrival,
    weeklyPickup,
  );

  const parentAuthored =
    arrivalDay?.exception?.source === "guardian" ||
    pickupDay?.exception?.source === "guardian";
  const overwritesParent =
    (arrivalDay?.exception?.source === "guardian" && arrivalChanged) ||
    (pickupDay?.exception?.source === "guardian" && pickupChanged);

  const title = isException
    ? `Ausnahme für ${getWeekdayLabel(arrivalDay.weekday)}, ${formatShortDate(arrivalDay.date)}`
    : "Wochenplan bearbeiten";

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    if (!isException) {
      const invalidWeekly = validateWeeklyRows(weeklyRows);
      if (invalidWeekly) {
        setError(invalidWeekly);
        return;
      }
      if (weeklyRemovals.length > 0) {
        setFormVisible(false);
        setShowRemovalConfirm(true);
        return;
      }
      void performSave();
      return;
    }

    if (arrivalMode === "time" && !TIME_PATTERN.test(arrivalTime)) {
      setError("Bitte eine gültige Ankunftszeit eingeben.");
      return;
    }
    if (pickupMode === "time" && !TIME_PATTERN.test(pickupTime)) {
      setError("Bitte eine gültige Abholzeit eingeben.");
      return;
    }
    if (!arrivalChanged && !pickupChanged) {
      setError("Bitte zuerst eine Zeit ändern.");
      return;
    }
    if (overwritesParent) {
      setFormVisible(false);
      setShowRemovalConfirm(false);
      setShowParentConfirm(true);
      return;
    }

    void performSave();
  };

  const cancelConfirm = () => {
    setShowRemovalConfirm(false);
    setShowParentConfirm(false);
    setFormVisible(true);
  };

  const performSave = async () => {
    setError(null);
    setIsSubmitting(true);
    try {
      if (!isException) {
        await onSubmitWeekly(toWeeklySubmit(weeklyRows));
        toast.success("Wochenplan wurde gespeichert");
      } else {
        await onSubmitException({
          date: toDayISO(arrivalDay.date),
          arrival: arrivalChanged
            ? toLegSubmit(arrivalMode, arrivalTime, arrivalReason)
            : null,
          pickup: pickupChanged
            ? toLegSubmit(pickupMode, pickupTime, pickupReason)
            : null,
        });
        toast.success("Ausnahme wurde gespeichert");
      }
      onClose();
    } catch (err) {
      const raw =
        err instanceof Error
          ? err.message
          : "Änderung konnte nicht gespeichert werden";
      // The backend refuses to let an account without a staff profile overwrite
      // a parent-set time. Surface that as a readable reason, not a raw 403.
      const message = raw.includes("staff_profile_required")
        ? "Diese Zeit wurde von den Eltern gesetzt und kann nur von Mitarbeitenden mit Personalprofil geändert werden."
        : raw;
      setError(message);
      toast.error(message);
      cancelConfirm();
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isSubmitting}
      >
        Abbrechen
      </Button>
      <Button
        type="submit"
        size="md"
        form="care-plan-editor-form"
        className="gap-2"
        disabled={isSubmitting}
      >
        {isSubmitting ? (
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        ) : null}
        Speichern
      </Button>
    </>
  );

  return (
    <>
      <FormModal
        isOpen={isOpen && formVisible}
        onClose={onClose}
        title={title}
        footer={footer}
        size={isException ? "lg" : "xl"}
        mobilePosition="bottom"
      >
        {/* noValidate: a half-cleared <input type="time"> (e.g. backspacing the
            hour of "15:00") reports validity.badInput, which makes the browser
            refuse the submit — the Save button then does nothing beyond a native
            bubble on an off-screen field. Such a field reads back as "", which
            is the removal the warning below already announces. */}
        <form
          id="care-plan-editor-form"
          noValidate
          onSubmit={handleSubmit}
          className="space-y-5"
        >
          {error ? (
            <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 px-4 py-3 text-sm text-[#CC2626]">
              {error}
            </div>
          ) : null}

          {isException ? (
            <>
              <p className="text-sm leading-6 text-gray-600">
                Gilt nur an diesem Tag. Die festen Zeiten der Woche bleiben
                unverändert.
              </p>

              {parentAuthored ? (
                <div className="flex items-start gap-2.5 rounded-xl border border-[#5080D8]/20 bg-[#5080D8]/10 px-4 py-3 text-sm text-[#3a63b0]">
                  <Users
                    className="mt-0.5 h-4 w-4 shrink-0"
                    aria-hidden="true"
                  />
                  <span>
                    Diese Zeiten wurden von den Eltern über das Elternportal
                    gesetzt. Wenn du sie änderst, ersetzt deine Eingabe die
                    Angabe der Eltern.
                  </span>
                </div>
              ) : null}

              <div className="grid gap-3 sm:grid-cols-2">
                <LegSection
                  label="Ankunft"
                  icon={<Clock className="h-4 w-4" aria-hidden="true" />}
                  color={LOCATION_COLORS.GROUP_ROOM}
                  regularLabel={`Regulär: ${formatRegularArrival(arrivalDay)}`}
                  mode={arrivalMode}
                  onModeChange={(mode) => setArrivalMode(mode as ArrivalMode)}
                  options={[
                    ["regular", "Regulär"],
                    ["time", "Andere Zeit"],
                    ["absent", "Kommt nicht"],
                  ]}
                  time={arrivalTime}
                  onTimeChange={setArrivalTime}
                  reason={arrivalReason}
                  onReasonChange={setArrivalReason}
                  showTime={arrivalMode === "time"}
                  showReason={arrivalMode !== "regular"}
                />
                <LegSection
                  label="Abholung"
                  icon={<CalendarDays className="h-4 w-4" aria-hidden="true" />}
                  color={LOCATION_COLORS.SCHOOLYARD}
                  regularLabel={`Regulär: ${formatRegularPickup(pickupDay)}`}
                  mode={pickupMode}
                  onModeChange={(mode) => setPickupMode(mode as PickupMode)}
                  options={[
                    ["regular", "Regulär"],
                    ["time", "Andere Zeit"],
                    ["none", "Keine Abholung"],
                  ]}
                  time={pickupTime}
                  onTimeChange={setPickupTime}
                  reason={pickupReason}
                  onReasonChange={setPickupReason}
                  showTime={pickupMode === "time"}
                  showReason={pickupMode !== "regular"}
                />
              </div>
            </>
          ) : (
            <WeeklySection
              rows={weeklyRows}
              expandedWeekdays={expandedWeekdays}
              removals={weeklyRemovals}
              onToggleNotes={(weekday) =>
                setExpandedWeekdays((current) => {
                  const next = new Set(current);
                  if (next.has(weekday)) {
                    next.delete(weekday);
                  } else {
                    next.add(weekday);
                  }
                  return next;
                })
              }
              onChange={(weekday, field, value) =>
                setWeeklyRows((rows) =>
                  rows.map((row) =>
                    row.weekday === weekday ? { ...row, [field]: value } : row,
                  ),
                )
              }
            />
          )}
        </form>
      </FormModal>

      <ConfirmationModal
        isOpen={showParentConfirm}
        onClose={cancelConfirm}
        onConfirm={() => void performSave()}
        title="Eltern-Angabe überschreiben?"
        confirmText="Trotzdem überschreiben"
        cancelText="Abbrechen"
        isConfirmLoading={isSubmitting}
        confirmButtonClass="bg-[#CC2626] hover:bg-[#B91C1C]"
      >
        <p className="text-sm leading-6 text-gray-600">
          Du überschreibst eine von den Eltern gesetzte Zeit. Die ursprüngliche
          Angabe wird ersetzt und der Tag gilt anschließend als von der
          Einrichtung geändert.
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={showRemovalConfirm}
        onClose={cancelConfirm}
        onConfirm={() => void performSave()}
        title="Zeiten entfernen?"
        confirmText="Trotzdem speichern"
        cancelText="Zurück"
        isConfirmLoading={isSubmitting}
        confirmButtonClass="bg-[#CC2626] hover:bg-[#B91C1C]"
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
    </>
  );
}

function LegSection({
  label,
  icon,
  color,
  regularLabel,
  mode,
  onModeChange,
  options,
  time,
  onTimeChange,
  reason,
  onReasonChange,
  showTime,
  showReason,
}: {
  readonly label: string;
  readonly icon: React.ReactNode;
  readonly color: string;
  readonly regularLabel: string;
  readonly mode: string;
  readonly onModeChange: (mode: string) => void;
  readonly options: ReadonlyArray<readonly [string, string]>;
  readonly time: string;
  readonly onTimeChange: (value: string) => void;
  readonly reason: string;
  readonly onReasonChange: (value: string) => void;
  readonly showTime: boolean;
  readonly showReason: boolean;
}) {
  const timeId = `exception-${label.toLowerCase()}-time`;
  const reasonId = `exception-${label.toLowerCase()}-reason`;
  return (
    <section className="rounded-xl border border-gray-200 bg-white p-3 shadow-sm sm:rounded-2xl sm:p-4">
      <div className="mb-3 flex items-start gap-3">
        <span
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg"
          style={{ backgroundColor: `${color}14`, color }}
        >
          {icon}
        </span>
        <div>
          <h3 className="text-sm font-semibold text-gray-900">{label}</h3>
          <p className="text-xs text-gray-500">{regularLabel}</p>
        </div>
      </div>

      <div className="grid gap-2">
        {options.map(([value, text]) => (
          <Button
            key={value}
            type="button"
            size="md"
            variant={mode === value ? "primary" : "outline"}
            className="w-full justify-start"
            onClick={() => onModeChange(value)}
          >
            {text}
          </Button>
        ))}
      </div>

      {showTime ? (
        <label className="mt-3 block" htmlFor={timeId}>
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Uhrzeit
          </span>
          <input
            id={timeId}
            type="time"
            value={time}
            onChange={(event) => onTimeChange(event.target.value)}
            className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
          />
        </label>
      ) : null}

      {/* "Grund", not "Notiz": on a day the free text explains the deviation,
          and calling it a note was half of what made #893 confusing. Notes live
          in the weekly plan and mean "every week". */}
      {showReason ? (
        <label className="mt-3 block" htmlFor={reasonId}>
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Grund
          </span>
          <input
            id={reasonId}
            type="text"
            value={reason}
            onChange={(event) => onReasonChange(event.target.value)}
            maxLength={255}
            className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
            placeholder="Optional"
          />
        </label>
      ) : null}
    </section>
  );
}

function WeeklySection({
  rows,
  expandedWeekdays,
  removals,
  onToggleNotes,
  onChange,
}: {
  readonly rows: WeeklyRow[];
  readonly expandedWeekdays: Set<number>;
  readonly removals: string[];
  readonly onToggleNotes: (weekday: number) => void;
  readonly onChange: (
    weekday: number,
    field: "arrivalTime" | "pickupTime" | "arrivalNotes" | "pickupNotes",
    value: string,
  ) => void;
}) {
  return (
    <div className="space-y-3">
      <p className="text-sm leading-6 text-gray-600">
        Gilt ab sofort für alle kommenden Wochen. Bereits eingetragene Ausnahmen
        bleiben bestehen.
      </p>

      <div className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm sm:rounded-2xl">
        <div className="hidden grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)] gap-3 border-b border-gray-100 bg-gray-50 px-4 py-3 text-xs font-semibold tracking-wide text-gray-500 uppercase sm:grid">
          <span>Tag</span>
          <span>Ankunft</span>
          <span>Abholung</span>
        </div>
        <div className="divide-y divide-gray-100">
          {WEEKDAYS.map((day) => {
            const row = rows.find((entry) => entry.weekday === day.value);
            return (
              <div
                key={day.value}
                className="grid gap-3 px-3 py-4 sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)] sm:items-center sm:px-4"
              >
                <div>
                  <div className="text-sm font-semibold text-gray-900">
                    {day.label}
                  </div>
                  <button
                    type="button"
                    onClick={() => onToggleNotes(day.value)}
                    className="mt-1 inline-flex items-center gap-1 text-xs font-medium text-gray-500 transition-colors hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    <StickyNote className="h-3.5 w-3.5" aria-hidden="true" />
                    Notizen
                    <ChevronDown
                      className={`h-3.5 w-3.5 transition-transform ${
                        expandedWeekdays.has(day.value) ? "rotate-180" : ""
                      }`}
                      aria-hidden="true"
                    />
                  </button>
                </div>
                <WeeklyTimeField
                  id={`weekly-arrival-${day.value}`}
                  label="Ankunft"
                  value={row?.arrivalTime ?? ""}
                  onChange={(value) =>
                    onChange(day.value, "arrivalTime", value)
                  }
                />
                <WeeklyTimeField
                  id={`weekly-pickup-${day.value}`}
                  label="Abholung"
                  value={row?.pickupTime ?? ""}
                  onChange={(value) => onChange(day.value, "pickupTime", value)}
                />
                {expandedWeekdays.has(day.value) ? (
                  <div className="grid gap-3 sm:col-span-3 sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)]">
                    <div className="hidden sm:block" />
                    <WeeklyNoteField
                      id={`weekly-arrival-notes-${day.value}`}
                      label="Ankunftsnotiz (jede Woche)"
                      value={row?.arrivalNotes ?? ""}
                      onChange={(value) =>
                        onChange(day.value, "arrivalNotes", value)
                      }
                    />
                    <WeeklyNoteField
                      id={`weekly-pickup-notes-${day.value}`}
                      label="Abholnotiz (jede Woche)"
                      value={row?.pickupNotes ?? ""}
                      onChange={(value) =>
                        onChange(day.value, "pickupNotes", value)
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
        <div className="rounded-xl border border-[#F78C10]/25 bg-[#F78C10]/10 px-4 py-3 text-sm text-[#9A5B08]">
          Wird beim Speichern entfernt: {removals.join(", ")}.
        </div>
      ) : null}
    </div>
  );
}

function WeeklyNoteField({
  id,
  label,
  value,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="mb-1 block text-xs font-medium text-gray-500">
        {label}
      </span>
      <input
        id={id}
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        maxLength={500}
        className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
        placeholder="Optional"
      />
    </label>
  );
}

function WeeklyTimeField({
  id,
  label,
  value,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="mb-1 block text-xs font-medium text-gray-500 sm:hidden">
        {label}
      </span>
      <input
        id={id}
        type="time"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
      />
    </label>
  );
}

// --- pure helpers -----------------------------------------------------------

interface LegState {
  readonly mode: string;
  readonly time: string;
  readonly reason: string;
}

function legChanged(current: LegState, initial: LegState): boolean {
  if (current.mode !== initial.mode) return true;
  if (current.mode === "time" && current.time !== initial.time) return true;
  return (
    current.mode !== "regular" &&
    current.reason.trim() !== initial.reason.trim()
  );
}

function toLegSubmit(
  mode: string,
  time: string,
  reason: string,
): CareLegSubmit {
  const trimmed = reason.trim() ? reason.trim() : null;
  if (mode === "regular") return { kind: "regular" };
  if (mode === "time") return { kind: "time", time, reason: trimmed };
  return { kind: "none", reason: trimmed };
}

/** Local calendar day of a Date, never via toISOString (UTC shifts a day). */
function toDayISO(value: Date): string {
  const month = `${value.getMonth() + 1}`.padStart(2, "0");
  const day = `${value.getDate()}`.padStart(2, "0");
  return `${value.getFullYear()}-${month}-${day}`;
}

function arrivalInitialState(day: ArrivalDayData | null): {
  mode: ArrivalMode;
  time: string;
  reason: string;
} {
  if (day?.exception) {
    return {
      mode: day.exception.expected_arrival ? "time" : "absent",
      time: day.exception.expected_arrival?.slice(0, 5) ?? "",
      reason: day.exception.reason ?? "",
    };
  }
  return { mode: "regular", time: day?.effectiveTime ?? "", reason: "" };
}

function pickupInitialState(day: PickupDayData | null): {
  mode: PickupMode;
  time: string;
  reason: string;
} {
  if (day?.exception) {
    return {
      mode: day.exception.pickupTime ? "time" : "none",
      time: day.exception.pickupTime
        ? formatPickupTime(day.exception.pickupTime)
        : "",
      reason: day.exception.reason ?? "",
    };
  }
  return {
    mode: "regular",
    time: day?.effectiveTime ? formatPickupTime(day.effectiveTime) : "",
    reason: "",
  };
}

function formatRegularArrival(day: ArrivalDayData | null): string {
  return day?.baseSchedule?.expected_arrival
    ? day.baseSchedule.expected_arrival.slice(0, 5)
    : "nicht geplant";
}

function formatRegularPickup(day: PickupDayData | null): string {
  return day?.baseSchedule?.pickupTime
    ? formatPickupTime(day.baseSchedule.pickupTime)
    : "nicht geplant";
}

function buildWeeklyRows(
  arrival: readonly ArrivalScheduleFormEntry[],
  pickup: readonly PickupScheduleFormData[],
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
      arrivalTime: arrivalEntry?.expected_arrival ?? "",
      arrivalNotes: arrivalEntry?.notes ?? "",
      pickupTime: pickupEntry?.pickupTime ?? "",
      pickupNotes: pickupEntry?.notes ?? "",
    };
  });
}

function validateWeeklyRows(rows: readonly WeeklyRow[]): string | null {
  for (const row of rows) {
    const day = WEEKDAYS.find((weekday) => weekday.value === row.weekday);
    if (row.arrivalTime && !TIME_PATTERN.test(row.arrivalTime)) {
      return `Ungültige Ankunftszeit für ${day?.label ?? "diesen Tag"}.`;
    }
    if (row.pickupTime && !TIME_PATTERN.test(row.pickupTime)) {
      return `Ungültige Abholzeit für ${day?.label ?? "diesen Tag"}.`;
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
    const hadArrival = arrival.some(
      (entry) => entry.weekday === row.weekday && entry.expected_arrival,
    );
    const hadPickup = pickup.some(
      (entry) => entry.weekday === row.weekday && entry.pickupTime,
    );
    if (hadArrival && !row.arrivalTime.trim()) {
      removals.push(`${label} Ankunft`);
    }
    if (hadPickup && !row.pickupTime.trim()) {
      removals.push(`${label} Abholung`);
    }
  }
  return removals;
}

function toWeeklySubmit(rows: readonly WeeklyRow[]): CarePlanWeeklySubmit {
  return {
    arrivalSchedules: rows
      .filter((row) => row.arrivalTime.trim() !== "")
      .map((row) => ({
        weekday: row.weekday,
        expected_arrival: row.arrivalTime,
        notes: row.arrivalNotes.trim() ? row.arrivalNotes : null,
      })),
    pickupSchedules: rows
      .filter((row) => row.pickupTime.trim() !== "")
      .map((row) => ({
        weekday: row.weekday,
        pickupTime: row.pickupTime,
        notes: row.pickupNotes.trim() ? row.pickupNotes : undefined,
      })),
  };
}
