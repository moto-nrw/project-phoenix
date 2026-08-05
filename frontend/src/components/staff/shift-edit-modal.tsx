"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { ClosingDayConfirmModal } from "~/components/planning/closing-day-marker";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DatePicker } from "~/components/ui/date-picker";
import { Modal } from "~/components/ui/modal";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  findPeriodForDate,
  shouldMaterializeWeekPattern,
  weekPatternForDate,
  type CalendarPeriod,
} from "~/lib/calendar-period-helpers";
import {
  findFirstClosingDayConflict,
  type ClosingDayConflict,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";
import { useClosingDaysState } from "~/lib/hooks/use-closing-days";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
  ShiftApiError,
  staffShiftService,
  staffShiftSeriesService,
  type SeriesResult,
  type SeriesRule,
} from "~/lib/shift-api";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";
import {
  latestISODate,
  materializedRecurrenceDates,
  weekdayDatesInRange,
} from "~/lib/timetable-helpers";

import {
  formatWeekdays,
  RhythmPicker,
  WeekdayPicker,
} from "./shift-recurrence-fields";

const logger = createLogger({ component: "ShiftEditModal" });
const STAFF_SHIFT_MAX_BREAK_MINUTES = 300;
// Stable empty default so the optional staffOptions prop does not create a new
// array reference each render (oxlint react/no-object-type-as-default-prop).
const EMPTY_STAFF_OPTIONS: readonly StaffScheduleStaff[] = [];
// Stable empty default for the optional existingReplacements prop (same
// rationale as EMPTY_STAFF_OPTIONS).
const EMPTY_REPLACEMENTS: readonly StaffShift[] = [];
const EMPTY_CLOSING_DAY_RANGES: readonly ClosingDayRange[] = [];

function getShiftMutationErrorMessage(
  err: unknown,
  action: "speichern" | "löschen",
): string {
  if (err instanceof ShiftApiError) {
    const detail = err.detail.toLowerCase();
    if (err.status === 409 || detail.includes("overlap")) {
      return "Diese Schicht überschneidet sich mit einer bestehenden Schicht.";
    }
    if (err.status === 400) {
      // A series edit rejects for reasons that have nothing to do with the
      // times, so translate those instead of sending the planner to check
      // Beginn/Ende/Pause (#2028).
      if (detail.includes("no occurrences left")) {
        return 'Diese Serie hat ab morgen keine Termine mehr. Setzen Sie „Gültig bis" auf ein späteres Datum, um sie fortzuführen.';
      }
      if (detail.includes("calendar period")) {
        return "Der gewählte Zeitraum liegt außerhalb des Kalenderzeitraums der Serie.";
      }
      if (detail.includes("week cycle")) {
        return "Woche A/B benötigt einen Kalenderzeitraum mit gepflegtem Wochenzyklus.";
      }
      return "Ungültige Schichtdaten. Bitte prüfen Sie Beginn, Ende und Pause.";
    }
    if (err.status === 401) {
      return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
    }
    if (err.status === 403) {
      return `Sie haben keine Berechtigung, diese Schicht zu ${action}.`;
    }
    return err.detail || `Schicht konnte nicht ${action} werden.`;
  }

  return getApiErrorMessage(
    err,
    action,
    "Schicht",
    action === "speichern"
      ? "Speichern fehlgeschlagen."
      : "Löschen fehlgeschlagen.",
  );
}

// Create/edit modal for one planned shift (Dienstplan). Times are plain
// "HH:MM" strings end-to-end — no Date/ISO conversion, which sidesteps the
// timezone pitfalls of the session edit modal.
export type ShiftEditMode = "create" | "edit";

interface ShiftEditModalProps {
  readonly isOpen: boolean;
  readonly mode: ShiftEditMode;
  readonly staffId: string;
  readonly staffName: string;
  /** Calendar day as "YYYY-MM-DD" */
  readonly date: string;
  readonly shift: StaffShift | null;
  /** All shift types (Schichtarten); the picker offers active ones plus the
   *  one currently attached to this shift (even if inactive). */
  readonly shiftTypes: readonly ShiftType[];
  /** All staff, used to pick a replacement person when a shift is left open
   *  (#1841). The edited shift's own staff member is excluded automatically.
   *  Optional so callers that never left a gap open (and tests) can omit it. */
  readonly staffOptions?: readonly StaffScheduleStaff[];
  /** Replacement shifts already covering this (cancelled) shift — the rows whose
   *  originShiftId is this shift's id (#1841). Passed in so reopening the modal
   *  shows the existing covers instead of an empty set; without them, saving any
   *  edit to an already-cancelled shift would send an empty replacement set and
   *  the backend would delete every cover. Optional / empty for create and for
   *  never-cancelled shifts. */
  readonly existingReplacements?: readonly StaffShift[];
  /** All stored ranges, used to warn for every generated series occurrence. */
  readonly closingDayRanges?: readonly ClosingDayRange[];
  readonly onClose: () => void;
  readonly onSaved: () => void;
}

/** One replacement (Vertretung) covering a cancelled shift. Several rows split
 *  a single gap across people (#1841). breakMinutes/shiftTypeId are carried
 *  per row (not editable in this modal) so re-saving the origin preserves each
 *  cover's own values instead of clobbering them with 0 / the origin's type. */
interface ReplacementRow {
  staffId: string;
  startTime: string;
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string;
}

interface OccurrenceDraft {
  readonly startTime: string;
  readonly endTime: string;
  readonly breakMinutesStr: string;
  readonly shiftTypeId: string;
}

