"use client";

import { useEffect, useId, useMemo, useState } from "react";
import {
  CalendarDays,
  CalendarRange,
  ChevronDown,
  Clock,
  Loader2,
  Repeat,
  StickyNote,
  Trash2,
  Users,
} from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { useToast } from "~/contexts/ToastContext";
import { toISODate } from "~/lib/date-helpers";
import {
  type ArrivalDayData,
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";
import type { ArrivalNote } from "~/lib/student-arrival-api";
import type {
  DayData as PickupDayData,
  PickupNote,
  PickupScheduleFormData,
} from "~/lib/pickup-schedule-helpers";
import { formatPickupTime } from "~/lib/pickup-schedule-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";

/**
 * How far a care-plan edit reaches. The three values are the three things a
 * user actually wants to do (issue #893): change today only, change a stretch
 * of days, or change the recurring plan from now on. Before this editor the
 * middle one did not exist and the other two lived behind two identical-looking
 * pencils.
 */
export type CarePlanScope = "single" | "range" | "weekly";

type ArrivalMode = "regular" | "time" | "absent";
type PickupMode = "regular" | "time" | "none";
type NoteTarget = "arrival" | "pickup";

/** One leg (arrival or pickup) of a date-scoped edit. */
export type CareLegSubmit =
  | { kind: "regular" }
  | { kind: "time"; time: string; reason: string | null }
  | { kind: "none"; reason: string | null };

export interface CarePlanExceptionSubmit {
  /** ISO days the override applies to, ascending. Never empty. */
  readonly dates: string[];
  /** null = this leg was not touched and must stay exactly as it is. */
  readonly arrival: CareLegSubmit | null;
  readonly pickup: CareLegSubmit | null;
  readonly note: {
    readonly target: NoteTarget;
    readonly content: string;
  } | null;
}

export interface CarePlanWeeklySubmit {
  readonly arrivalSchedules: ArrivalScheduleFormEntry[];
  readonly pickupSchedules: PickupScheduleFormData[];
}

interface CarePlanEditorModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** The day the editor was opened from. null = opened from the week header. */
  readonly date: Date | null;
  readonly arrivalDay: ArrivalDayData | null;
  readonly pickupDay: PickupDayData | null;
  readonly weeklyArrival: ArrivalScheduleFormEntry[];
  readonly weeklyPickup: PickupScheduleFormData[];
  /**
   * ISO days that currently carry a GUARDIAN-authored exception. Used to warn
   * before a batch silently reclaims a parent's entry — the modal only sees one
   * day's data, so the caller supplies the full set.
   */
  readonly guardianArrivalDates: readonly string[];
  readonly guardianPickupDates: readonly string[];
  readonly onSubmitExceptions: (
    payload: CarePlanExceptionSubmit,
  ) => Promise<void>;
  readonly onSubmitWeekly: (payload: CarePlanWeeklySubmit) => Promise<void>;
  readonly onDeleteArrivalNote: (noteId: number) => Promise<void>;
  readonly onDeletePickupNote: (noteId: string) => Promise<void>;
}

const TIME_PATTERN = /^([01]?\d|2[0-3]):[0-5]\d$/;

/**
 * Calendar span a range edit may cover, and the resulting cap on school days.
 * The backend rejects more than 60 dates per bulk write (one request
 * transaction locks every day it touches), so the form refuses earlier with a
 * readable message instead of surfacing a 400.
 */
