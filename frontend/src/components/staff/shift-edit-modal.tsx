"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { DatePicker } from "~/components/ui/date-picker";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  findPeriodForDate,
  weekPatternForDate,
  type CalendarPeriod,
} from "~/lib/calendar-period-helpers";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
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
const WEEKDAY_REQUIRED_MESSAGE =
  "Bitte wählen Sie mindestens einen Wochentag aus.";

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
  /** Vorbelegte Zeiten für eine neue Schicht ("HH:MM"), z. B. aus der im
   *  Viertelstunden-Raster aufgezogenen Spanne (#3818). Nur bei `create`. */
  readonly initialStartTime?: string;
  readonly initialEndTime?: string;
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
  initialStartTime,
  initialEndTime,
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
    if (initialStartTime && initialEndTime) {
      // Aufgezogene Spanne aus dem Viertelstunden-Raster: die Pause folgt der
      // Länge (mehr als sechs Stunden: 30 Minuten), damit ein kurzer Block wie
      // eine Randstunde nicht mit einer Pause startet, die länger ist als er.
      const span = shiftDurationMinutes(initialStartTime, initialEndTime) ?? 0;
      return {
        startTime: initialStartTime,
        endTime: initialEndTime,
        breakMinutes: span > 360 ? 30 : 0,
        shiftTypeId: "",
      };
    }
    return {
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: "",
    };
  }, [shift, initialStartTime, initialEndTime]);

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
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const deleteErrors = useApiFormError();
  const periodsLoad = useApiLoadError();
  const seriesRuleLoad = useApiLoadError();
  const [periodsReload, setPeriodsReload] = useState(0);
  const [seriesRuleReload, setSeriesRuleReload] = useState(0);
  // „Wiederholen“ sendet den aktuellen Stand, nicht den vom Fehlerzeitpunkt.
  const latestSaveRef = useRef<() => void>(() => undefined);
  const latestDeleteRef = useRef<() => void>(() => undefined);
  // Which scope the last series save chose, so a retry repeats it.
  const scopeChoiceRef = useRef<"single" | "following" | null>(null);

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
  // Ferien und Schließtage (#3820): ohne Haken lässt die Serie sie aus.
  const [includeSchoolBreaks, setIncludeSchoolBreaks] = useState(false);
  // After a series create/split with skipped days the modal shows this
  // notice instead of closing, so the admin sees which days were left out.
  const [seriesNotice, setSeriesNotice] = useState<string | null>(null);
  // Scope question ("Nur diese Woche" / "Ab jetzt dauerhaft" / "Alle Termine
  // der Serie") for edits and deletes of a series-backed row.
  const [scopeQuestion, setScopeQuestion] = useState<"edit" | null>(null);
  const [deleteScope, setDeleteScope] = useState<"single" | "following" | null>(
    null,
  );

  // Serienbearbeitung (#2028): the rule behind this shift, plus the state of
  // the "Alle Termine der Serie" editor. The rule is loaded whenever a series
  // row is opened so its rhythm is visible right away instead of only after
  // the user tries to save.
  const [seriesRule, setSeriesRule] = useState<SeriesRule | null>(null);
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
  const [seriesIncludeSchoolBreaks, setSeriesIncludeSchoolBreaks] =
    useState(false);

  const clearFormError = formErrors.clear;
  const clearDeleteError = deleteErrors.clear;
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
    clearFormError();
    clearDeleteError();
    setConfirmDeleteOpen(false);
    setRepeatEnabled(false);
    setWeekdays([isoWeekdayOf(date)]);
    setBiweekly(false);
    setAbPattern(1);
    setAbTouched(false);
    setValidUntil("");
    setSeriesNotice(null);
    setIncludeSchoolBreaks(false);
    setScopeQuestion(null);
    setSeriesRule(null);
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
  }, [
    isOpen,
    initial,
    date,
    shift,
    existingReplacements,
    clearFormError,
    clearDeleteError,
  ]);

  const showPeriodsLoadError = periodsLoad.show;
  const clearPeriodsLoadError = periodsLoad.clear;
  const showSeriesRuleLoadError = seriesRuleLoad.show;
  const clearSeriesRuleLoadError = seriesRuleLoad.clear;

  // Calendar periods load once the series section is opened; the period is
  // required (it bounds the series and anchors Woche A/B). A series row needs
  // them too: its own period decides whether Woche A/B is available at all
  // (#2028).
  useEffect(() => {
    if ((!repeatEnabled && !isSeriesRow) || periods !== null) return;
    let cancelled = false;
    clearPeriodsLoadError();
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
        // Periods stay unloaded: an empty list would claim the school has
        // no calendar period at all.
        void showPeriodsLoadError(err, {
          object: "die Liste der Kalenderzeiträume",
          retry: () => setPeriodsReload((n) => n + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    repeatEnabled,
    isSeriesRow,
    periods,
    date,
    periodsReload,
    showPeriodsLoadError,
    clearPeriodsLoadError,
  ]);

  // The rule behind a series shift (#2028). Loaded on open so the modal can
  // state the rhythm up front — "Diese Schicht ist Teil einer Serie" alone
  // does not tell the planner what the series actually does.
  const seriesId = shift?.seriesId ?? null;
  useEffect(() => {
    if (!isOpen || mode !== "edit" || seriesId === null) return;
    let cancelled = false;
    clearSeriesRuleLoadError();
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
        void showSeriesRuleLoadError(err, {
          object: "die Serie",
          retry: () => setSeriesRuleReload((n) => n + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    isOpen,
    mode,
    seriesId,
    seriesRuleReload,
    showSeriesRuleLoadError,
    clearSeriesRuleLoadError,
  ]);

  const selectedPeriod = useMemo(
    () => periods?.find((p) => p.id === periodId) ?? null,
    [periods, periodId],
  );
  const periodHasCycle = (selectedPeriod?.weekCycleLength ?? 1) > 1;
  const defaultAbPattern = weekPatternForDate(selectedPeriod, date);
  const effectiveWeekPattern = biweekly && periodHasCycle ? abPattern : 0;

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
  const seriesHasRemainingOccurrence =
    seriesHasDateRange &&
    (seriesPeriod === null || seriesEditClosingDayWindow?.dates.length !== 0);
  const seriesNoOccurrenceMessage = seriesHasDateRange
    ? `Für die gewählten Wochentage und den Wochenrhythmus bleibt ab ${formatShortDate(seriesAppliesFrom)} kein Termin mehr. Setzen Sie „Gültig bis" auf ein späteres Datum oder ändern Sie die Wiederholung.`
    : `Diese Serie endet am ${formatShortDate(seriesValidUntil)}, Änderungen wirken aber erst ab ${formatShortDate(seriesAppliesFrom)}. Setzen Sie „Gültig bis" auf ein späteres Datum, um die Serie fortzuführen.`;

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
    // The button stays disabled until the rule has loaded.
    if (!seriesRule) return;
    formErrors.clear();
    setOccurrenceDraft({ startTime, endTime, breakMinutesStr, shiftTypeId });
    setSeriesWeekdays([...seriesRule.weekdays].sort((a, b) => a - b));
    setSeriesBiweekly(seriesRule.weekPattern !== 0);
    setSeriesAbPattern(seriesRule.weekPattern === 2 ? 2 : 1);
    // Stored valid_until is exclusive, the picker inclusive.
    setSeriesValidUntil(
      seriesRule.validUntil ? dayBeforeISO(seriesRule.validUntil) : "",
    );
    setSeriesIncludeSchoolBreaks(seriesRule.includeSchoolBreaks);
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
      formErrors.invalid(WEEKDAY_REQUIRED_MESSAGE);
      return;
    }
    if (!seriesHasRemainingOccurrence) {
      formErrors.invalid(seriesNoOccurrenceMessage);
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
        includeSchoolBreaks: seriesIncludeSchoolBreaks,
      });
      finishSeriesMutation(result);
    } catch (err: unknown) {
      logger.error("shift_series_rule_save_failed", {
        series_id: seriesRule.id,
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "die Serie",
        retry: () => latestSaveRef.current(),
      });
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
    const notices: string[] = [];
    if (result.skippedNonWorkingDays > 0) {
      notices.push(schoolBreaksNotice(result.skippedNonWorkingDays));
    }
    if (result.skippedDates.length > 0) {
      notices.push(
        `An folgenden Tagen besteht bereits eine Schicht, sie wurden übersprungen: ${result.skippedDates
          .map(formatShortDate)
          .join(", ")}.`,
      );
    }
    if (notices.length > 0) {
      setSeriesNotice(notices.join(" "));
      return;
    }
    onClose();
  };

  const validateInputs = (): boolean => {
    formErrors.clear();
    if (!timesValid) {
      formErrors.invalid("Das Ende muss nach dem Beginn liegen.", {
        end_time: "Bitte eine spätere Zeit wählen.",
      });
      return false;
    }
    if (!breakValid || breakMinutes === null) {
      formErrors.invalid(
        `Die Pause muss zwischen 0 und ${breakMaxMinutes} Minuten liegen.`,
        { break_minutes: `Bitte 0 bis ${breakMaxMinutes} Minuten eintragen.` },
      );
      return false;
    }
    if (repeatEnabled && weekdays.length === 0) {
      formErrors.invalid(WEEKDAY_REQUIRED_MESSAGE);
      return false;
    }
    if (repeatEnabled && periodId === "") {
      formErrors.invalid("Bitte wählen Sie einen Kalenderzeitraum aus.", {
        calendar_period_id: "Bitte einen Zeitraum wählen.",
      });
      return false;
    }
    if (mode === "edit" && cancelled) {
      for (const row of replacements) {
        if (row.staffId === "") {
          formErrors.invalid(
            "Bitte wählen Sie für jede Vertretung eine Person aus.",
          );
          return false;
        }
        if (!(
          row.startTime !== "" &&
          row.endTime !== "" &&
          row.startTime < row.endTime
        )) {
          formErrors.invalid(
            "Bei jeder Vertretung muss das Ende nach dem Beginn liegen.",
          );
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
        weekPattern: effectiveWeekPattern,
        validFrom: date,
        // The picker is inclusive ("Gültig bis" = last day WITH a shift);
        // the API's valid_until is exclusive — send the day after.
        validUntil: validUntil === "" ? null : dayAfterISO(validUntil),
        includeSchoolBreaks,
      });
      finishSeriesMutation(result);
    } catch (err: unknown) {
      logger.error("shift_series_create_failed", {
        staff_id: staffId,
        date,
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "die Serie",
        retry: () => latestSaveRef.current(),
      });
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
      await formErrors.show(err, {
        object: "die Schicht",
        retry: () => latestSaveRef.current(),
      });
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
      await formErrors.show(err, {
        object: "die Serie",
        retry: () => latestSaveRef.current(),
      });
    } finally {
      setIsSaving(false);
      setScopeQuestion(null);
    }
  };

  const handleSubmit = async () => {
    scopeChoiceRef.current = null;
    if (!validateInputs()) return;
    if (mode === "create" && repeatEnabled) {
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
    deleteErrors.clear();
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
      // The confirmation stays open and says why.
      await deleteErrors.show(err, {
        object: "die Schicht",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setIsDeleting(false);
      setScopeQuestion(null);
    }
  };

  const endSeriesFromHere = async () => {
    if (!shift?.seriesId) return;
    setIsDeleting(true);
    deleteErrors.clear();
    try {
      await staffShiftSeriesService.endSeries(
        shift.seriesId,
        seriesEffectiveDate,
      );
      setConfirmDeleteOpen(false);
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("shift_series_end_failed", {
        series_id: shift.seriesId,
        date: seriesEffectiveDate,
        error: err instanceof Error ? err.message : String(err),
      });
      await deleteErrors.show(err, {
        object: "die Serie",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setIsDeleting(false);
      setScopeQuestion(null);
    }
  };

  // Löschen (#3110): ein Dialog; bei einer Serienschicht liegt die Scope-Wahl
  // (nur diese Woche / ab jetzt dauerhaft) als Slot in der ConfirmDeleteModal.
  const handleDeleteClick = () => {
    deleteErrors.clear();
    setDeleteScope(isSeriesRow ? null : "single");
    setConfirmDeleteOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (deleteScope === "following") await endSeriesFromHere();
    else await deleteSingleShift();
  };

  useLayoutEffect(() => {
    // Retry repeats the action that failed, with what the form holds now.
    latestSaveRef.current = () => {
      if (seriesEditOpen) void saveSeriesRule();
      else if (scopeChoiceRef.current === "following") {
        if (validateInputs()) void splitSeriesFromHere();
      } else if (scopeChoiceRef.current === "single") {
        if (validateInputs()) void saveSingleShift();
      } else void handleSubmit();
    };
    latestDeleteRef.current = () => void handleConfirmDelete();
  });

  const handleScopeSelect = (value: string) => {
    scopeChoiceRef.current = value === "single" ? "single" : "following";
    if (value === "single") void saveSingleShift();
    else void splitSeriesFromHere();
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
          formErrors.clear();
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
          isSaving || !timesValid || !breakValid || seriesWeekdays.length === 0
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
          isSaving || isDeleting || !timesValid || !breakValid || !seriesValid
        }
      >
        {submitLabel(mode, repeatEnabled)}
      </Button>
    </div>
  );

  const scopeOverlayOpen = scopeQuestion !== null;

  /** Binds a plain control to its API field: name for focus, marking. */
  const fieldControl = (name: string) => {
    const invalid = formErrors.fieldError(name) !== undefined;
    return {
      name,
      "aria-invalid": invalid ? true : undefined,
      "aria-describedby": invalid ? `shift-${name}-error` : undefined,
    };
  };
  const fieldHint = (name: string) => {
    const hint = formErrors.fieldError(name);
    return hint ? (
      <p
        id={`shift-${name}-error`}
        className="text-moto-red-strong mt-1 text-xs"
      >
        {hint}
      </p>
    ) : null;
  };

  return (
    <>
      {/* Hidden while another confirmation is open — all use the same fixed
          z-index, so stacking them puts the topmost dialog's buttons behind
          this modal's body. */}
      <SlideOver
        open={isOpen && !confirmDeleteOpen && !scopeOverlayOpen}
        onOpenChange={(next) => {
          if (!next) onClose();
        }}
      >
        <SlideOverContent widthClass="sm:w-[760px]">
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0">
              <SlideOverTitle>{title}</SlideOverTitle>
            </div>
            <SlideOverCloseButton />
          </SlideOverHeader>
          <SlideOverBody error={seriesNotice ? null : formErrors.error}>
            {seriesNotice ? (
              <div className="space-y-3 text-sm">
                <p className="text-gray-700">Die Serie wurde gespeichert.</p>
                <p className="bg-moto-amber/10 text-moto-amber-strong rounded-md px-3 py-2 text-xs">
                  {seriesNotice}
                </p>
              </div>
            ) : (
              <div ref={formRef} className="space-y-4 text-sm">
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
                    <LoadErrorAlert error={seriesRuleLoad.error} />
                    <LoadErrorAlert error={periodsLoad.error} />
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
                      {...fieldControl("start_time")}
                      value={startTime}
                      onChange={(e) => setStartTime(e.target.value)}
                      className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                    />
                    {fieldHint("start_time")}
                  </Field>
                  <Field label="Ende">
                    <input
                      type="time"
                      {...fieldControl("end_time")}
                      value={endTime}
                      onChange={(e) => setEndTime(e.target.value)}
                      className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                    />
                    {fieldHint("end_time")}
                  </Field>
                </div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Field label="Pause (Minuten)">
                    <input
                      type="number"
                      {...fieldControl("break_minutes")}
                      min={0}
                      max={breakMaxMinutes}
                      inputMode="numeric"
                      value={breakMinutesStr}
                      onChange={(e) => setBreakMinutesStr(e.target.value)}
                      className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:outline-none"
                    />
                    {fieldHint("break_minutes")}
                  </Field>
                </div>
                {typeOptions.length > 1 && (
                  <Field label="Schichtart">
                    <CustomSelect
                      value={shiftTypeId}
                      options={typeOptions}
                      onChange={setShiftTypeId}
                      name="shift_type_id"
                      invalid={!!formErrors.fieldError("shift_type_id")}
                      ariaLabel="Schichtart"
                      placeholder="Keine Schichtart"
                    />
                  </Field>
                )}
                {mode === "edit" && !seriesEditOpen && (
                  <Field label="Grund der Änderung (optional)">
                    <input
                      type="text"
                      {...fieldControl("change_reason")}
                      value={changeReason}
                      maxLength={200}
                      onChange={(e) => setChangeReason(e.target.value)}
                      placeholder="z. B. Krankheit, Fortbildung, Tausch"
                      className="focus:border-moto-green w-full rounded-md border border-gray-200 px-3 py-2 focus:outline-none"
                    />
                    {fieldHint("change_reason")}
                    {/* A permanent series change ("Ab jetzt dauerhaft") re-plans the
                    series and carries no per-day reason, so be honest that the
                    reason only sticks to a single-occurrence change. */}
                    {isSeriesRow && (
                      <p className="mt-1 text-xs text-gray-500">
                        Der Grund wird nur bei „Nur diese Woche" gespeichert,
                        nicht bei „Ab jetzt dauerhaft".
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
                    Diese Schicht ist eine Vertretung für eine ausgefallene
                    Schicht.
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
                    <SchoolBreaksCheckbox
                      id="shift-series-edit-school-breaks"
                      checked={seriesIncludeSchoolBreaks}
                      onChange={setSeriesIncludeSchoolBreaks}
                    />
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
                            Die Schicht zählt nicht mehr zur geplanten Zeit.
                            Ohne Vertretung bleibt die Lücke bewusst offen;
                            sonst tragen Sie eine oder mehrere Ersatzpersonen
                            ein.
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
                          {periodsLoad.error ? (
                            <LoadErrorAlert error={periodsLoad.error} />
                          ) : periods !== null && periodOptions.length === 0 ? (
                            <p className="bg-moto-amber/10 text-moto-amber-strong rounded-md px-3 py-2 text-xs">
                              Kein aktiver Kalenderzeitraum vorhanden. Bitte
                              zuerst unter Planung → Kalenderzeiträume einen
                              Zeitraum anlegen.
                            </p>
                          ) : (
                            <CustomSelect
                              value={periodId}
                              options={periodOptions}
                              onChange={setPeriodId}
                              name="calendar_period_id"
                              invalid={
                                !!formErrors.fieldError("calendar_period_id")
                              }
                              ariaLabel="Kalenderzeitraum"
                              placeholder={
                                periods === null
                                  ? "Lade Zeiträume…"
                                  : "Auswählen"
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
                                validUntil === ""
                                  ? null
                                  : parseISODate(validUntil)
                              }
                              onChange={(picked) =>
                                setValidUntil(picked ? toISODate(picked) : "")
                              }
                              placeholder="Datum auswählen"
                            />
                          </FieldGroup>
                        </div>
                        <SchoolBreaksCheckbox
                          id="shift-series-school-breaks"
                          checked={includeSchoolBreaks}
                          onChange={setIncludeSchoolBreaks}
                        />
                        <p className="text-xs text-gray-500">
                          Ohne Enddatum läuft die Serie bis zum Ende des
                          Kalenderzeitraums; mit Enddatum bis einschließlich
                          diesem Tag. Schichten werden ab morgen eingeplant;
                          bestehende Schichten bleiben unverändert.
                        </p>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </SlideOverBody>
          <SlideOverFooter className="flex-row justify-end gap-2">
            {footer}
          </SlideOverFooter>
        </SlideOverContent>
      </SlideOver>
      <ConfirmDeleteModal
        isOpen={confirmDeleteOpen}
        title="Schicht löschen"
        description={
          isSeriesRow ? (
            <>
              Die Schicht am <strong>{formatLongDate(date)}</strong> für{" "}
              <strong>{staffName}</strong> ist Teil einer Serie.
            </>
          ) : (
            <>
              Die geplante Schicht am <strong>{formatLongDate(date)}</strong>{" "}
              für <strong>{staffName}</strong> wird gelöscht.
            </>
          )
        }
        scope={
          isSeriesRow
            ? {
                label: "Was soll gelöscht werden?",
                name: "shift-delete-scope",
                value: deleteScope,
                onChange: (value) =>
                  setDeleteScope(
                    value === "following" ? "following" : "single",
                  ),
                options: [
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
                ],
              }
            : undefined
        }
        gate={{ mode: "twoStep" }}
        confirmLabel={isSeriesRow ? "Löschen" : "Endgültig löschen"}
        loading={isDeleting}
        error={deleteErrors.error}
        onConfirm={handleConfirmDelete}
        onClose={() => {
          deleteErrors.clear();
          setConfirmDeleteOpen(false);
        }}
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

/** Ferien und Schließtage je Serie (#3820). Ohne Haken bleiben sie frei;
 *  Feiertage sind immer frei. */
function SchoolBreaksCheckbox({
  id,
  checked,
  onChange,
}: {
  readonly id: string;
  readonly checked: boolean;
  readonly onChange: (checked: boolean) => void;
}) {
  return (
    <label htmlFor={id} className="flex items-start gap-2">
      <Checkbox
        id={id}
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>
        <span className="block text-sm font-medium text-gray-800">
          Auch in den Ferien und an Schließtagen planen
        </span>
        <span className="block text-xs text-gray-500">
          Ohne Haken bleiben diese Tage frei. An Feiertagen gibt es nie eine
          Schicht.
        </span>
      </span>
    </label>
  );
}

function schoolBreaksNotice(count: number): string {
  return count === 1
    ? "1 Tag liegt in den Ferien, an einem Schließtag oder Feiertag und bleibt frei."
    : `${count} Tage liegen in den Ferien, an Schließtagen oder Feiertagen und bleiben frei.`;
}