export function ShiftEditModal({
  isOpen,
  mode,
  staffId,
  staffName,
  date,
  shift,
  shiftTypes,
  staffOptions = EMPTY_STAFF_OPTIONS,
  existingReplacements = EMPTY_REPLACEMENTS,
  closingDayRanges = EMPTY_CLOSING_DAY_RANGES,
  onClose,
  onSaved,
}: ShiftEditModalProps) {
  const initial = useMemo(() => {
    if (shift) {
      return {
        startTime: shift.startTime,
        endTime: shift.endTime,
        breakMinutes: shift.breakMinutes,
        shiftTypeId: shift.shiftTypeId ?? "",
      };
    }
    return {
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: "",
    };
  }, [shift]);

  const [startTime, setStartTime] = useState(initial.startTime);
  const [endTime, setEndTime] = useState(initial.endTime);
  // "" = no shift type. An inactive type still attached to this shift is kept
  // as an option so editing a shift doesn't silently drop it.
  const [shiftTypeId, setShiftTypeId] = useState(initial.shiftTypeId);
  // Raw string so the field can be cleared while typing; parsed on submit
  // (same rationale as AdminSessionEditModal).
  const [breakMinutesStr, setBreakMinutesStr] = useState(
    String(initial.breakMinutes),
  );
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Flexible daily changes (#1841): optional reason, "Ausfall" (leave the gap
  // open), and one or more replacements that split the gap across people.
  const [changeReason, setChangeReason] = useState("");
  const [cancelled, setCancelled] = useState(false);
  const [replacements, setReplacements] = useState<ReplacementRow[]>([]);
  // A replacement shift itself is never cancelled or re-covered from here.
  const isReplacementRow = shift?.originShiftId != null;

  // Wiederholung (#1889): create-only series section.
  const [repeatEnabled, setRepeatEnabled] = useState(false);
  const [periods, setPeriods] = useState<CalendarPeriod[] | null>(null);
  const [periodId, setPeriodId] = useState("");
  const [weekdays, setWeekdays] = useState<number[]>([]);
  const [biweekly, setBiweekly] = useState(false);
  const [abPattern, setAbPattern] = useState<1 | 2>(1);
  const [abTouched, setAbTouched] = useState(false);
  const [validUntil, setValidUntil] = useState("");
  // After a series create/split with skipped days the modal shows this
  // notice instead of closing, so the admin sees which days were left out.
  const [seriesNotice, setSeriesNotice] = useState<string | null>(null);
  const [closingDayPrompt, setClosingDayPrompt] = useState<{
    conflict: ClosingDayConflict;
    confirmationKey: string;
    /** Welche Aktion nach dem Bestätigen weiterläuft. */
    action: "submit" | "seriesRule";
  } | null>(null);
  const confirmedClosingConflict = useRef<string | null>(null);
  // Scope question ("Nur diese Woche" / "Ab jetzt dauerhaft" / "Alle Termine
  // der Serie") for edits and deletes of a series-backed row.
  const [scopeQuestion, setScopeQuestion] = useState<"edit" | "delete" | null>(
    null,
  );

  // Serienbearbeitung (#2028): the rule behind this shift, plus the state of
  // the "Alle Termine der Serie" editor. The rule is loaded whenever a series
  // row is opened so its rhythm is visible right away instead of only after
  // the user tries to save.
  const [seriesRule, setSeriesRule] = useState<SeriesRule | null>(null);
  const [seriesRuleError, setSeriesRuleError] = useState(false);
  const [seriesEditOpen, setSeriesEditOpen] = useState(false);
  const [seriesWeekdays, setSeriesWeekdays] = useState<number[]>([]);
  const [seriesBiweekly, setSeriesBiweekly] = useState(false);
  const [seriesAbPattern, setSeriesAbPattern] = useState<1 | 2>(1);
  // The series editor temporarily reuses the occurrence fields. Keep their
  // draft so returning to the occurrence cannot turn a later single save into
  // an unintended series value.
  const [occurrenceDraft, setOccurrenceDraft] =
    useState<OccurrenceDraft | null>(null);
  // Inclusive "Gültig bis" as shown in the picker; the stored valid_until is
  // exclusive (converted on load and on save).
  const [seriesValidUntil, setSeriesValidUntil] = useState("");

  const isSeriesRow = mode === "edit" && shift?.seriesId != null;
  // A moved occurrence keeps its original recurrence slot. Series changes
  // begin at that slot, so every editor hint must describe the same date.
  const seriesEffectiveDate = shift?.seriesOccurrenceDate ?? date;

  useEffect(() => {
    if (!isOpen) return;
    setStartTime(initial.startTime);
    setEndTime(initial.endTime);
    setBreakMinutesStr(String(initial.breakMinutes));
    setShiftTypeId(initial.shiftTypeId);
    setError(null);
    setConfirmDeleteOpen(false);
    setRepeatEnabled(false);
    setWeekdays([isoWeekdayOf(date)]);
    setBiweekly(false);
    setAbPattern(1);
    setAbTouched(false);
    setValidUntil("");
    setSeriesNotice(null);
    setClosingDayPrompt(null);
    confirmedClosingConflict.current = null;
    setScopeQuestion(null);
    setSeriesRule(null);
    setSeriesRuleError(false);
    setSeriesEditOpen(false);
    setOccurrenceDraft(null);
    setChangeReason(shift?.changeReason ?? "");
    setCancelled(shift?.cancelled ?? false);
    // Seed the replacement rows from the covers already attached to this shift
    // so reopening a cancelled shift preserves them; saving would otherwise send
    // an empty set and the backend would delete every existing cover (#1841).
    setReplacements(
      existingReplacements.map((cover) => ({
        staffId: cover.staffId,
        startTime: cover.startTime,
        endTime: cover.endTime,
        breakMinutes: cover.breakMinutes,
        shiftTypeId: cover.shiftTypeId ?? "",
      })),
    );
  }, [isOpen, initial, date, shift, existingReplacements]);

  // Calendar periods load once the series section is opened; the period is
  // required (it bounds the series and anchors Woche A/B). A series row needs
  // them too: its own period decides whether Woche A/B is available at all
  // (#2028).
  useEffect(() => {
    if ((!repeatEnabled && !isSeriesRow) || periods !== null) return;
    let cancelled = false;
    calendarPeriodService
      .list()
      .then((loaded) => {
        if (cancelled) return;
        setPeriods(loaded);
        const match = findPeriodForDate(loaded, date);
        setPeriodId((current) => current || (match?.id ?? ""));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("shift_series_periods_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setPeriods([]);
      });
    return () => {
      cancelled = true;
    };
  }, [repeatEnabled, isSeriesRow, periods, date]);

  // The rule behind a series shift (#2028). Loaded on open so the modal can
  // state the rhythm up front — "Diese Schicht ist Teil einer Serie" alone
  // does not tell the planner what the series actually does.
  const seriesId = shift?.seriesId ?? null;
  useEffect(() => {
    if (!isOpen || mode !== "edit" || seriesId === null) return;
    let cancelled = false;
    staffShiftSeriesService
      .getSeries(seriesId)
      .then((rule) => {
        if (!cancelled) setSeriesRule(rule);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("shift_series_load_failed", {
          series_id: seriesId,
          error: err instanceof Error ? err.message : String(err),
        });
        setSeriesRuleError(true);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, mode, seriesId]);

  const selectedPeriod = useMemo(
    () => periods?.find((p) => p.id === periodId) ?? null,
    [periods, periodId],
  );
  const periodHasCycle = (selectedPeriod?.weekCycleLength ?? 1) > 1;
  const defaultAbPattern = weekPatternForDate(selectedPeriod, date);
  const effectiveWeekPattern = biweekly && periodHasCycle ? abPattern : 0;
  const seriesClosingDayWindow = useMemo(() => {
    if (!repeatEnabled || !selectedPeriod) return null;
    const tomorrow = dayAfterISO(berlinTodayISO());
    const from = latestISODate(date, selectedPeriod.startDate, tomorrow);
    const to =
      validUntil !== "" && validUntil < selectedPeriod.endDate
        ? validUntil
        : selectedPeriod.endDate;
    const dates = weekdayDatesInRange(from, to, weekdays).filter((dateISO) =>
      shouldMaterializeWeekPattern(
        selectedPeriod,
        dateISO,
        effectiveWeekPattern,
      ),
    );
    return { from, to, dates };
  }, [
    date,
    effectiveWeekPattern,
    repeatEnabled,
    selectedPeriod,
    validUntil,
    weekdays,
  ]);
  const seriesClosingDayState = useClosingDaysState(
    seriesClosingDayWindow?.from ?? "",
    seriesClosingDayWindow?.to ?? "",
  );
  const seriesClosingDayConflict = useMemo(() => {
    const dates = seriesClosingDayWindow?.dates ?? [];
    for (const dateISO of dates) {
      const reason = seriesClosingDayState.closingDays.get(dateISO);
      if (reason !== undefined) return { dateISO, reason };
    }
    return findFirstClosingDayConflict(closingDayRanges, dates);
  }, [
    closingDayRanges,
    seriesClosingDayState.closingDays,
    seriesClosingDayWindow,
  ]);
  const seriesClosingDaysLoading =
    seriesClosingDayWindow !== null && seriesClosingDayState.isLoading;
  const closingDayConfirmationKey =
    seriesClosingDayConflict === null
      ? null
      : JSON.stringify({
          action: "create",
          conflict: seriesClosingDayConflict,
          weekdays,
          periodId,
          weekPattern: effectiveWeekPattern,
          validUntil,
        });

  // Default the A/B choice to the parity of the clicked week (mirrors the
  // timetable Wochenrhythmus control from #1882); a manual choice sticks.
  useEffect(() => {
    if (!biweekly || abTouched) return;
    if (defaultAbPattern !== null) setAbPattern(defaultAbPattern);
  }, [biweekly, abTouched, defaultAbPattern]);

  // The series edits against its OWN period, not the create form's selection:
  // that period decides whether Woche A/B exists and anchors the A/B parity.
  const seriesPeriod = useMemo(
    () => periods?.find((p) => p.id === seriesRule?.calendarPeriodId) ?? null,
    [periods, seriesRule],
  );
  const seriesPeriodHasCycle = (seriesPeriod?.weekCycleLength ?? 1) > 1;
  // The backend never re-plans today or the past, so a series edit takes effect
  // tomorrow at the earliest (clamped up to the segment start). Every hint in
  // the editor describes THIS date — showing the opened occurrence promised a
  // change for a day the re-plan does not touch (#2028).
  const seriesAppliesFrom = latestISODate(
    seriesEffectiveDate,
    dayAfterISO(berlinTodayISO()),
    seriesRule?.validFrom ?? seriesEffectiveDate,
  );
  // "Gültig bis" is inclusive in the picker: the segment still has something to
  // change while its last day is on or after the applies-from date. Without the
  // check the save fails server-side with a 400 the planner cannot act on.
  const seriesHasDateRange =
    seriesValidUntil === "" || seriesValidUntil >= seriesAppliesFrom;
  const seriesDefaultAbPattern = weekPatternForDate(
    seriesPeriod,
    seriesAppliesFrom,
  );
  // Keep the stored A/B pattern until the series period is available:
  // otherwise a save while its metadata is still loading would silently turn
  // an alternating series into a weekly one (#2028).
  const seriesRuleWeekPattern =
    seriesPeriod === null
      ? (seriesRule?.weekPattern ?? 0)
      : seriesBiweekly && seriesPeriodHasCycle
        ? seriesAbPattern
        : 0;

  // Schließtage der Serienbearbeitung (#2032): "Alle Termine der Serie" plant
  // ab dieser Stelle neu, kann also spätere Termine auf einen Schließtag
  // legen — dieselbe Rückfrage wie beim Anlegen. Das Fenster beginnt frühestens
  // morgen, weil die Materialisierung vergangene Tage ohnehin nicht anfasst.
  const seriesEditClosingDayWindow = useMemo(() => {
    if (!seriesEditOpen || !seriesPeriod) return null;
    const from = latestISODate(seriesAppliesFrom, seriesPeriod.startDate);
    const to =
      seriesValidUntil !== "" && seriesValidUntil < seriesPeriod.endDate
        ? seriesValidUntil
        : seriesPeriod.endDate;
    const dates = materializedRecurrenceDates({
      period: seriesPeriod,
      fromISO: from,
      weekdays: seriesWeekdays,
      weekPattern: seriesRuleWeekPattern,
      validUntil:
        seriesValidUntil === "" ? undefined : dayAfterISO(seriesValidUntil),
    });
    return { from, to, dates };
  }, [
    seriesEditOpen,
    seriesAppliesFrom,
    seriesPeriod,
    seriesRuleWeekPattern,
    seriesValidUntil,
    seriesWeekdays,
  ]);
  const seriesEditClosingDayState = useClosingDaysState(
    seriesEditClosingDayWindow?.from ?? "",
    seriesEditClosingDayWindow?.to ?? "",
  );
  const seriesEditClosingDayConflict = useMemo(() => {
    const dates = seriesEditClosingDayWindow?.dates ?? [];
    for (const dateISO of dates) {
      const reason = seriesEditClosingDayState.closingDays.get(dateISO);
      if (reason !== undefined) return { dateISO, reason };
    }
    return findFirstClosingDayConflict(closingDayRanges, dates);
  }, [
    closingDayRanges,
    seriesEditClosingDayState.closingDays,
    seriesEditClosingDayWindow,
  ]);
  // Solange die Zeiträume fehlen, ist das Fenster nicht bestimmbar und die
  // Rückfrage ließe sich durch schnelles Speichern umgehen. Bleibt der
  // Zeitraum der Serie dauerhaft unauffindbar, wird nicht blockiert: die
  // Markierung ist ein Hinweis, kein Sperrmechanismus.
  const seriesEditClosingDaysLoading =
    seriesEditOpen && (periods === null || seriesEditClosingDayState.isLoading);
  const seriesHasRemainingOccurrence =
    seriesHasDateRange &&
    (seriesPeriod === null || seriesEditClosingDayWindow?.dates.length !== 0);
  const seriesNoOccurrenceMessage = seriesHasDateRange
    ? `Für die gewählten Wochentage und den Wochenrhythmus bleibt ab ${formatShortDate(seriesAppliesFrom)} kein Termin mehr. Setzen Sie „Gültig bis" auf ein späteres Datum oder ändern Sie die Wiederholung.`
    : `Diese Serie endet am ${formatShortDate(seriesValidUntil)}, Änderungen wirken aber erst ab ${formatShortDate(seriesAppliesFrom)}. Setzen Sie „Gültig bis" auf ein späteres Datum, um die Serie fortzuführen.`;
  const seriesEditConfirmationKey =
    seriesEditClosingDayConflict === null
      ? null
      : JSON.stringify({
          action: "seriesRule",
          conflict: seriesEditClosingDayConflict,
          weekdays: seriesWeekdays,
          weekPattern: seriesRuleWeekPattern,
          validUntil: seriesValidUntil,
        });

  const timesValid = startTime !== "" && endTime !== "" && startTime < endTime;
  const breakMaxMinutes = timesValid
    ? Math.min(
        STAFF_SHIFT_MAX_BREAK_MINUTES,
        shiftDurationMinutes(startTime, endTime) ??
          STAFF_SHIFT_MAX_BREAK_MINUTES,
      )
    : STAFF_SHIFT_MAX_BREAK_MINUTES;
  const breakMinutes = parseBreakMinutes(breakMinutesStr, breakMaxMinutes);
  const breakValid = breakMinutes !== null;
  const seriesValid =
    !repeatEnabled || (weekdays.length > 0 && periodId !== "");

  const typeOptions = useMemo(() => {
    const opts: { value: string; label: string }[] = [
      { value: "", label: "Keine Schichtart" },
    ];
    for (const type of shiftTypes) {
      if (type.isActive || type.id === shift?.shiftTypeId) {
        opts.push({
          value: type.id,
          label: type.isActive ? type.name : `${type.name} (inaktiv)`,
        });
      }
    }
    return opts;
  }, [shiftTypes, shift]);

  const periodOptions = useMemo(
    () =>
      (periods ?? [])
        .filter((p) => p.isActive)
        .map((p) => ({ value: p.id, label: p.name })),
    [periods],
  );

  // Replacement candidates: everyone except the absent staff member.
  const replacementStaffOptions = useMemo(
    () => [
      { value: "", label: "Person auswählen" },
      ...staffOptions
        .filter((member) => member.id !== staffId)
        .map((member) => ({
          value: member.id,
          label: `${member.lastName}, ${member.firstName}`,
        })),
    ],
    [staffOptions, staffId],
  );

  const addReplacement = () => {
    setReplacements((rows) => [
      ...rows,
      { staffId: "", startTime, endTime, breakMinutes: 0, shiftTypeId: "" },
    ]);
  };

  const updateReplacement = (index: number, patch: Partial<ReplacementRow>) => {
    setReplacements((rows) =>
      rows.map((row, i) => {
        if (i !== index) return row;
        const next = { ...row, ...patch };
        // A cover edited directly in the grid may carry a nonzero break, but this
        // row has no break field to correct it. Shortening the window below that
        // hidden break would otherwise submit a break longer than the shift and
        // the backend rejects the whole save. Clamp the preserved break to the
        // edited duration so it can never exceed the window (#1841).
        if (patch.startTime !== undefined || patch.endTime !== undefined) {
          const maxBreak = shiftDurationMinutes(next.startTime, next.endTime);
          if (maxBreak !== null && next.breakMinutes > maxBreak) {
            next.breakMinutes = maxBreak;
          }
        }
        return copyStableObjectKey(row, next);
      }),
    );
  };

  const removeReplacement = (index: number) => {
    setReplacements((rows) => rows.filter((_, i) => i !== index));
  };

  const toggleSeriesWeekday = (value: number) => {
    setSeriesWeekdays((current) =>
      current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value].sort((a, b) => a - b),
    );
  };

  /** Opens the series editor: the same shift modal, but editing the rule
   *  behind the shift (weekdays, rhythm, window, validity) instead of the
   *  single day. */
  const openSeriesEdit = () => {
    if (!seriesRule) {
      setError(
        "Die Serie konnte nicht geladen werden. Bitte schließen Sie das Fenster und versuchen Sie es erneut.",
      );
      return;
    }
    setError(null);
    setOccurrenceDraft({ startTime, endTime, breakMinutesStr, shiftTypeId });
    setSeriesWeekdays([...seriesRule.weekdays].sort((a, b) => a - b));
    setSeriesBiweekly(seriesRule.weekPattern !== 0);
    setSeriesAbPattern(seriesRule.weekPattern === 2 ? 2 : 1);
    // Stored valid_until is exclusive, the picker inclusive.
    setSeriesValidUntil(
      seriesRule.validUntil ? dayBeforeISO(seriesRule.validUntil) : "",
    );
    setStartTime(seriesRule.startTime);
    setEndTime(seriesRule.endTime);
    setBreakMinutesStr(String(seriesRule.breakMinutes));
    setShiftTypeId(seriesRule.shiftTypeId ?? "");
    setSeriesEditOpen(true);
  };

  /** Writes the edited rule through the existing split: it takes effect from
   *  this occurrence onwards, days before it keep the old rule. That is the
   *  one behaviour a series edit has — no second write path (#2028). */
  const saveSeriesRule = async () => {
    if (!seriesRule || !shift) return;
    if (!validateInputs()) return;
    if (seriesWeekdays.length === 0) {
      setError("Bitte mindestens einen Wochentag auswählen.");
      return;
    }
    if (!seriesHasRemainingOccurrence) {
      setError(seriesNoOccurrenceMessage);
      return;
    }
    // Schließtag-Rückfrage vor dem Neuplanen (#2032).
    if (seriesEditClosingDaysLoading) return;
    if (
      seriesEditClosingDayConflict !== null &&
      seriesEditConfirmationKey !== null &&
      confirmedClosingConflict.current !== seriesEditConfirmationKey
    ) {
      setClosingDayPrompt({
        conflict: seriesEditClosingDayConflict,
        confirmationKey: seriesEditConfirmationKey,
        action: "seriesRule",
      });
      return;
    }
    setIsSaving(true);
    try {
      const result = await staffShiftSeriesService.splitSeries(seriesRule.id, {
        effectiveDate: seriesEffectiveDate,
        occurrenceShiftId: shift.id,
        startTime,
        endTime,
        breakMinutes: breakMinutes ?? 0,
        shiftTypeId: shiftTypeId === "" ? null : shiftTypeId,
        weekdays: seriesWeekdays,
        weekPattern: seriesRuleWeekPattern,
        // The picker is inclusive, the API's valid_until exclusive.
        validUntil:
          seriesValidUntil === "" ? null : dayAfterISO(seriesValidUntil),
      });
      finishSeriesMutation(result);
    } catch (err: unknown) {
      logger.error("shift_series_rule_save_failed", {
        series_id: seriesRule.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "speichern"));
    } finally {
      setIsSaving(false);
    }
  };

  const toggleWeekday = (value: number) => {
    setWeekdays((current) =>
      current.includes(value)
        ? current.filter((v) => v !== value)
        : [...current, value].sort((a, b) => a - b),
    );
  };

  const finishSeriesMutation = (result: SeriesResult) => {
    onSaved();
    if (result.skippedDates.length > 0) {
      setSeriesNotice(
        `An folgenden Tagen besteht bereits eine Schicht, sie wurden übersprungen: ${result.skippedDates
          .map(formatShortDate)
          .join(", ")}.`,
      );
      return;
    }
    onClose();
  };

  const validateInputs = (): boolean => {
    setError(null);
    if (!timesValid) {
      setError("Ende muss nach Beginn liegen.");
      return false;
    }
    if (!breakValid || breakMinutes === null) {
      setError(
        `Pause muss eine ganze Zahl zwischen 0 und ${breakMaxMinutes} sein.`,
      );
      return false;
    }
    if (repeatEnabled && weekdays.length === 0) {
      setError("Bitte mindestens einen Wochentag auswählen.");
      return false;
    }
    if (repeatEnabled && periodId === "") {
      setError("Bitte einen Kalenderzeitraum auswählen.");
      return false;
    }
    if (mode === "edit" && cancelled) {
      for (const row of replacements) {
        if (row.staffId === "") {
          setError("Bitte für jede Vertretung eine Person auswählen.");
          return false;
        }
        if (!(
          row.startTime !== "" &&
          row.endTime !== "" &&
          row.startTime < row.endTime
        )) {
          setError("Vertretung: Ende muss nach Beginn liegen.");
          return false;
        }
      }
    }
    return true;
  };

  const createSeries = async () => {
    setIsSaving(true);
    try {
      const result = await staffShiftSeriesService.createSeries({
        staffId,
        weekdays,
        startTime,
        endTime,
        breakMinutes: breakMinutes ?? 0,
        shiftTypeId: shiftTypeId === "" ? null : shiftTypeId,
        calendarPeriodId: periodId,
        // Guard against stale biweekly state: switching to a period without
        // a week cycle hides the A/B control but does not reset the flag.
        weekPattern: biweekly && periodHasCycle ? abPattern : 0,
        validFrom: date,
        // The picker is inclusive ("Gültig bis" = last day WITH a shift);
        // the API's valid_until is exclusive — send the day after.
        validUntil: validUntil === "" ? null : dayAfterISO(validUntil),
      });
      finishSeriesMutation(result);
    } catch (err: unknown) {
      logger.error("shift_series_create_failed", {
        staff_id: staffId,
        date,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "speichern"));
    } finally {
      setIsSaving(false);
    }
  };

  const saveSingleShift = async () => {
    setIsSaving(true);
    try {
      const resolvedShiftTypeId = shiftTypeId === "" ? null : shiftTypeId;
      if (mode === "edit" && shift) {
        // Anything that touches the cancellation state (cancelling, editing the
        // replacement set, or reactivating) goes through one atomic endpoint so
        // the origin flag and the whole replacement set change together (#1841).
        // Reactivating removes the now-obsolete replacements server-side; a plain
        // edit never carries `cancelled`, so the stored flag is preserved.
        const wasCancelled = shift.cancelled ?? false;
        const cancellationInvolved =
          !isReplacementRow && (cancelled || wasCancelled);
        if (cancellationInvolved) {
          await staffShiftService.applyCancellation(shift.id, {
            cancelled,
            changeReason,
            // Carry the shift's own edited window/type so a time change made in
            // the same save is applied to the origin, not silently dropped, and
            // a reactivation re-checks overlap against the edited window (#1841).
            startTime,
            endTime,
            breakMinutes: breakMinutes ?? 0,
            shiftTypeId: resolvedShiftTypeId,
            // Each cover keeps its own break and shift type — a replacement may
            // have been edited directly in the grid to differ from the origin,
            // so re-saving the origin must not reset them (#1841).
            replacements: cancelled
              ? replacements.map((row) => ({
                  staffId: row.staffId,
                  startTime: row.startTime,
                  endTime: row.endTime,
                  breakMinutes: row.breakMinutes,
                  shiftTypeId: row.shiftTypeId === "" ? null : row.shiftTypeId,
                }))
              : [],
          });
        } else {
          await staffShiftService.updateShift(shift.id, {
            staffId,
            date,
            startTime,
            endTime,
            breakMinutes: breakMinutes ?? 0,
            shiftTypeId: resolvedShiftTypeId,
            changeReason,
          });
        }
      } else {
        await staffShiftService.createShift({
          staffId,
          date,
          startTime,
          endTime,
          breakMinutes: breakMinutes ?? 0,
          shiftTypeId: resolvedShiftTypeId,
        });
      }
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("shift_save_failed", {
        staff_id: staffId,
        date,
        mode,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "speichern"));
    } finally {
      setIsSaving(false);
      setScopeQuestion(null);
    }
  };

  const splitSeriesFromHere = async () => {
    if (!shift?.seriesId) return;
    setIsSaving(true);
    try {
      const result = await staffShiftSeriesService.splitSeries(shift.seriesId, {
        effectiveDate: seriesEffectiveDate,
        occurrenceShiftId: shift.id,
        startTime,
        endTime,
        breakMinutes: breakMinutes ?? 0,
        shiftTypeId: shiftTypeId === "" ? null : shiftTypeId,
      });
      finishSeriesMutation(result);
    } catch (err: unknown) {
      logger.error("shift_series_split_failed", {
        series_id: shift.seriesId,
        date: shift.date,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "speichern"));
    } finally {
      setIsSaving(false);
      setScopeQuestion(null);
    }
  };

  const handleSubmit = async () => {
    if (!validateInputs()) return;
    if (mode === "create" && repeatEnabled) {
      if (seriesClosingDaysLoading) return;
      if (
        seriesClosingDayConflict !== null &&
        closingDayConfirmationKey !== null &&
        confirmedClosingConflict.current !== closingDayConfirmationKey
      ) {
        setClosingDayPrompt({
          conflict: seriesClosingDayConflict,
          confirmationKey: closingDayConfirmationKey,
          action: "submit",
        });
        return;
      }
      await createSeries();
      return;
    }
    // A cancellation (or reactivation) is always a single-occurrence change:
    // the backend detaches the series row, so the "Nur diese Woche / Ab jetzt"
    // scope question does not apply. Plain time edits keep it.
    const cancellationChanged = cancelled !== (shift?.cancelled ?? false);
    if (isSeriesRow && !cancelled && !cancellationChanged) {
      setScopeQuestion("edit");
      return;
    }
    await saveSingleShift();
  };

  const deleteSingleShift = async () => {
    if (!shift) return;
    setIsDeleting(true);
    try {
      await staffShiftService.deleteShift(shift.id);
      setConfirmDeleteOpen(false);
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("shift_delete_failed", {
        shift_id: shift.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "löschen"));
      setConfirmDeleteOpen(false);
    } finally {
      setIsDeleting(false);
      setScopeQuestion(null);
    }
  };

  const endSeriesFromHere = async () => {
    if (!shift?.seriesId) return;
    setIsDeleting(true);
    try {
      await staffShiftSeriesService.endSeries(
        shift.seriesId,
        seriesEffectiveDate,
      );
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("shift_series_end_failed", {
        series_id: shift.seriesId,
        date: seriesEffectiveDate,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "löschen"));
    } finally {
      setIsDeleting(false);
      setScopeQuestion(null);
    }
  };

  const handleDeleteClick = () => {
    if (isSeriesRow) {
      setScopeQuestion("delete");
      return;
    }
    setConfirmDeleteOpen(true);
  };

  const handleScopeSelect = (value: string) => {
    if (scopeQuestion === "edit") {
      if (value === "single") void saveSingleShift();
      else void splitSeriesFromHere();
      return;
    }
    if (scopeQuestion === "delete") {
      if (value === "single") void deleteSingleShift();
      else void endSeriesFromHere();
    }
  };

  const title = seriesEditOpen
    ? "Serie bearbeiten"
    : mode === "edit"
      ? `Schicht bearbeiten · ${formatLongDate(date)}`
      : `Schicht anlegen · ${formatLongDate(date)}`;

  // The series editor writes the rule, not this day — so it gets its own
  // footer: no per-day delete, and a back route to the single occurrence.
  const seriesFooter = (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-end">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => {
          if (occurrenceDraft) {
            setStartTime(occurrenceDraft.startTime);
            setEndTime(occurrenceDraft.endTime);
            setBreakMinutesStr(occurrenceDraft.breakMinutesStr);
            setShiftTypeId(occurrenceDraft.shiftTypeId);
          }
          setSeriesEditOpen(false);
          setOccurrenceDraft(null);
          setError(null);
        }}
        disabled={isSaving}
      >
        Zurück
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void saveSeriesRule()}
        isLoading={isSaving}
        loadingText="Speichern…"
        disabled={
          isSaving ||
          !timesValid ||
          !breakValid ||
          seriesWeekdays.length === 0 ||
          seriesEditClosingDaysLoading
        }
      >
        Serie speichern
      </Button>
    </div>
  );

  const footer = seriesNotice ? (
    <Button type="button" variant="primary" size="md" onClick={onClose}>
      Schließen
    </Button>
  ) : seriesEditOpen ? (
    seriesFooter
  ) : (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center">
      {mode === "edit" && shift && (
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          className="sm:mr-auto"
          onClick={handleDeleteClick}
          disabled={isSaving || isDeleting}
        >
          Schicht löschen
        </Button>
      )}
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isSaving || isDeleting}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={handleSubmit}
        isLoading={isSaving}
        loadingText="Speichern…"
        disabled={
          isSaving ||
          isDeleting ||
          !timesValid ||
          !breakValid ||
          !seriesValid ||
          seriesClosingDaysLoading
        }
      >
        {submitLabel(mode, repeatEnabled)}
      </Button>
    </div>
  );

  const scopeOverlayOpen = scopeQuestion !== null || closingDayPrompt !== null;

  return (
    <>
      {/* Hidden while another confirmation is open — all use the same fixed
          z-index, so stacking them puts the topmost dialog's buttons behind
          this modal's body. */}
      <Modal
        isOpen={isOpen && !confirmDeleteOpen && !scopeOverlayOpen}
        onClose={onClose}
        title={title}
        footer={footer}
      >
        {seriesNotice ? (
          <div className="space-y-3 text-sm">
            <p className="text-gray-700">Die Serie wurde gespeichert.</p>
            <p className="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800">
              {seriesNotice}
            </p>
          </div>
        ) : (
          <div className="space-y-4 text-sm">
            <p className="text-sm text-gray-600">{staffName}</p>
            {isSeriesRow && (
              // The series panel states the rule itself (#2028). Knowing only
              // "Teil einer Serie" leaves the planner guessing which days the
              // series covers and how far it runs — and the way to the whole
              // series used to exist only as a dialog after Speichern.
              <div className="space-y-2 rounded-md bg-gray-50 px-3 py-2.5">
                <p className="text-xs text-gray-600">
                  {shift?.detached
                    ? "Diese Schicht gehört zu einer Serie und wurde für diese Woche angepasst."
                    : "Diese Schicht ist Teil einer Serie."}
                </p>
                {seriesRule && (
                  <p className="text-xs font-medium text-gray-800">
                    {describeSeriesRule(seriesRule)}
                  </p>
                )}
                {seriesRuleError && (
                  <p className="text-xs text-gray-500">
                    Die Serienangaben konnten nicht geladen werden. Änderungen
                    an dieser Schicht sind weiterhin möglich.
                  </p>
                )}
                {!seriesEditOpen && (
                  <Button
                    type="button"
                    variant="outline"
                    size="compact"
                    onClick={openSeriesEdit}
                    disabled={seriesRule === null || isSaving || isDeleting}
                  >
                    Serie bearbeiten
                  </Button>
                )}
              </div>
            )}
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Field label="Beginn">
                <input
                  type="time"
                  value={startTime}
                  onChange={(e) => setStartTime(e.target.value)}
                  className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                />
              </Field>
              <Field label="Ende">
                <input
                  type="time"
                  value={endTime}
                  onChange={(e) => setEndTime(e.target.value)}
                  className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                />
              </Field>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Field label="Pause (Minuten)">
                <input
                  type="number"
                  min={0}
                  max={breakMaxMinutes}
                  inputMode="numeric"
                  value={breakMinutesStr}
                  onChange={(e) => setBreakMinutesStr(e.target.value)}
                  className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                />
              </Field>
            </div>
            {typeOptions.length > 1 && (
              <Field label="Schichtart">
                <CustomSelect
                  value={shiftTypeId}
                  options={typeOptions}
                  onChange={setShiftTypeId}
                  ariaLabel="Schichtart"
                  placeholder="Keine Schichtart"
                />
              </Field>
            )}
            {mode === "edit" && !seriesEditOpen && (
              <Field label="Grund der Änderung (optional)">
                <input
                  type="text"
                  value={changeReason}
                  maxLength={200}
                  onChange={(e) => setChangeReason(e.target.value)}
                  placeholder="z. B. Krankheit, Fortbildung, Tausch"
                  className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 focus:outline-none"
                />
                {/* A permanent series change ("Ab jetzt dauerhaft") re-plans the
                    series and carries no per-day reason, so be honest that the
                    reason only sticks to a single-occurrence change. */}
                {isSeriesRow && (
                  <p className="mt-1 text-xs text-gray-500">
                    Der Grund wird nur bei „Nur diese Woche" gespeichert, nicht
                    bei „Ab jetzt dauerhaft".
                  </p>
                )}
              </Field>
            )}
            {isReplacementRow && (
              <p
                className="rounded-md px-3 py-2 text-xs"
                style={{
                  backgroundColor: `${LOCATION_COLORS.OTHER_ROOM}14`,
                  color: LOCATION_COLORS.OTHER_ROOM,
                }}
              >
                Diese Schicht ist eine Vertretung für eine ausgefallene Schicht.
              </p>
            )}
            {seriesEditOpen && seriesRule && (
              <div className="space-y-3 rounded-md border border-gray-200 p-3">
                <p className="text-xs text-gray-600">
                  {`Die Änderungen gelten ab ${formatShortDate(seriesAppliesFrom)} für alle weiteren Termine der Serie. Termine davor bleiben unverändert, ebenso Termine bis heute.`}
                </p>
                <FieldGroup label="Wochentage">
                  <WeekdayPicker
                    idPrefix="shift-series-edit"
                    weekdays={seriesWeekdays}
                    onToggle={toggleSeriesWeekday}
                  />
                </FieldGroup>
                <FieldGroup label="Wochenrhythmus">
                  <RhythmPicker
                    idPrefix="shift-series-edit"
                    biweekly={seriesBiweekly}
                    onBiweeklyChange={setSeriesBiweekly}
                    abPattern={seriesAbPattern}
                    onAbPatternChange={setSeriesAbPattern}
                    periodHasCycle={seriesPeriodHasCycle}
                    weekHint={
                      seriesDefaultAbPattern !== null
                        ? `Die Woche vom ${formatShortDate(seriesAppliesFrom)} ist Woche ${
                            seriesDefaultAbPattern === 1 ? "A" : "B"
                          }.`
                        : undefined
                    }
                  />
                </FieldGroup>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {/* The date the change actually takes effect: the opened
                      occurrence, moved forward to tomorrow when it already
                      passed — the re-plan never touches today or earlier. */}
                  <Field label="Gilt ab">
                    <input
                      type="text"
                      value={formatShortDate(seriesAppliesFrom)}
                      disabled
                      className="w-full rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-gray-500"
                    />
                  </Field>
                  {/* FieldGroup, not Field — see the create block: a label
                      around the DatePicker re-dispatches clicks. */}
                  <FieldGroup label="Gültig bis (optional)">
                    <DatePicker
                      value={
                        seriesValidUntil === ""
                          ? null
                          : parseISODate(seriesValidUntil)
                      }
                      onChange={(picked) =>
                        setSeriesValidUntil(picked ? toISODate(picked) : "")
                      }
                      placeholder="Datum auswählen"
                    />
                  </FieldGroup>
                </div>
                {/* Say it before the save fails: a segment whose last day has
                    arrived has nothing left for the re-plan to change (#2028). */}
                {!seriesHasRemainingOccurrence && (
                  <p
                    className="rounded-md px-3 py-2 text-xs"
                    style={{
                      backgroundColor: `${LOCATION_COLORS.SCHOOLYARD}14`,
                      color: LOCATION_COLORS.SCHOOLYARD,
                    }}
                  >
                    {seriesNoOccurrenceMessage}
                  </p>
                )}
              </div>
            )}
            {mode === "edit" &&
              shift &&
              !isReplacementRow &&
              !seriesEditOpen && (
                <div className="space-y-3 rounded-md border border-gray-200 p-3">
                  <label
                    htmlFor="shift-cancel-toggle"
                    className="flex items-center gap-2"
                  >
                    <Checkbox
                      id="shift-cancel-toggle"
                      checked={cancelled}
                      // Keep the replacement rows across an un-check/re-check: the
                      // section is hidden while unchecked and the save only sends
                      // replacements when `cancelled` is true, so clearing here
                      // would erase the existing covers for a reactivation that was
                      // never submitted — and re-checking must bring them back
                      // rather than silently wiping every cover on save (#1841).
                      onChange={(e) => setCancelled(e.target.checked)}
                    />
                    <span className="text-sm font-medium text-gray-800">
                      Diese Schicht fällt aus
                    </span>
                  </label>
                  {cancelled && (
                    <div className="space-y-3">
                      <p className="text-xs text-gray-500">
                        Die Schicht zählt nicht mehr zur geplanten Zeit. Ohne
                        Vertretung bleibt die Lücke bewusst offen; sonst tragen
                        Sie eine oder mehrere Ersatzpersonen ein.
                      </p>
                      {replacements.map((row, index) => (
                        <div
                          key={getStableObjectKey(row, "replacement")}
                          className="space-y-2 rounded-md border border-gray-100 bg-gray-50/60 p-2.5"
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
                              Vertretung {index + 1}
                            </span>
                            <Button
                              type="button"
                              variant="ghost"
                              size="compact"
                              onClick={() => removeReplacement(index)}
                            >
                              Entfernen
                            </Button>
                          </div>
                          <CustomSelect
                            value={row.staffId}
                            options={replacementStaffOptions}
                            onChange={(value) =>
                              updateReplacement(index, { staffId: value })
                            }
                            ariaLabel={`Ersatzperson ${index + 1}`}
                            placeholder="Person auswählen"
                          />
                          <div className="grid grid-cols-2 gap-2">
                            <Field label="Beginn">
                              <input
                                type="time"
                                aria-label={`Vertretung ${index + 1} Beginn`}
                                value={row.startTime}
                                onChange={(e) =>
                                  updateReplacement(index, {
                                    startTime: e.target.value,
                                  })
                                }
                                className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                              />
                            </Field>
                            <Field label="Ende">
                              <input
                                type="time"
                                aria-label={`Vertretung ${index + 1} Ende`}
                                value={row.endTime}
                                onChange={(e) =>
                                  updateReplacement(index, {
                                    endTime: e.target.value,
                                  })
                                }
                                className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                              />
                            </Field>
                          </div>
                        </div>
                      ))}
                      <Button
                        type="button"
                        variant="outline"
                        size="md"
                        onClick={addReplacement}
                        disabled={replacementStaffOptions.length <= 1}
                      >
                        + Vertretung hinzufügen
                      </Button>
                      {replacementStaffOptions.length <= 1 && (
                        <p className="text-xs text-gray-500">
                          Keine andere Person verfügbar, um eine Vertretung
                          einzutragen.
                        </p>
                      )}
                    </div>
                  )}
                </div>
              )}
            {mode === "create" && (
              <div className="space-y-3 rounded-md border border-gray-200 p-3">
                <label
                  htmlFor="shift-series-toggle"
                  className="flex items-center gap-2"
                >
                  <Checkbox
                    id="shift-series-toggle"
                    checked={repeatEnabled}
                    onChange={(e) => setRepeatEnabled(e.target.checked)}
                  />
                  <span className="text-sm font-medium text-gray-800">
                    Als Serie wiederholen
                  </span>
                </label>
                {repeatEnabled && (
                  <div className="space-y-3">
                    <FieldGroup label="Wochentage">
                      <WeekdayPicker
                        idPrefix="shift-series"
                        weekdays={weekdays}
                        onToggle={toggleWeekday}
                      />
                    </FieldGroup>
                    <Field label="Kalenderzeitraum">
                      {periods !== null && periodOptions.length === 0 ? (
                        <p className="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800">
                          Kein aktiver Kalenderzeitraum vorhanden. Bitte zuerst
                          unter Planung → Kalenderzeiträume einen Zeitraum
                          anlegen.
                        </p>
                      ) : (
                        <CustomSelect
                          value={periodId}
                          options={periodOptions}
                          onChange={setPeriodId}
                          ariaLabel="Kalenderzeitraum"
                          placeholder={
                            periods === null ? "Lade Zeiträume…" : "Auswählen"
                          }
                        />
                      )}
                    </Field>
                    <FieldGroup label="Wochenrhythmus">
                      <RhythmPicker
                        idPrefix="shift-series"
                        biweekly={biweekly}
                        onBiweeklyChange={setBiweekly}
                        abPattern={abPattern}
                        onAbPatternChange={(next) => {
                          setAbTouched(true);
                          setAbPattern(next);
                        }}
                        periodHasCycle={periodHasCycle}
                        weekHint={
                          defaultAbPattern !== null
                            ? `Die Woche vom ${formatShortDate(date)} ist Woche ${
                                defaultAbPattern === 1 ? "A" : "B"
                              }.`
                            : undefined
                        }
                      />
                    </FieldGroup>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <Field label="Gültig ab">
                        <input
                          type="text"
                          value={formatShortDate(date)}
                          disabled
                          className="w-full rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-gray-500"
                        />
                      </Field>
                      {/* FieldGroup, not Field: the DatePicker trigger and
                          calendar are buttons — inside Field's <label> a
                          click would re-dispatch onto the first labelable
                          child (see FieldGroup comment below). */}
                      <FieldGroup label="Gültig bis (optional)">
                        <DatePicker
                          value={
                            validUntil === "" ? null : parseISODate(validUntil)
                          }
                          onChange={(picked) =>
                            setValidUntil(picked ? toISODate(picked) : "")
                          }
                          placeholder="Datum auswählen"
                        />
                      </FieldGroup>
                    </div>
                    <p className="text-xs text-gray-500">
                      Ohne Enddatum läuft die Serie bis zum Ende des
                      Kalenderzeitraums; mit Enddatum bis einschließlich diesem
                      Tag. Schichten werden ab morgen eingeplant; bestehende
                      Schichten bleiben unverändert.
                    </p>
                  </div>
                )}
              </div>
            )}
            {error && (
              <p className="rounded-md bg-red-50 px-3 py-2 text-xs text-red-700">
                {error}
              </p>
            )}
          </div>
        )}
      </Modal>
      {closingDayPrompt !== null && (
        <ClosingDayConfirmModal
          dateISO={closingDayPrompt.conflict.dateISO}
          reason={closingDayPrompt.conflict.reason}
          subject="schicht"
          onCancel={() => setClosingDayPrompt(null)}
          onConfirm={() => {
            const { action, confirmationKey } = closingDayPrompt;
            confirmedClosingConflict.current = confirmationKey;
            setClosingDayPrompt(null);
            if (action === "seriesRule") void saveSeriesRule();
            else void handleSubmit();
          }}
        />
      )}
      <ConfirmDeleteModal
        isOpen={confirmDeleteOpen}
        title="Schicht löschen"
        description={
          <>
            Die geplante Schicht am <strong>{formatLongDate(date)}</strong> für{" "}
            <strong>{staffName}</strong> wird gelöscht.
          </>
        }
        gate={{ mode: "twoStep" }}
        loading={isDeleting}
        error=""
        onConfirm={deleteSingleShift}
        onClose={() => setConfirmDeleteOpen(false)}
      />
      <ChoiceModal
        isOpen={isOpen && scopeQuestion === "edit"}
        onClose={() => setScopeQuestion(null)}
        title="Änderung übernehmen"
        description="Diese Schicht ist Teil einer Serie. Wofür soll die Änderung gelten?"
        options={[
          {
            value: "single",
            label: "Nur diese Woche",
            description:
              "Nur dieser Termin wird angepasst; die Serie bleibt unverändert.",
          },
          {
            value: "following",
            label: "Ab jetzt dauerhaft",
            description:
              "Die Serie wird ab diesem Datum geteilt und mit den neuen Zeiten fortgeführt.",
          },
        ]}
        onSelect={handleScopeSelect}
        isBusy={isSaving}
      />
      <ChoiceModal
        isOpen={isOpen && scopeQuestion === "delete"}
        onClose={() => setScopeQuestion(null)}
        title="Schicht löschen"
        description="Diese Schicht ist Teil einer Serie. Was soll gelöscht werden?"
        options={[
          {
            value: "single",
            label: "Nur diese Woche",
            description:
              "Nur dieser Termin entfällt; die Serie plant ihn nicht erneut ein.",
          },
          {
            value: "following",
            label: "Ab jetzt dauerhaft",
            description:
              "Die Serie endet ab diesem Datum; einzeln angepasste Termine bleiben bestehen.",
          },
        ]}
        onSelect={handleScopeSelect}
        isBusy={isDeleting}
      />
    </>
  );
}