const MAX_RANGE_SPAN_DAYS = 120;
const MAX_RANGE_SCHOOL_DAYS = 60;

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
  guardianArrivalDates,
  guardianPickupDates,
  onSubmitExceptions,
  onSubmitWeekly,
  onDeleteArrivalNote,
  onDeletePickupNote,
}: CarePlanEditorModalProps) {
  const toast = useToast();
  const hasDayContext =
    date !== null && arrivalDay !== null && pickupDay !== null;

  const [scope, setScope] = useState<CarePlanScope>("single");
  const [rangeEnd, setRangeEnd] = useState("");
  const [arrivalMode, setArrivalMode] = useState<ArrivalMode>("regular");
  const [arrivalTime, setArrivalTime] = useState("");
  const [arrivalReason, setArrivalReason] = useState("");
  const [pickupMode, setPickupMode] = useState<PickupMode>("regular");
  const [pickupTime, setPickupTime] = useState("");
  const [pickupReason, setPickupReason] = useState("");
  const [noteTarget, setNoteTarget] = useState<NoteTarget>("pickup");
  const [noteContent, setNoteContent] = useState("");
  const [weeklyRows, setWeeklyRows] = useState<WeeklyRow[]>([]);
  const [expandedWeekdays, setExpandedWeekdays] = useState<Set<number>>(
    () => new Set(),
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [deletingNoteKey, setDeletingNoteKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirm | null>(
    null,
  );
  // The form steps aside while a confirmation is up so the two dialogs never
  // stack. Kept out of `isOpen` on purpose: the reset effect keys on `isOpen`,
  // so toggling this preserves what the user typed.
  const [formVisible, setFormVisible] = useState(true);
  const noteTargetId = useId();
  const rangeEndId = useId();

  useEffect(() => {
    if (!isOpen) return;

    setScope(hasDayContext ? "single" : "weekly");
    setRangeEnd("");
    setError(null);
    setPendingConfirm(null);
    setFormVisible(true);
    setNoteTarget("pickup");
    setNoteContent("");

    const arrivalInit = arrivalInitialState(arrivalDay);
    setArrivalMode(arrivalInit.mode);
    setArrivalTime(arrivalInit.time);
    setArrivalReason(arrivalInit.reason);

    const pickupInit = pickupInitialState(pickupDay);
    setPickupMode(pickupInit.mode);
    setPickupTime(pickupInit.time);
    setPickupReason(pickupInit.reason);

    setWeeklyRows(buildWeeklyRows(weeklyArrival, weeklyPickup));
    setExpandedWeekdays(
      new Set(
        buildWeeklyRows(weeklyArrival, weeklyPickup)
          .filter((row) => row.arrivalNotes || row.pickupNotes)
          .map((row) => row.weekday),
      ),
    );
  }, [
    isOpen,
    hasDayContext,
    arrivalDay,
    pickupDay,
    weeklyArrival,
    weeklyPickup,
  ]);

  const startISO = date ? toISODate(date) : "";
  const batchDates = useMemo(() => {
    if (!startISO) return [];
    if (scope === "single") return [startISO];
    if (scope === "range") return schoolDaysBetween(startISO, rangeEnd);
    return [];
  }, [scope, startISO, rangeEnd]);

  if (!isOpen) return null;

  const arrivalInit = arrivalInitialState(arrivalDay);
  const pickupInit = pickupInitialState(pickupDay);
  // An untouched leg is NOT re-saved: re-saving routes through the staff
  // override path, which reclaims a guardian-authored row and drops the
  // parent's editability. A note-only edit must leave the parent's row alone.
  const arrivalChanged = legChanged(
    { mode: arrivalMode, time: arrivalTime, reason: arrivalReason },
    arrivalInit,
  );
  const pickupChanged = legChanged(
    { mode: pickupMode, time: pickupTime, reason: pickupReason },
    pickupInit,
  );
  // The same rule holds for a range: the form is seeded from ONE day, so an
  // untouched leg says nothing about the other days it covers. Writing it
  // anyway would let "keine Abholung ab Montag" quietly delete arrival
  // overrides (including parent-set ones) across the whole range.
  const writeArrival = arrivalChanged;
  const writePickup = pickupChanged;

  const weeklyRemovals = collectWeeklyRemovals(
    weeklyRows,
    weeklyArrival,
    weeklyPickup,
  );
  const guardianHits = collectGuardianHits(batchDates, {
    arrival: writeArrival ? guardianArrivalDates : [],
    pickup: writePickup ? guardianPickupDates : [],
  });

  const dayNotes = getDayNotes(arrivalDay?.notes ?? [], pickupDay?.notes ?? []);
  const recurringNotes = getRecurringNotes(arrivalDay, pickupDay);

  const title = hasDayContext
    ? `${getWeekdayLabel(arrivalDay.weekday)}, ${formatShortDate(arrivalDay.date)}`
    : "Wochenplan bearbeiten";

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    if (scope === "weekly") {
      const invalidWeekly = validateWeeklyRows(weeklyRows);
      if (invalidWeekly) {
        setError(invalidWeekly);
        return;
      }
      if (weeklyRemovals.length > 0) {
        openConfirm({ kind: "weekly-removal" });
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
    if (scope === "range" && batchDates.length === 0) {
      setError("Bitte ein Enddatum wählen, das nach dem Starttag liegt.");
      return;
    }
    if (!writeArrival && !writePickup && !noteContent.trim()) {
      setError("Bitte zuerst eine Zeit oder eine Notiz ändern.");
      return;
    }
    if (batchDates.length > MAX_RANGE_SCHOOL_DAYS) {
      setError(
        `Ein Zeitraum darf höchstens ${MAX_RANGE_SCHOOL_DAYS} Schultage umfassen. Bitte einen kürzeren Zeitraum wählen.`,
      );
      return;
    }
    if (guardianHits.length > 0) {
      openConfirm({ kind: "guardian-overwrite" });
      return;
    }

    void performSave();
  };

  function openConfirm(next: PendingConfirm): void {
    setFormVisible(false);
    setPendingConfirm(next);
  }

  function cancelConfirm(): void {
    setPendingConfirm(null);
    setFormVisible(true);
  }

  const performSave = async () => {
    setError(null);
    setIsSubmitting(true);
    try {
      if (scope === "weekly") {
        await onSubmitWeekly(toWeeklySubmit(weeklyRows));
        toast.success("Wochenplan wurde gespeichert");
      } else {
        await onSubmitExceptions({
          dates: batchDates,
          arrival: writeArrival
            ? toLegSubmit(arrivalMode, arrivalTime, arrivalReason)
            : null,
          pickup: writePickup
            ? toLegSubmit(pickupMode, pickupTime, pickupReason)
            : null,
          note:
            scope === "single" && noteContent.trim()
              ? { target: noteTarget, content: noteContent.trim() }
              : null,
        });
        toast.success(
          batchDates.length > 1
            ? `Änderung für ${batchDates.length} Tage gespeichert`
            : "Tagesänderung wurde gespeichert",
        );
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
      setPendingConfirm(null);
      setFormVisible(true);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteNote = async (note: CareDayNote) => {
    setDeletingNoteKey(note.key);
    setError(null);
    try {
      if (note.kind === "arrival") {
        await onDeleteArrivalNote(Number(note.id));
      } else {
        await onDeletePickupNote(String(note.id));
      }
      toast.success("Notiz wurde gelöscht");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Notiz konnte nicht gelöscht werden";
      setError(message);
      toast.error(message);
    } finally {
      setDeletingNoteKey(null);
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

  const modal = (
    <FormModal
      isOpen={isOpen && formVisible}
      onClose={onClose}
      title={title}
      footer={footer}
      size={scope === "weekly" ? "xl" : "lg"}
      mobilePosition="bottom"
    >
      {/* noValidate: a half-cleared <input type="time"> (e.g. backspacing the
          hour of "15:00") reports validity.badInput, which makes the browser
          refuse the submit — the Save button then does nothing beyond a native
          bubble on an off-screen field. Such a field reads back as "", which is
          exactly the "remove this time" the removal warning already announces,
          so validation is left to the checks below. */}
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

        {hasDayContext ? (
          <ScopePicker
            scope={scope}
            onChange={setScope}
            dayLabel={formatShortDate(arrivalDay.date)}
            weekdayLabel={getWeekdayLabel(arrivalDay.weekday)}
          />
        ) : null}

        {scope === "range" ? (
          <div className="rounded-xl border border-gray-200 bg-gray-50 p-3 sm:p-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <span className="mb-1 block text-xs font-medium text-gray-500">
                  Von
                </span>
                <div className="flex h-11 items-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-900 shadow-sm sm:h-10">
                  {startISO ? formatShortDate(arrivalDay!.date) : ""}
                </div>
              </div>
              <ISODatePicker
                id={rangeEndId}
                label="Bis"
                value={rangeEnd}
                onChange={setRangeEnd}
                min={startISO}
              />
            </div>
            <p className="mt-2 text-xs leading-5 text-gray-500">
              {batchDates.length > 0
                ? `Gilt für ${batchDates.length} Schultage (Montag bis Freitag).`
                : "Wochenenden werden übersprungen."}
            </p>
          </div>
        ) : null}

        {scope === "weekly" ? (
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
        ) : (
          <>
            {guardianHits.length > 0 ? (
              <div className="flex items-start gap-2.5 rounded-xl border border-[#5080D8]/20 bg-[#5080D8]/10 px-4 py-3 text-sm text-[#3a63b0]">
                <Users className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                <span>
                  {guardianHits.length === 1
                    ? "An diesem Tag haben die Eltern eine Zeit über das Elternportal gesetzt. Deine Eingabe ersetzt die Angabe der Eltern."
                    : `An ${guardianHits.length} der gewählten Tage haben die Eltern eine Zeit über das Elternportal gesetzt. Deine Eingabe ersetzt diese Angaben.`}
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

            <NotesSection
              dayNotes={dayNotes}
              recurringNotes={recurringNotes}
              deletingNoteKey={deletingNoteKey}
              onDeleteNote={(note) => void handleDeleteNote(note)}
              onSwitchToWeekly={() => setScope("weekly")}
              composer={
                scope === "single"
                  ? {
                      noteTargetId,
                      noteTarget,
                      onTargetChange: setNoteTarget,
                      noteContent,
                      onContentChange: setNoteContent,
                    }
                  : null
              }
            />
          </>
        )}
      </form>
    </FormModal>
  );

  return (
    <>
      {modal}
      <ConfirmationModal
        isOpen={pendingConfirm?.kind === "guardian-overwrite"}
        onClose={cancelConfirm}
        onConfirm={() => void performSave()}
        title="Eltern-Angabe überschreiben?"
        confirmText="Trotzdem überschreiben"
        cancelText="Abbrechen"
        isConfirmLoading={isSubmitting}
        confirmButtonClass="bg-[#CC2626] hover:bg-[#B91C1C]"
      >
        <p className="text-sm leading-6 text-gray-600">
          {guardianHits.length === 1
            ? "Du überschreibst eine von den Eltern gesetzte Zeit. Die ursprüngliche Angabe wird ersetzt und der Tag gilt anschließend als von der Einrichtung geändert."
            : `Du überschreibst von den Eltern gesetzte Zeiten an ${guardianHits.length} Tagen. Die ursprünglichen Angaben werden ersetzt.`}
        </p>
      </ConfirmationModal>
      <ConfirmationModal
        isOpen={pendingConfirm?.kind === "weekly-removal"}
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

type PendingConfirm =
  { readonly kind: "guardian-overwrite" } | { readonly kind: "weekly-removal" };

function ScopePicker({
  scope,
  onChange,
  dayLabel,
  weekdayLabel,
}: {
  readonly scope: CarePlanScope;
  readonly onChange: (scope: CarePlanScope) => void;
  readonly dayLabel: string;
  readonly weekdayLabel: string;
}) {
  const options: ReadonlyArray<{
    value: CarePlanScope;
    title: string;
    hint: string;
    icon: React.ReactNode;
  }> = [
    {
      value: "single",
      title: "Nur an diesem Tag",
      hint: dayLabel,
      icon: <CalendarDays className="h-4 w-4" aria-hidden="true" />,
    },
    {
      value: "range",
      title: "An mehreren Tagen",
      hint: "Zeitraum wählen",
      icon: <CalendarRange className="h-4 w-4" aria-hidden="true" />,
    },
    {
      value: "weekly",
      title: `Jeden ${weekdayLabel}`,
      hint: "Ändert den Wochenplan",
      icon: <Repeat className="h-4 w-4" aria-hidden="true" />,
    },
  ];

  return (
    <fieldset>
      <legend className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
        Gilt für
      </legend>
      <div className="grid gap-2 sm:grid-cols-3">
        {options.map((option) => {
          const isActive = scope === option.value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={isActive}
              onClick={() => onChange(option.value)}
              className={`flex min-w-0 items-start gap-2.5 rounded-lg border p-3 text-left transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                isActive
                  ? "border-gray-900 bg-gray-900 text-white"
                  : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
              }`}
            >
              <span className="mt-0.5 shrink-0">{option.icon}</span>
              <span className="min-w-0">
                <span className="block truncate text-sm font-semibold">
                  {option.title}
                </span>
                <span
                  className={`block truncate text-xs ${
                    isActive ? "text-gray-300" : "text-gray-500"
                  }`}
                >
                  {option.hint}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </fieldset>
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
        <label className="mt-3 block">
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Uhrzeit
          </span>
          <input
            type="time"
            value={time}
            onChange={(event) => onTimeChange(event.target.value)}
            className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
          />
        </label>
      ) : null}

      {showReason ? (
        <label className="mt-3 block">
          <span className="mb-1 block text-xs font-medium text-gray-500">
            Grund
          </span>
          <input
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

function NotesSection({
  dayNotes,
  recurringNotes,
  deletingNoteKey,
  onDeleteNote,
  onSwitchToWeekly,
  composer,
}: {
  readonly dayNotes: CareDayNote[];
  readonly recurringNotes: RecurringNote[];
  readonly deletingNoteKey: string | null;
  readonly onDeleteNote: (note: CareDayNote) => void;
  readonly onSwitchToWeekly: () => void;
  readonly composer: {
    readonly noteTargetId: string;
    readonly noteTarget: NoteTarget;
    readonly onTargetChange: (target: NoteTarget) => void;
    readonly noteContent: string;
    readonly onContentChange: (value: string) => void;
  } | null;
}) {
  return (
    <section className="rounded-xl border border-gray-200 bg-white p-3 shadow-sm sm:rounded-2xl sm:p-4">
      <div className="mb-3 flex items-center gap-2">
        <StickyNote className="h-4 w-4 text-gray-400" aria-hidden="true" />
        <h3 className="text-sm font-semibold text-gray-900">Notizen</h3>
      </div>

      {dayNotes.length > 0 || recurringNotes.length > 0 ? (
        <div className="mb-4 space-y-2">
          {dayNotes.map((note) => (
            <div
              key={note.key}
              className="flex items-start gap-2 rounded-xl bg-gray-50 px-3 py-2 text-sm text-gray-700"
            >
              <span className="mt-0.5 shrink-0 rounded-full bg-white px-2 py-0.5 text-[11px] font-semibold text-gray-500 shadow-sm">
                {note.kind === "arrival" ? "Ankunft" : "Abholung"}
              </span>
              <span className="min-w-0 flex-1 break-words">{note.content}</span>
              <button
                type="button"
                onClick={() => onDeleteNote(note)}
                disabled={deletingNoteKey === note.key}
                className="rounded-md p-1.5 text-gray-400 hover:bg-white hover:text-[#CC2626] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                aria-label="Notiz löschen"
              >
                {deletingNoteKey === note.key ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Trash2 className="h-3.5 w-3.5" />
                )}
              </button>
            </div>
          ))}

          {/* Recurring notes live on the weekly schedule, not on this date. They
              used to appear on the day card but nowhere in the day editor, so a
              user who wanted to remove one had no way to get there (#893). */}
          {recurringNotes.map((note) => (
            <div
              key={note.key}
              className="flex items-start gap-2 rounded-xl border border-dashed border-gray-200 px-3 py-2 text-sm text-gray-600"
            >
              <span className="mt-0.5 shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-semibold text-gray-500">
                {note.kind === "arrival" ? "Ankunft" : "Abholung"} · Jede Woche
              </span>
              <span className="min-w-0 flex-1 break-words">{note.content}</span>
              <button
                type="button"
                onClick={onSwitchToWeekly}
                className="shrink-0 rounded-md px-2 py-1 text-xs font-semibold text-gray-500 hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                Im Wochenplan ändern
              </button>
            </div>
          ))}
        </div>
      ) : null}

      {composer ? (
        <div className="grid gap-3 sm:grid-cols-[160px_1fr]">
          <label htmlFor={composer.noteTargetId} className="block">
            <span className="mb-1 block text-xs font-medium text-gray-500">
              Bereich
            </span>
            <CustomSelect
              id={composer.noteTargetId}
              value={composer.noteTarget}
              options={[
                { value: "arrival", label: "Ankunft" },
                { value: "pickup", label: "Abholung" },
              ]}
              onChange={(next) => composer.onTargetChange(next as NoteTarget)}
              ariaLabel="Bereich"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-gray-500">
              Neue Notiz
            </span>
            <input
              type="text"
              value={composer.noteContent}
              onChange={(event) => composer.onContentChange(event.target.value)}
              maxLength={500}
              className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
              placeholder="Optional"
            />
          </label>
        </div>
      ) : (
        <p className="text-xs leading-5 text-gray-500">
          Notizen lassen sich für einen einzelnen Tag hinterlegen. Für mehrere
          Tage trage den Anlass als Grund bei Ankunft oder Abholung ein.
        </p>
      )}
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
        Gilt ab sofort für alle kommenden Wochen. Bereits eingetragene
        Tagesänderungen bleiben bestehen.
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

/**
 * Every Monday-to-Friday day from `startISO` to `endISO` inclusive. Weekends
 * carry no care plan, so including them would write exceptions nobody reads.
 * Returns [] when the range is empty or inverted.
 */
export function schoolDaysBetween(startISO: string, endISO: string): string[] {
  if (!startISO || !endISO) return [];
  const start = new Date(`${startISO}T00:00:00`);
  const end = new Date(`${endISO}T00:00:00`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return [];
  if (end < start) return [];

  const days: string[] = [];
  // Stepping a fresh Date by an offset (rather than mutating one cursor) keeps
  // the arithmetic DST-exact and the loop condition obviously terminating.
  for (let offset = 0; offset <= MAX_RANGE_SPAN_DAYS; offset += 1) {
    const cursor = new Date(start);
    cursor.setDate(cursor.getDate() + offset);
    if (cursor > end) break;
    const weekday = cursor.getDay();
    if (weekday >= 1 && weekday <= 5) days.push(toISODate(cursor));
  }
  return days;
}

function collectGuardianHits(
  dates: readonly string[],
  guardian: {
    readonly arrival: readonly string[];
    readonly pickup: readonly string[];
  },
): string[] {
  const touched = new Set([...guardian.arrival, ...guardian.pickup]);
  return dates.filter((date) => touched.has(date));
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
 * Which previously scheduled times an emptied weekly field would delete. The
 * weekly bulk write REPLACES the plan, so clearing a field really does remove
 * that day — this is what the removal warning and confirmation are built from,
 * so a mistyped Friday cannot vanish silently.
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

interface CareDayNote {
  readonly key: string;
  readonly id: number | string;
  readonly kind: NoteTarget;
  readonly content: string;
}

interface RecurringNote {
  readonly key: string;
  readonly kind: NoteTarget;
  readonly content: string;
}

function getDayNotes(
  arrivalNotes: readonly ArrivalNote[],
  pickupNotes: readonly PickupNote[],
): CareDayNote[] {
  return [
    ...arrivalNotes.map((note) => ({
      key: `arrival-${note.id}`,
      id: note.id,
      kind: "arrival" as const,
      content: note.content,
    })),
    ...pickupNotes.map((note) => ({
      key: `pickup-${note.id}`,
      id: note.id,
      kind: "pickup" as const,
      content: note.content,
    })),
  ];
}

function getRecurringNotes(
  arrivalDay: ArrivalDayData | null,
  pickupDay: PickupDayData | null,
): RecurringNote[] {
  const notes: RecurringNote[] = [];
  if (arrivalDay?.baseSchedule?.notes) {
    notes.push({
      key: "recurring-arrival",
      kind: "arrival",
      content: arrivalDay.baseSchedule.notes,
    });
  }
  if (pickupDay?.baseSchedule?.notes) {
    notes.push({
      key: "recurring-pickup",
      kind: "pickup",
      content: pickupDay.baseSchedule.notes,
    });
  }
  return notes;
}