/** One line describing what a series does: "Mo, Mi · 08:00–16:00 · Woche A ·
 *  bis 31.01.2027". Without it "Teil einer Serie" leaves the planner guessing
 *  which days are covered and how long the rule runs (#2028). */
function describeSeriesRule(rule: SeriesRule): string {
  const parts = [
    formatWeekdays(rule.weekdays),
    `${rule.startTime}–${rule.endTime}`,
  ];
  if (rule.weekPattern === 1) parts.push("Woche A");
  if (rule.weekPattern === 2) parts.push("Woche B");
  parts.push(
    rule.validUntil
      ? // valid_until is exclusive; the last day WITH a shift is the day before.
        `bis ${formatShortDate(dayBeforeISO(rule.validUntil))}`
      : "bis Ende des Kalenderzeitraums",
  );
  return parts.filter(Boolean).join(" · ");
}

function submitLabel(mode: ShiftEditMode, repeatEnabled: boolean): string {
  if (mode === "edit") return "Änderungen speichern";
  return repeatEnabled ? "Serie anlegen" : "Schicht anlegen";
}

function Field({
  label,
  children,
}: {
  readonly label: string;
  readonly children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </span>
      {children}
    </label>
  );
}

// Field variant for control GROUPS (weekday checkboxes, rhythm buttons).
// Deliberately a div: wrapping a group in a <label> makes clicks on any
// child bubble into the label's activation behavior, which re-dispatches
// the click onto the FIRST labelable descendant and silently toggles the
// wrong control.
function FieldGroup({
  label,
  children,
}: {
  readonly label: string;
  readonly children: React.ReactNode;
}) {
  return (
    <div>
      <span className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </span>
      {children}
    </div>
  );
}

// The "Gültig bis" picker is inclusive; the backend's valid_until is
// exclusive (timetable split convention) — convert by adding one day.
function dayAfterISO(isoDate: string): string {
  const d = parseISODate(isoDate);
  d.setDate(d.getDate() + 1);
  return toISODate(d);
}

// Inverse of dayAfterISO: turns the stored exclusive valid_until back into the
// inclusive "Gültig bis" the picker and the summary line show.
function dayBeforeISO(isoDate: string): string {
  const d = parseISODate(isoDate);
  d.setDate(d.getDate() - 1);
  return toISODate(d);
}

function isoWeekdayOf(isoDate: string): number {
  const [y, m, d] = isoDate.split("-").map(Number);
  const day = new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1).getDay();
  return day === 0 ? 7 : day;
}

function formatShortDate(isoDate: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  const date = new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1);
  return date.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatLongDate(isoDate: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  const date = new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1);
  return date.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    weekday: "long",
    day: "2-digit",
    month: "long",
    year: "numeric",
  });
}

function shiftDurationMinutes(
  startTime: string,
  endTime: string,
): number | null {
  const start = parseClockMinutes(startTime);
  const end = parseClockMinutes(endTime);
  if (start === null || end === null || end <= start) return null;
  return end - start;
}

function parseClockMinutes(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) return null;
  return hours * 60 + minutes;
}

function parseBreakMinutes(raw: string, maxMinutes: number): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0 || n > maxMinutes) {
    return null;
  }
  return n;
}
