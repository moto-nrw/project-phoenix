import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";

import { useToast } from "~/contexts/ToastContext";
import type { ActivityCategory } from "~/lib/activity-helpers";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import {
  weekCycleSlotForDate,
  weekPatternForDate,
} from "~/lib/calendar-period-helpers";
import {
  findFirstClosingDayConflict,
  type ClosingDayConflict,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchPlannerActivityCategories,
  fetchPlannerGroups,
  fetchPlannerRooms,
} from "~/lib/planner-reference-api";
import { staffService } from "~/lib/staff-api";
import { getSchoolYear } from "~/lib/student-helpers";
import { useTenant } from "~/lib/tenant-context";
import { timetableService } from "~/lib/timetable-api";
import {
  chunkDateRange,
  getGermanWeekdayLong,
  materializedRecurrenceDates,
  resolveTemplateCalendarPeriodId,
  weekdayDatesInRange,
} from "~/lib/timetable-helpers";
import { useDebounce } from "~/lib/use-debounce";
import {
  emptyForm,
  fetchAllStudentOptions,
  formFromInstance,
  formFromSeries,
  initialPrimaryStaffID,
  initialStaffIDs,
  initialStudentIDs,
  isoWeekday,
  parseRequiredStaffOverride,
  schoolClassLabel,
  sortPeople,
  targetCohortActionLabel,
} from "./form-model";
import type { EventFormState, PersonOption, RepeatMode } from "./form-model";
import type {
  ConflictWarningItem,
  CreateInstanceBody,
  CreateTemplateBody,
  EditedInWindowResult,
  EnrichedInstance,
  ShiftCoverageCheckParams,
  ShiftCoverageWarningItem,
  TargetGroupType,
  TimetableTemplate,
  UpdateTemplateBody,
} from "~/lib/timetable-types";

export interface RoomOption {
  id: number;
  name: string;
  building?: string;
}

export interface GroupOption {
  id: string;
  name: string;
}

const MAX_SUPPORTED_TARGET_GRADE_LEVEL = 13;
const STUDENT_LOAD_ERROR =
  "Die Kinderliste konnte nicht vollständig geladen werden. Die Kinderzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.";
const STAFF_LOAD_ERROR =
  "Die Personalliste konnte nicht vollständig geladen werden. Die Personalzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.";
const COVERAGE_CHECK_ERROR =
  "Die Dienstplan-Abdeckung konnte nicht geprüft werden. Speichern ist weiterhin möglich.";
const COVERAGE_CHECK_TIMEOUT_MS = 5_000;

function checkShiftCoverageWithSignal(
  probe: ShiftCoverageCheckParams,
  signal: AbortSignal,
) {
  const aborted = new Promise<never>((_, reject) => {
    signal.addEventListener(
      "abort",
      () => reject(new DOMException("Coverage check aborted", "AbortError")),
      { once: true },
    );
  });
  return Promise.race([
    timetableService.checkShiftCoverage(probe, { signal }),
    aborted,
  ]);
}

export type TimetableEventModalResult =
  | { kind: "instance"; instance: EnrichedInstance }
  | { kind: "series"; seriesId: string; linkedInstanceId?: string };

const logger = createLogger({ component: "TimetableEventModal" });

export const WEEKDAYS = [1, 2, 3, 4, 5] as const;

/**
 * Backend cap for a single materialization window
 * (MaxMaterializationWindowDays in services/schedule/materialization_service.go).
 * Whole-period runs are split into windows of this size.
 */
const MATERIALIZE_CHUNK_DAYS = 56;

/**
 * Shown when the primary template write succeeded but a follow-up
 * materialize/replan chunk failed. The form must still close in that case —
 * re-submitting would duplicate the already-saved template.
 */
const FOLLOW_UP_WARNING =
  "Regeltermin gespeichert, aber nicht alle Termine konnten eingetragen werden. Die fehlenden Termine werden beim nächsten automatischen Lauf ergänzt.";

/** Plain-language repeat presets shown in the quick variant. */
type QuickRepeatPreset =
  "einmalig" | "woechentlich-am" | "jeden-wochentag" | "benutzerdefiniert";

function mondayOfWeekISO(dateISO: string): string {
  const date = parseISODate(dateISO);
  date.setDate(date.getDate() - (isoWeekday(dateISO) - 1));
  return toISODate(date);
}

export interface UseEventFormParams {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (result: TimetableEventModalResult) => void;
  defaultDate: string;
  weekFrom?: string;
  weekTo?: string;
  calendarPeriods: CalendarPeriod[];
  defaultCalendarPeriodId?: string | null;
  initialInstance: EnrichedInstance | null;
  initialSeries: TimetableTemplate | null;
  convertInstance: EnrichedInstance | null;
  onDeleteSeries?: (
    template: TimetableTemplate,
    effectiveDate: string,
  ) => Promise<void>;
  defaultRepeat: RepeatMode;
  variant: "full" | "quick";
  defaultStartTime?: string;
  defaultEndTime?: string;
  canCheckShiftCoverage: boolean;
  closingDayRanges?: readonly ClosingDayRange[];
}

/**
 * State container of the Termin form: form state and its reset, the reference
 * data loads, the advisory conflict/coverage probes, validation, the body
 * builders, and every write path (submit, series scope flow, #1875 lost-edits
 * confirmation, series delete). The modal renders what this returns; it holds
 * no state of its own beyond the shared UI contexts.
 */
export function useEventForm({
  isOpen,
  onClose,
  onSaved,
  defaultDate,
  weekFrom,
  weekTo,
  calendarPeriods,
  defaultCalendarPeriodId,
  initialInstance,
  initialSeries,
  convertInstance,
  onDeleteSeries,
  defaultRepeat,
  variant,
  defaultStartTime,
  defaultEndTime,
  canCheckShiftCoverage,
  closingDayRanges,
}: UseEventFormParams) {
  const { tenant } = useTenant();
  const {
    success: toastSuccess,
    error: toastError,
    warning: toastWarning,
  } = useToast();
  const [form, setForm] = useState<EventFormState>(() =>
    emptyForm(
      defaultDate,
      defaultCalendarPeriodId,
      defaultRepeat,
      defaultStartTime,
      defaultEndTime,
    ),
  );
  const [initialStudentIDsSnapshot, setInitialStudentIDsSnapshot] = useState(
    () => initialStudentIDs(initialInstance, initialSeries, convertInstance),
  );
  const [initialStaffIDsSnapshot, setInitialStaffIDsSnapshot] = useState(() =>
    initialStaffIDs(initialInstance, initialSeries, convertInstance),
  );
  const [initialPrimaryStaffIDSnapshot, setInitialPrimaryStaffIDSnapshot] =
    useState(() =>
      initialPrimaryStaffID(initialInstance, initialSeries, convertInstance),
    );
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [groups, setGroups] = useState<GroupOption[]>([]);
  const [students, setStudents] = useState<PersonOption[]>([]);
  const [staff, setStaff] = useState<PersonOption[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [loadingStudents, setLoadingStudents] = useState(false);
  const [studentLoadError, setStudentLoadError] = useState<string | null>(null);
  const [loadingStaff, setLoadingStaff] = useState(false);
  const [staffLoadError, setStaffLoadError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteEffectiveDate, setDeleteEffectiveDate] = useState("");
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [deletingSeries, setDeletingSeries] = useState(false);
  const [expanded, setExpanded] = useState(variant === "full");
  // Validated room id stashed while the Dreifach-Frage dialog (US-5) is open.
  const [pendingSeriesEdit, setPendingSeriesEdit] = useState<{
    roomId: number;
  } | null>(null);
  const [scopeClosingDayWarning, setScopeClosingDayWarning] = useState<{
    conflict: ClosingDayConflict;
    scope: "all" | "following";
    roomId: number;
    template: TimetableTemplate;
    periodEnd: string | undefined;
    groupId: string;
  } | null>(null);
  // #1875: a series edit that would discard single-occurrence changes is
  // deferred here until the user confirms the loss (with the concrete dates).
  // `onConfirm` runs the actual destructive edit — shared by the occurrence
  // scope picker (handleScopeSelect) and the direct Regeltermin edit
  // (handleSubmit); `scope` only drives the warning wording.
  const [lostEdits, setLostEdits] = useState<{
    scope: "all" | "following";
    result: EditedInWindowResult;
    onConfirm: () => Promise<void>;
  } | null>(null);
  const [conflictWarnings, setConflictWarnings] = useState<
    ConflictWarningItem[]
  >([]);
  const [coverageWarnings, setCoverageWarnings] = useState<
    ShiftCoverageWarningItem[]
  >([]);
  const [coverageWarningCount, setCoverageWarningCount] = useState(0);
  const [coverageCheckError, setCoverageCheckError] = useState<string | null>(
    null,
  );
  // Reference data can outlive a close/reopen or a changed edit target. Keep
  // the shared, student, and staff load generations separate so a retry may
  // replace only its own roster while stale completions cannot overwrite a
  // newer modal state.
  const referenceLoadSeq = useRef(0);
  const studentLoadSeq = useRef(0);
  const staffLoadSeq = useRef(0);
  const invalidateReferenceLoads = useCallback(() => {
    referenceLoadSeq.current++;
    studentLoadSeq.current++;
    staffLoadSeq.current++;
  }, []);
  // Monotonically increasing probe id so stale responses are dropped.
  const probeSeq = useRef(0);
  const coverageProbeSeq = useRef(0);
  // A materialized occurrence only exposes its own staffing pin, not the
  // template override it may inherit. Remember user intent separately so an
  // unrelated all/following edit can preserve the freshly fetched template.
  const requiredStaffTouched = useRef(false);
  // An occurrence may carry substitute rows that do not belong to the series
  // roster. Preserve the fetched template roster for all/following edits until
  // the user explicitly changes Personal; otherwise a title-only split would
  // promote a substitute to planned series staff and erase its deviation role.
  const staffRosterTouched = useRef(false);
  // An occurrence form starts from the occurrence's OWN Listenart snapshot,
  // which may differ from the series template's classification (a per-occurrence
  // override, or a blank that inherits). So the form value alone can't tell an
  // untouched inherited value from a deliberate series-wide edit. Remember
  // explicit user intent separately: an all/following edit echoes the fetched
  // template's Listenart until the user actually changes the field, then it
  // writes form.listKind to the series (#1565 review).
  const listKindTouched = useRef(false);
  // Once the user picks Woche A/B explicitly, date or period changes must not
  // override that choice with the recomputed default parity — and the pick
  // survives switching the repeat mode away and back within the same modal
  // session. null = no manual pick yet (defaults apply).
  const manualWeekPattern = useRef<1 | 2 | null>(null);
  // Mirror of the last validateForm() result, readable synchronously right
  // after the call (the fieldErrors state only lands on the next render). The
  // wizard shell uses it to decide whether the CURRENT step is clean without
  // re-implementing any validation rule.
  const lastValidationErrors = useRef<Record<string, string>>({});

  const isEditingInstance = initialInstance !== null;
  const isEditingSeries = initialSeries !== null;
  const isConverting = convertInstance !== null;
  const isSeriesFlow = form.repeat !== "none" || isEditingSeries;
  const choiceDialogOpen = pendingSeriesEdit !== null;
  const canDeleteSeries = isEditingSeries && initialSeries && onDeleteSeries;
  const selectedCalendarPeriod = useMemo(
    () => calendarPeriods.find((item) => item.id === form.calendarPeriodId),
    [calendarPeriods, form.calendarPeriodId],
  );
  const abWeekCycleSlot = useMemo(
    () => weekCycleSlotForDate(selectedCalendarPeriod, form.date),
    [selectedCalendarPeriod, form.date],
  );
  const abWeekParity: 1 | 2 | null =
    abWeekCycleSlot === 1 || abWeekCycleSlot === 2 ? abWeekCycleSlot : null;
  // The backend materializes the A/B week_pattern by matching the period's
  // cycle slot. It only represents a fortnightly schedule when the period has
  // an anchored two-week cycle. New biweekly series are therefore disallowed
  // for every other period (validateForm); stored series keep their pattern
  // editable. Weeks beyond B (Woche C/D) only exist in longer cycles and are
  // surfaced via the hint below.
  const biweeklyUnavailable =
    selectedCalendarPeriod?.weekCycleLength !== 2 ||
    !selectedCalendarPeriod.weekCycleAnchor;
  const abWeekBeyondCycle = abWeekCycleSlot !== null && abWeekCycleSlot > 2;
  const abWeekHint = abWeekBeyondCycle
    ? `Woche vom ${formatDate(mondayOfWeekISO(form.date))} ist Woche ${String.fromCharCode(64 + abWeekCycleSlot)} und liegt außerhalb des A/B-Rhythmus. Ein 14-tägiger Termin findet in dieser Woche nicht statt.`
    : abWeekParity !== null
      ? `Woche vom ${formatDate(mondayOfWeekISO(form.date))} ist Woche ${abWeekParity === 1 ? "A" : "B"}`
      : "Kein A/B-Zyklus im Planungszeitraum hinterlegt (Standard: Woche B)";

  // Keep the default A/B choice in sync with the selected week while the user
  // has not picked one explicitly. Series edits keep their stored pattern.
  useEffect(() => {
    if (!isOpen || isEditingSeries || form.repeat !== "biweekly") return;
    if (manualWeekPattern.current !== null) return;
    const parity = abWeekParity ?? 2;
    if (form.weekPattern !== parity) {
      setForm((prev) => ({ ...prev, weekPattern: parity }));
    }
  }, [isOpen, isEditingSeries, form.repeat, form.weekPattern, abWeekParity]);
  const gradeLevelMax = tenant?.gradeLevelMax;
  const targetGradeOptions = useMemo(() => {
    if (gradeLevelMax === undefined) return [];
    const options = Array.from({ length: gradeLevelMax }, (_, index) => {
      const value = String(index + 1);
      return { value, label: `Jahrgang ${value}`, disabled: false };
    });
    if (
      form.targetGradeLevel !== "" &&
      !options.some((option) => option.value === form.targetGradeLevel)
    ) {
      const gradeLevel = Number(form.targetGradeLevel);
      const supported =
        Number.isInteger(gradeLevel) &&
        gradeLevel >= 1 &&
        gradeLevel <= MAX_SUPPORTED_TARGET_GRADE_LEVEL;
      options.push({
        value: form.targetGradeLevel,
        label: `Jahrgang ${form.targetGradeLevel} (${supported ? "bestehend" : "ungültig"})`,
        disabled: true,
      });
    }
    return options;
  }, [form.targetGradeLevel, gradeLevelMax]);
  const preservesExistingTargetGrade =
    initialSeries?.targetGradeLevel !== undefined &&
    initialSeries.targetGradeLevel !== null &&
    form.targetGradeLevel === String(initialSeries.targetGradeLevel) &&
    Number.isInteger(initialSeries.targetGradeLevel) &&
    initialSeries.targetGradeLevel >= 1 &&
    initialSeries.targetGradeLevel <= MAX_SUPPORTED_TARGET_GRADE_LEVEL;
  const preservesGradeAboveTenantCap =
    preservesExistingTargetGrade &&
    gradeLevelMax !== undefined &&
    Number(form.targetGradeLevel) > gradeLevelMax;
  const studentRosterEditable = !loadingStudents && studentLoadError === null;
  const studentIDsForSave = studentRosterEditable
    ? form.studentIds
    : initialStudentIDsSnapshot;
  const staffRosterEditable = !loadingStaff && staffLoadError === null;
  const staffIDsForSave = staffRosterEditable
    ? form.staffIds
    : initialStaffIDsSnapshot;
  const primaryStaffIDForSave = staffRosterEditable
    ? form.primaryStaffId
    : initialPrimaryStaffIDSnapshot;

  useEffect(() => {
    if (!isOpen) {
      invalidateReferenceLoads();
      return;
    }
    const referenceSeq = ++referenceLoadSeq.current;
    const studentSeq = ++studentLoadSeq.current;
    const staffSeq = ++staffLoadSeq.current;
    const isCurrentReferenceLoad = () =>
      referenceLoadSeq.current === referenceSeq;
    const isCurrentStudentLoad = () => studentLoadSeq.current === studentSeq;
    const isCurrentStaffLoad = () => staffLoadSeq.current === staffSeq;

    const nextForm = initialSeries
      ? formFromSeries(initialSeries, defaultDate, defaultCalendarPeriodId)
      : convertInstance
        ? formFromInstance(convertInstance, defaultCalendarPeriodId, "weekly")
        : initialInstance
          ? formFromInstance(initialInstance, defaultCalendarPeriodId)
          : emptyForm(
              defaultDate,
              defaultCalendarPeriodId,
              defaultRepeat,
              defaultStartTime,
              defaultEndTime,
            );
    setInitialStudentIDsSnapshot([...nextForm.studentIds]);
    setInitialStaffIDsSnapshot([...nextForm.staffIds]);
    setInitialPrimaryStaffIDSnapshot(nextForm.primaryStaffId);
    requiredStaffTouched.current = false;
    staffRosterTouched.current = false;
    listKindTouched.current = false;
    manualWeekPattern.current = null;
    setForm(nextForm);
    setValidationError(null);
    setFieldErrors({});
    setDeleteConfirmOpen(false);
    setDeleteEffectiveDate(berlinTodayISO());
    setDeleteError(null);
    setDeletingSeries(false);
    setExpanded(variant === "full");
    setPendingSeriesEdit(null);
    setScopeClosingDayWarning(null);
    setLostEdits(null);
    setConflictWarnings([]);
    setCoverageWarnings([]);
    setCoverageWarningCount(0);
    setCoverageCheckError(null);
    setLoadingRefs(true);
    setLoadingStudents(true);
    setStudentLoadError(null);
    setStudents([]);
    setLoadingStaff(true);
    setStaffLoadError(null);
    setStaff([]);

    void Promise.all([
      fetchPlannerRooms()
        .then((items) =>
          items.map((room) => ({
            id: Number(room.id),
            name: room.name ?? room.room_name ?? `Raum ${room.id}`,
            building: room.building ?? undefined,
          })),
        )
        .catch((err: unknown) => {
          logger.error("rooms_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as RoomOption[];
        }),
      fetchPlannerActivityCategories().catch((err: unknown) => {
        logger.error("categories_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return [] as ActivityCategory[];
      }),
      fetchPlannerGroups()
        .then((items) =>
          items.map((group) => ({ id: String(group.id), name: group.name })),
        )
        .catch((err: unknown) => {
          logger.error("groups_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as GroupOption[];
        }),
    ])
      .then(([roomData, categoryData, groupData]) => {
        const sortedRooms = [...roomData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        const sortedCategories = [...categoryData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        const sortedGroups = [...groupData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        if (isCurrentReferenceLoad()) {
          setRooms(sortedRooms);
          setCategories(sortedCategories);
          setGroups(sortedGroups);
          setForm((prev) =>
            prev.categoryId || sortedCategories.length === 0
              ? prev
              : { ...prev, categoryId: sortedCategories[0]?.id ?? "" },
          );
        }
      })
      .finally(() => {
        if (isCurrentReferenceLoad()) setLoadingRefs(false);
      });

    // The student catalog has a stricter permission boundary than the shared
    // planner references. Keep it on an independent lifecycle so users without
    // users:read can still use rooms, categories and groups immediately.
    void fetchAllStudentOptions()
      .then((studentData) => {
        if (!isCurrentStudentLoad()) return;
        setStudents(sortPeople(studentData));
      })
      .catch((err: unknown) => {
        logger.error("students_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!isCurrentStudentLoad()) return;
        setStudents([]);
        setStudentLoadError(STUDENT_LOAD_ERROR);
      })
      .finally(() => {
        if (isCurrentStudentLoad()) setLoadingStudents(false);
      });

    // Staff carries the same users:read boundary as students. Keep it
    // independent from rooms/categories/groups so a 403 cannot masquerade as
    // an empty editable roster or delay the planner references.
    void staffService
      .getAllStaff()
      .then((items) => {
        if (!isCurrentStaffLoad()) return;
        setStaff(
          sortPeople(items.map((item) => ({ id: item.id, name: item.name }))),
        );
      })
      .catch((err: unknown) => {
        logger.error("staff_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!isCurrentStaffLoad()) return;
        setStaff([]);
        setStaffLoadError(STAFF_LOAD_ERROR);
      })
      .finally(() => {
        if (isCurrentStaffLoad()) setLoadingStaff(false);
      });

    return invalidateReferenceLoads;
  }, [
    convertInstance,
    defaultCalendarPeriodId,
    defaultDate,
    defaultEndTime,
    defaultRepeat,
    defaultStartTime,
    initialInstance,
    initialSeries,
    invalidateReferenceLoads,
    isOpen,
    variant,
  ]);

  // Advisory room/staff/student conflicts and Dienstplan coverage are
  // separate reads. Exact shift gaps have the stricter time-tracking
  // permission boundary, and a failed coverage read must not erase valid
  // planning conflicts or disable Speichern.
  const probeKey = JSON.stringify({
    date: form.date,
    startTime: form.startTime,
    endTime: form.endTime,
    roomId: form.roomId,
    staffIds: staffIDsForSave,
    studentIds: studentIDsForSave,
  });
  const debouncedProbeKey = useDebounce(probeKey, 500);
  // The convert flow edits an existing instance too — its own slot must not
  // self-conflict, exactly like the regular instance edit.
  const excludeInstanceId = initialInstance?.id ?? convertInstance?.id;

  const coverageProbe = useMemo(() => {
    if (!form.startTime || !form.endTime || staffIDsForSave.length === 0) {
      return null;
    }
    if (!isSeriesFlow) {
      return {
        dates: form.date ? [form.date] : [],
        startTime: form.startTime,
        endTime: form.endTime,
        staffIds: staffIDsForSave,
        excludeInstanceId: initialInstance?.id,
      };
    }

    const period = calendarPeriods.find(
      (candidate) => candidate.id === form.calendarPeriodId,
    );
    if (!period) return null;
    const today = berlinTodayISO();
    const from =
      initialSeries && today > period.startDate ? today : period.startDate;
    const dates = weekdayDatesInRange(from, period.endDate, form.weekdays);
    if (convertInstance && form.date && !dates.includes(form.date)) {
      dates.push(form.date);
      dates.sort((left, right) => left.localeCompare(right));
    }
    return {
      dates,
      startTime: form.startTime,
      endTime: form.endTime,
      staffIds: staffIDsForSave,
      excludeInstanceId: convertInstance?.id,
      concreteInstanceDate: convertInstance ? form.date : undefined,
      replanActivityGroupId: initialSeries?.id,
      calendarPeriodId: period.id,
      weekPattern: form.weekPattern,
    };
  }, [
    calendarPeriods,
    convertInstance,
    form.calendarPeriodId,
    form.date,
    form.endTime,
    form.weekPattern,
    form.startTime,
    form.weekdays,
    initialInstance?.id,
    initialSeries,
    isSeriesFlow,
    staffIDsForSave,
  ]);
  const coverageProbeKey = JSON.stringify(coverageProbe);
  const debouncedCoverageProbeKey = useDebounce(coverageProbeKey, 500);

  useEffect(() => {
    if (!isOpen) return;
    // While the debounced key lags behind the live form (typing, or a
    // reopen that reset the form), probing would use outdated values —
    // skip and invalidate any in-flight probe for the outdated key.
    if (debouncedProbeKey !== probeKey) {
      probeSeq.current++;
      return;
    }
    const probe = JSON.parse(debouncedProbeKey) as {
      date: string;
      startTime: string;
      endTime: string;
      roomId: string;
      staffIds: string[];
      studentIds: string[];
    };
    const hasResource =
      probe.roomId !== "" ||
      probe.staffIds.length > 0 ||
      probe.studentIds.length > 0;
    if (!probe.date || !probe.startTime || !probe.endTime || !hasResource) {
      setConflictWarnings([]);
      return;
    }
    const seq = ++probeSeq.current;
    timetableService
      .checkConflicts({
        date: probe.date,
        startTime: probe.startTime,
        endTime: probe.endTime,
        roomId: probe.roomId || undefined,
        staffIds: probe.staffIds.length > 0 ? probe.staffIds : undefined,
        studentIds: probe.studentIds.length > 0 ? probe.studentIds : undefined,
        excludeInstanceId,
      })
      .then((result) => {
        if (probeSeq.current !== seq) return; // out-of-order response
        setConflictWarnings(result.warnings);
      })
      .catch((err: unknown) => {
        logger.warn("conflict_probe_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (probeSeq.current === seq) {
          setConflictWarnings([]);
        }
      });
  }, [debouncedProbeKey, excludeInstanceId, isOpen, probeKey]);

  useEffect(() => {
    if (!isOpen) return;
    if (!canCheckShiftCoverage) {
      setCoverageWarnings([]);
      setCoverageWarningCount(0);
      setCoverageCheckError(null);
      return;
    }
    if (debouncedCoverageProbeKey !== coverageProbeKey) {
      coverageProbeSeq.current++;
      return;
    }
    const probe = JSON.parse(debouncedCoverageProbeKey) as typeof coverageProbe;
    if (!probe || probe.dates.length === 0) {
      setCoverageWarnings([]);
      setCoverageWarningCount(0);
      setCoverageCheckError(null);
      return;
    }

    const seq = ++coverageProbeSeq.current;
    const controller = new AbortController();
    const timeoutID = window.setTimeout(
      () => controller.abort(),
      COVERAGE_CHECK_TIMEOUT_MS,
    );
    setCoverageCheckError(null);
    checkShiftCoverageWithSignal(probe, controller.signal)
      .then((result) => {
        if (coverageProbeSeq.current !== seq) return;
        setCoverageWarnings(result.coverageWarnings);
        setCoverageWarningCount(
          result.coverageWarningCount ?? result.coverageWarnings.length,
        );
        setCoverageCheckError(null);
      })
      .catch((err: unknown) => {
        logger.warn("shift_coverage_probe_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (coverageProbeSeq.current === seq) {
          setCoverageWarnings([]);
          setCoverageWarningCount(0);
          setCoverageCheckError(COVERAGE_CHECK_ERROR);
        }
      })
      .finally(() => window.clearTimeout(timeoutID));
    return () => {
      window.clearTimeout(timeoutID);
      controller.abort();
    };
  }, [
    canCheckShiftCoverage,
    coverageProbeKey,
    debouncedCoverageProbeKey,
    isOpen,
  ]);

  // Recheck the exact live payload before every write. The debounced probe
  // gives early feedback while editing, while this awaited probe closes the
  // fast-save race. Warnings and probe failures remain advisory: both are
  // surfaced as durable toasts and the write continues without confirmation.
  const checkCoverageBeforeSave = async (
    probe: ShiftCoverageCheckParams | null,
  ): Promise<void> => {
    if (!canCheckShiftCoverage || !probe || probe.dates.length === 0) return;
    const controller = new AbortController();
    const timeoutID = window.setTimeout(
      () => controller.abort(),
      COVERAGE_CHECK_TIMEOUT_MS,
    );
    try {
      const result = await checkShiftCoverageWithSignal(
        probe,
        controller.signal,
      );
      const warningCount =
        result.coverageWarningCount ?? result.coverageWarnings.length;
      setCoverageWarnings(result.coverageWarnings);
      setCoverageWarningCount(warningCount);
      setCoverageCheckError(null);
      for (const warning of result.coverageWarnings.slice(0, 3)) {
        toastWarning(warning.message, { duration: 10_000 });
      }
      if (warningCount > 3) {
        toastWarning(
          `${warningCount - 3} weitere Dienstplan-Lücken wurden gefunden.`,
          { duration: 10_000 },
        );
      }
    } catch (coverageError) {
      logger.warn("shift_coverage_pre_save_failed", {
        error:
          coverageError instanceof Error
            ? coverageError.message
            : String(coverageError),
      });
      setCoverageWarnings([]);
      setCoverageWarningCount(0);
      setCoverageCheckError(COVERAGE_CHECK_ERROR);
      toastWarning(COVERAGE_CHECK_ERROR, { duration: 10_000 });
    } finally {
      window.clearTimeout(timeoutID);
    }
  };

  const clearFieldError = (key: string) => {
    setFieldErrors((prev) => {
      if (!(key in prev)) return prev;
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const update = <K extends keyof EventFormState>(
    key: K,
    value: EventFormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
    clearFieldError(key);
    // The end-before-start error is stored under endTime but depends on both
    // time fields, so correcting the start time must clear it too.
    if (key === "startTime") clearFieldError("endTime");
    // The cycle-length error is stored under weekPattern but is resolved by
    // switching to a period with an A/B (or no) week cycle.
    if (key === "calendarPeriodId") clearFieldError("weekPattern");
  };

  const updateRepeat = (repeat: RepeatMode) => {
    setForm((prev) => {
      let weekPattern: 0 | 1 | 2 = 0;
      if (repeat === "biweekly") {
        // A manual Woche-A/B pick survives switching the repeat mode away and
        // back; otherwise default to the parity of the currently selected
        // week so the series runs in the week the user is looking at. B is
        // the fallback when the period has no A/B cycle (previous hardcoded
        // behavior).
        const period = calendarPeriods.find(
          (item) => item.id === prev.calendarPeriodId,
        );
        weekPattern =
          manualWeekPattern.current ??
          weekPatternForDate(period, prev.date) ??
          2;
      }
      return { ...prev, repeat, weekPattern };
    });
    setValidationError(null);
    clearFieldError("repeat");
    clearFieldError("weekPattern");
  };

  const toggleWeekday = (iso: number) => {
    setForm((prev) => {
      const has = prev.weekdays.includes(iso);
      const next = has
        ? prev.weekdays.filter((day) => day !== iso)
        : [...prev.weekdays, iso].sort((a, b) => a - b);
      return { ...prev, weekdays: next };
    });
    setValidationError(null);
    clearFieldError("weekdays");
  };

  const changeTargetGroupType = (nextType: TargetGroupType) => {
    setForm((current) => ({
      ...current,
      targetGroupType: nextType,
      targetGradeLevel: nextType === "jahrgang" ? current.targetGradeLevel : "",
      targetSchoolClass: nextType === "klasse" ? current.targetSchoolClass : "",
      educationGroupId: nextType === "gruppe" ? current.educationGroupId : "",
    }));
    setValidationError(null);
    setFieldErrors((current) => {
      const next = { ...current };
      delete next.targetGradeLevel;
      delete next.targetSchoolClass;
      delete next.educationGroupId;
      return next;
    });
  };

  const validateForm = (): { roomId: number; categoryId?: number } | null => {
    const errors: Record<string, string> = {};
    if (form.title.trim() === "") {
      errors.title = "Bitte einen Titel eingeben.";
    }
    if (form.date === "") {
      errors.date = "Bitte ein Datum auswählen.";
    }
    if (form.startTime === "") {
      errors.startTime = "Bitte eine Startzeit angeben.";
    }
    if (form.endTime === "") {
      errors.endTime = "Bitte eine Endzeit angeben.";
    } else if (form.startTime !== "" && form.endTime <= form.startTime) {
      errors.endTime = "Endzeit muss nach der Startzeit liegen.";
    }
    const roomId = Number.parseInt(form.roomId, 10);
    if (!Number.isFinite(roomId) || roomId <= 0) {
      errors.roomId = "Bitte einen Raum auswählen.";
    }
    let categoryId: number | undefined;
    if (isSeriesFlow) {
      const parsedCategoryId = Number.parseInt(form.categoryId, 10);
      if (!Number.isFinite(parsedCategoryId) || parsedCategoryId <= 0) {
        errors.categoryId = "Bitte eine Kategorie auswählen.";
      } else {
        categoryId = parsedCategoryId;
      }
      const calendarPeriodId = Number.parseInt(form.calendarPeriodId, 10);
      if (!Number.isFinite(calendarPeriodId) || calendarPeriodId <= 0) {
        errors.calendarPeriodId = "Bitte einen Planungszeitraum auswählen.";
      }
      if (form.weekdays.length === 0) {
        errors.weekdays = "Bitte mindestens einen Wochentag auswählen.";
      }
      // "Alle 2 Wochen" only genuinely repeats every two weeks in an anchored
      // two-week period. Otherwise the A/B week_pattern either fires weekly or
      // once per longer cycle. Series edits are exempt so their stored pattern
      // stays editable.
      if (
        form.repeat === "biweekly" &&
        !isEditingSeries &&
        biweeklyUnavailable
      ) {
        errors.weekPattern =
          'Der gewählte Planungszeitraum hat keinen verankerten Zwei-Wochen-Zyklus. Eine 14-tägige Wiederholung ist hier nicht möglich; bitte "Jede Woche" wählen.';
      }
      if (form.targetGroupType === "jahrgang") {
        const gradeLevel = Number(form.targetGradeLevel);
        if (form.targetGradeLevel === "") {
          errors.targetGradeLevel = "Bitte einen Jahrgang auswählen.";
        } else if (
          !Number.isInteger(gradeLevel) ||
          gradeLevel < 1 ||
          gradeLevel > MAX_SUPPORTED_TARGET_GRADE_LEVEL ||
          gradeLevelMax === undefined ||
          (gradeLevel > gradeLevelMax && !preservesExistingTargetGrade)
        ) {
          errors.targetGradeLevel = "Bitte einen gültigen Jahrgang auswählen.";
        }
      }
      if (
        form.targetGroupType === "klasse" &&
        form.targetSchoolClass.trim() === ""
      ) {
        errors.targetSchoolClass = "Bitte eine Klasse auswählen.";
      }
      if (form.targetGroupType === "gruppe" && form.educationGroupId === "") {
        errors.educationGroupId = "Bitte eine Gruppe auswählen.";
      }
    }
    setFieldErrors(errors);
    lastValidationErrors.current = errors;
    if (Object.keys(errors).length > 0) {
      // Quick mode hides the series controls; expand so an inline error on
      // a hidden field (Kategorie, Planungszeitraum, Wochentage) is visible.
      if (
        !expanded &&
        (errors.categoryId ??
          errors.calendarPeriodId ??
          errors.weekdays ??
          errors.weekPattern ??
          errors.targetGradeLevel ??
          errors.targetSchoolClass ??
          errors.educationGroupId)
      ) {
        setExpanded(true);
      }
      return null;
    }
    return { roomId, categoryId };
  };

  // -------------------------------------------------------------------
  // Quick-variant repeat presets ("Wiederholt sich"). The select's value
  // is derived back from (repeat, weekdays) so external changes keep it
  // consistent; anything that doesn't match a preset reads as
  // "Benutzerdefiniert".
  // -------------------------------------------------------------------
  const dateWeekday = isoWeekday(form.date);
  const quickPreset: QuickRepeatPreset =
    form.repeat === "none"
      ? "einmalig"
      : form.repeat === "weekly" &&
          form.weekdays.length === 1 &&
          form.weekdays[0] === dateWeekday
        ? "woechentlich-am"
        : form.repeat === "weekly" &&
            form.weekdays.length === WEEKDAYS.length &&
            WEEKDAYS.every((day) => form.weekdays.includes(day))
          ? "jeden-wochentag"
          : "benutzerdefiniert";
  const dateWeekdayName =
    getGermanWeekdayLong(new Date(`${form.date}T00:00:00`)) || "Wochentag";

  const handleQuickPresetChange = (value: string) => {
    switch (value as QuickRepeatPreset) {
      case "einmalig":
        updateRepeat("none");
        break;
      case "woechentlich-am":
        updateRepeat("weekly");
        update(
          "weekdays",
          dateWeekday >= 1 && dateWeekday <= 5 ? [dateWeekday] : [1],
        );
        break;
      case "jeden-wochentag":
        updateRepeat("weekly");
        update("weekdays", [...WEEKDAYS]);
        break;
      case "benutzerdefiniert":
        setExpanded(true);
        break;
    }
  };

  const instanceBody = (roomId: number, activityGroupId?: string) =>
    ({
      date: form.date,
      title: form.title.trim(),
      start_time: form.startTime,
      end_time: form.endTime,
      room_id: roomId,
      notes: form.notes.trim() || undefined,
      list_kind: form.listKind || undefined,
      activity_group_id: activityGroupId ? Number(activityGroupId) : undefined,
      staff_ids: staffIDsForSave.map(Number),
      student_ids: studentIDsForSave.map(Number),
      required_staff: parseRequiredStaffOverride(form.requiredStaff),
    }) satisfies CreateInstanceBody;

  const seriesBody = (
    roomId: number,
    categoryId: number,
  ): CreateTemplateBody => ({
    name: form.title.trim(),
    type: form.type,
    list_kind: form.listKind || undefined,
    weekdays: form.weekdays,
    start_time: form.startTime,
    end_time: form.endTime,
    room_id: roomId,
    category_id: categoryId,
    notes: form.seriesNotes.trim() || undefined,
    education_group_id: form.educationGroupId
      ? Number(form.educationGroupId)
      : undefined,
    target_group_type: form.targetGroupType,
    target_grade_level:
      form.targetGroupType === "jahrgang" && form.targetGradeLevel
        ? Number(form.targetGradeLevel)
        : undefined,
    target_school_class:
      form.targetGroupType === "klasse" && form.targetSchoolClass
        ? form.targetSchoolClass.trim()
        : undefined,
    calendar_period_id: Number(form.calendarPeriodId),
    week_pattern: form.weekPattern,
    required_staff: parseRequiredStaffOverride(form.requiredStaff),
    student_ids: studentIDsForSave.map(Number),
    staff_ids: staffIDsForSave.map(Number),
    primary_staff_id: primaryStaffIDForSave
      ? Number(primaryStaffIDForSave)
      : undefined,
  });

  const findPeriod = (id?: string): CalendarPeriod | undefined =>
    id ? calendarPeriods.find((period) => period.id === id) : undefined;

  /**
   * Maps the fetched template plus the fields the instance-edit form
   * actually edits (title, times, room, people) onto an update/split body.
   * Weekdays, category, period and week pattern come from the template —
   * the instance form never edits them.
   */
  const templateBodyFromForm = (
    template: TimetableTemplate,
    roomId: number,
  ): UpdateTemplateBody => {
    const firstSchedule = template.schedules[0];
    const calendarPeriodId =
      resolveTemplateCalendarPeriodId(template) ?? form.calendarPeriodId;
    const weekdays = template.schedules.map((schedule) => schedule.weekday);
    // A materialized occurrence may have a deliberate roster override. When
    // users:read is unavailable, the occurrence snapshot is authoritative only
    // for the single-instance scope; an all/following write targets the fetched
    // template and must preserve that template's own roster instead.
    const templateStudentIDs = studentRosterEditable
      ? form.studentIds
      : template.studentIds;
    const templateStaffIDs =
      staffRosterEditable && staffRosterTouched.current
        ? form.staffIds
        : template.staffIds;
    const primaryStaffId =
      staffRosterEditable && staffRosterTouched.current
        ? form.primaryStaffId || template.primaryStaffId
        : template.primaryStaffId;
    return {
      name: form.title.trim(),
      type: template.type,
      // This builder runs only for occurrence-scope edits ("Alle Termine" /
      // "Diesen und folgende"), both of which target the SERIES. `form.listKind`
      // starts as the OCCURRENCE's own classification (its snapshot, or a
      // per-occurrence override), so blindly copying it onto the template would
      // clear the series (stale empty) or promote a per-occurrence override to
      // every future occurrence. Until the user actually changes Listenart, echo
      // the fetched template's value (`?? null` clears only an already-unset
      // series — a no-op). Once they edit it, both scope descriptions promise the
      // change applies to the series, so write the new value to the series;
      // `|| null` sends an explicit clear when they pick "Keine". Both the update
      // and split endpoints honor either (#1565 review).
      list_kind: listKindTouched.current
        ? form.listKind || null
        : (template.listKind ?? null),
      weekdays: weekdays.length > 0 ? weekdays : [1],
      start_time: form.startTime,
      end_time: form.endTime,
      room_id: roomId,
      category_id: Number(template.categoryId),
      // Preserve the series' own Wochennotiz verbatim — an instance-scope edit
      // (all/following) must never wipe it. It is read-only in this flow.
      notes: template.notes ?? undefined,
      education_group_id: template.educationGroupId
        ? Number(template.educationGroupId)
        : undefined,
      // Zielgruppe is preserved from the existing template, not edited —
      // this body builder only carries the fields a single-instance edit
      // actually changes (title, times, room, people).
      target_group_type: template.targetGroupType,
      target_grade_level: template.targetGradeLevel,
      target_school_class: template.targetSchoolClass,
      max_participants:
        template.maxParticipants > 0 ? template.maxParticipants : undefined,
      // An occurrence form starts from the occurrence's own pin, which is
      // blank when it inherits from the series. Preserve the fetched template
      // override until the user explicitly edits this field.
      required_staff: requiredStaffTouched.current
        ? parseRequiredStaffOverride(form.requiredStaff)
        : (template.requiredStaffOverride ?? null),
      week_pattern: firstSchedule?.weekPattern ?? 0,
      calendar_period_id: calendarPeriodId
        ? Number(calendarPeriodId)
        : undefined,
      student_ids: templateStudentIDs.map(Number),
      staff_ids: templateStaffIDs.map(Number),
      primary_staff_id:
        primaryStaffId && templateStaffIDs.includes(primaryStaffId)
          ? Number(primaryStaffId)
          : undefined,
    };
  };

  /**
   * Rebuilds a template's future planned instances: chunked scoped
   * re-plan from today through the period end (each window stays within
   * the backend's 56-day materialization cap). Chunk failures are caught
   * here — the primary template write already succeeded, so they must not
   * bubble into the form's error path. Returns false when a chunk failed.
   */
  const replanTemplateFuture = async (
    templateId: string,
    periodEndISO?: string,
  ): Promise<boolean> => {
    const endISO =
      periodEndISO ?? findPeriod(form.calendarPeriodId)?.endDate ?? weekTo;
    if (!endISO) return true;
    const chunks = chunkDateRange(
      berlinTodayISO(),
      endISO,
      MATERIALIZE_CHUNK_DAYS,
    );
    for (const chunk of chunks) {
      try {
        const result = await timetableService.replanWeek(
          chunk.from,
          chunk.to,
          templateId,
        );
        // A precondition like "no_active_period" applies to every chunk.
        if (result.warnings.some((w) => w.code === "no_active_period")) break;
      } catch (err) {
        logger.error("series_replan_chunk_failed", {
          template_id: templateId,
          from: chunk.from,
          to: chunk.to,
          error: err instanceof Error ? err.message : String(err),
        });
        return false;
      }
    }
    return true;
  };

  /**
   * Creates the template materializing the whole selected period (US-1
   * Phase 3). The backend caps one materialization window at 56 days, so
   * the create call carries only the first chunk; the rest follow as
   * separate materialize calls. Falls back to the visible week when no
   * period is resolvable. Follow-up chunk failures are caught here (the
   * template already exists; a thrown error would invite a duplicating
   * retry) and reported via `followUpOk`.
   */
  const createSeriesForPeriod = async (
    body: CreateTemplateBody,
  ): Promise<{
    templateId: string;
    totalCreated: number;
    followUpOk: boolean;
  }> => {
    const period = findPeriod(form.calendarPeriodId);
    const chunks = period
      ? chunkDateRange(period.startDate, period.endDate, MATERIALIZE_CHUNK_DAYS)
      : [];
    const [firstChunk, ...restChunks] = chunks;
    if (!firstChunk) {
      const created = await timetableService.createTemplate({
        ...body,
        materialize_from: weekFrom,
        materialize_to: weekTo,
      });
      return {
        templateId: created.templateId,
        totalCreated: created.instancesCreated ?? 0,
        followUpOk: true,
      };
    }
    const created = await timetableService.createTemplate({
      ...body,
      materialize_from: firstChunk.from,
      materialize_to: firstChunk.to,
    });
    let totalCreated = created.instancesCreated ?? 0;
    let followUpOk = true;
    for (const chunk of restChunks) {
      try {
        const result = await timetableService.materialize(chunk.from, chunk.to);
        totalCreated += result.instancesCreated;
        if (result.warnings.some((w) => w.code === "no_active_period")) break;
      } catch (err) {
        logger.error("series_materialize_chunk_failed", {
          template_id: created.templateId,
          from: chunk.from,
          to: chunk.to,
          error: err instanceof Error ? err.message : String(err),
        });
        followUpOk = false;
        break;
      }
    }
    return { templateId: created.templateId, totalCreated, followUpOk };
  };

  /**
   * Materializes the whole selected period after linking a converted
   * instance. Failures are caught (template and link are already saved)
   * and reported via the return value.
   */
  const materializePeriodAfterConvert = async (): Promise<boolean> => {
    const period = findPeriod(form.calendarPeriodId);
    const chunks = period
      ? chunkDateRange(period.startDate, period.endDate, MATERIALIZE_CHUNK_DAYS)
      : [];
    try {
      if (chunks.length === 0) {
        if (weekFrom && weekTo) {
          await timetableService.materialize(weekFrom, weekTo);
        }
        return true;
      }
      for (const chunk of chunks) {
        const result = await timetableService.materialize(chunk.from, chunk.to);
        if (result.warnings.some((w) => w.code === "no_active_period")) break;
      }
      return true;
    } catch (err) {
      logger.error("series_materialize_chunk_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      return false;
    }
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (submitting) return;
    if (isEditingInstance && initialInstance?.status !== "planned") return;
    const parsed = validateForm();
    if (!parsed) return;

    // US-5 Dreifach-Frage: editing an instance that belongs to a series
    // first asks for the scope instead of writing immediately.
    if (!isSeriesFlow && initialInstance?.activityGroupId) {
      setPendingSeriesEdit({ roomId: parsed.roomId });
      return;
    }

    setSubmitting(true);
    try {
      await checkCoverageBeforeSave(coverageProbe);
      if (!isSeriesFlow) {
        const body = instanceBody(
          parsed.roomId,
          initialInstance?.activityGroupId,
        );
        const saved = initialInstance
          ? await timetableService.update(initialInstance.id, body)
          : await timetableService.create(body);
        toastSuccess(
          initialInstance ? "Termin gespeichert" : "Termin angelegt",
        );
        onSaved({ kind: "instance", instance: saved });
        onClose();
        return;
      }

      if (!parsed.categoryId) {
        setFieldErrors({ categoryId: "Bitte eine Kategorie auswählen." });
        return;
      }

      if (initialSeries) {
        const seriesId = initialSeries.id;
        const categoryId = parsed.categoryId;
        const runSeriesEdit = async () => {
          await timetableService.updateTemplate(
            seriesId,
            seriesBody(parsed.roomId, categoryId),
          );
          if (await replanTemplateFuture(seriesId)) {
            toastSuccess("Regeltermin gespeichert");
          } else {
            toastWarning(FOLLOW_UP_WARNING);
          }
          onSaved({ kind: "series", seriesId });
        };

        // #1875: a direct Regeltermin edit runs the same destructive re-plan as
        // the "Alle Termine" scope, so it needs the same lost-edits warning.
        // replanTemplateFuture re-plans [today, period end]; probe that window.
        const editsTo =
          findPeriod(form.calendarPeriodId)?.endDate ??
          weekTo ??
          berlinTodayISO();
        let lost: EditedInWindowResult | null = null;
        try {
          const probe = await timetableService.countEditedInWindow(
            seriesId,
            berlinTodayISO(),
            editsTo,
          );
          if (probe.count > 0) lost = probe;
        } catch (probeErr) {
          logger.warn("edited_in_window_probe_failed", {
            error:
              probeErr instanceof Error ? probeErr.message : String(probeErr),
          });
        }
        if (lost) {
          setLostEdits({
            scope: "all",
            result: lost,
            onConfirm: runSeriesEdit,
          });
          setSubmitting(false);
          return;
        }

        await runSeriesEdit();
        onClose();
        return;
      }

      if (convertInstance) {
        const created = await timetableService.createTemplate(
          seriesBody(parsed.roomId, parsed.categoryId),
        );
        await timetableService.update(convertInstance.id, {
          ...instanceBody(parsed.roomId, created.templateId),
          // The value entered during conversion belongs to the new series.
          // Keep the seed occurrence unpinned so future series edits apply.
          required_staff: null,
        });
        if (await materializePeriodAfterConvert()) {
          toastSuccess("Termin wiederholt");
        } else {
          toastWarning(FOLLOW_UP_WARNING);
        }
        onSaved({
          kind: "series",
          seriesId: created.templateId,
          linkedInstanceId: convertInstance.id,
        });
      } else {
        const { templateId, totalCreated, followUpOk } =
          await createSeriesForPeriod(
            seriesBody(parsed.roomId, parsed.categoryId),
          );
        if (followUpOk) {
          toastSuccess(
            totalCreated > 0
              ? `Regeltermin angelegt: ${totalCreated} Termin${totalCreated === 1 ? "" : "e"} eingetragen`
              : "Regeltermin angelegt",
          );
        } else {
          toastWarning(FOLLOW_UP_WARNING);
        }
        onSaved({ kind: "series", seriesId: templateId });
      }
      onClose();
    } catch (err) {
      logger.error("event_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Termin konnte nicht gespeichert werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  /**
   * Executes the scope picked in the Dreifach-Frage dialog (US-5).
   * "single" keeps the plain instance PUT; "following" splits the
   * template at the instance date; "all" updates the template and
   * rebuilds future planned instances. A changed Datum only applies to
   * "single" — series-wide scopes ignore it.
   */
  const seriesCoverageProbe = (
    template: TimetableTemplate,
    body: UpdateTemplateBody,
    fromISO: string,
    replanActivityGroupId?: string,
  ) => {
    const calendarPeriodId =
      resolveTemplateCalendarPeriodId(template) ?? form.calendarPeriodId;
    const period = findPeriod(calendarPeriodId);
    const staffIds = (body.staff_ids ?? []).map(String);
    if (
      !period ||
      body.weekdays.length === 0 ||
      staffIds.length === 0 ||
      !body.start_time ||
      !body.end_time
    ) {
      return null;
    }
    const from = fromISO > period.startDate ? fromISO : period.startDate;
    let dates = weekdayDatesInRange(from, period.endDate, body.weekdays);
    // Without replanActivityGroupId the backend cannot apply the segment's
    // validity envelope, so the probe caps itself at the schedules'
    // validUntil (exclusive boundary): a split successor inherits it and
    // never creates occurrences on or after that date. All schedules of a
    // segment share the boundary; the latest one wins if they ever diverge,
    // and one open-ended schedule leaves the probe uncapped.
    let latestValidUntil: string | undefined;
    for (const schedule of template.schedules) {
      if (!schedule.validUntil) {
        latestValidUntil = undefined;
        break;
      }
      if (!latestValidUntil || schedule.validUntil > latestValidUntil) {
        latestValidUntil = schedule.validUntil;
      }
    }
    if (latestValidUntil) {
      const boundary = latestValidUntil;
      dates = dates.filter((date) => date < boundary);
    }
    return {
      dates,
      startTime: body.start_time,
      endTime: body.end_time,
      staffIds,
      replanActivityGroupId,
      calendarPeriodId: period.id,
      weekPattern: body.week_pattern ?? 0,
    };
  };

  const findScopeClosingDayConflict = (
    template: TimetableTemplate,
    body: UpdateTemplateBody,
    fromISO: string,
  ): ClosingDayConflict | null => {
    const calendarPeriodId =
      resolveTemplateCalendarPeriodId(template) ?? form.calendarPeriodId;
    const period = findPeriod(calendarPeriodId);
    if (!period) return null;
    const validity = template.schedules[0];
    const dates = materializedRecurrenceDates({
      period,
      fromISO,
      weekdays: body.weekdays,
      weekPattern: body.week_pattern ?? 0,
      validFrom: validity?.validFrom,
      validUntil: validity?.validUntil,
    });
    return findFirstClosingDayConflict(closingDayRanges, dates);
  };

  const handleScopeError = (scope: string, err: unknown) => {
    logger.error("series_scope_save_failed", {
      scope,
      error: err instanceof Error ? err.message : String(err),
    });
    const raw =
      err instanceof Error
        ? err.message
        : "Termin konnte nicht gespeichert werden";
    // Backend rejects splits whose effective date already passed; the
    // raw English message is meaningless to school staff.
    const msg = raw.includes("effective_date must not be in the past")
      ? "Der Stichtag liegt in der Vergangenheit. Bitte einen künftigen Termin der Serie wählen."
      : raw;
    setPendingSeriesEdit(null);
    setScopeClosingDayWarning(null);
    setLostEdits(null);
    setValidationError(msg);
    toastError(msg);
  };

  /**
   * Applies a series-wide edit ("all" or "following") against a pre-loaded
   * template. Split out of handleScopeSelect so the #1875 lost-edits warning
   * can defer it until the user confirms.
   */
  const performSeriesEdit = async (
    typedScope: "all" | "following",
    roomId: number,
    template: TimetableTemplate,
    periodEnd: string | undefined,
    groupId: string,
  ) => {
    if (!initialInstance) return;
    const body = templateBodyFromForm(template, roomId);
    const scopeProbe = seriesCoverageProbe(
      template,
      body,
      typedScope === "following" ? initialInstance.date : berlinTodayISO(),
      typedScope === "all" ? groupId : undefined,
    );
    await checkCoverageBeforeSave(scopeProbe);
    if (typedScope === "following") {
      const effectiveDate = initialInstance.date;
      const chunks = chunkDateRange(
        effectiveDate,
        periodEnd ?? weekTo ?? effectiveDate,
        MATERIALIZE_CHUNK_DAYS,
      );
      const split = await timetableService.splitTemplate(groupId, {
        ...body,
        effective_date: effectiveDate,
        materialize_from: effectiveDate,
        materialize_to: chunks[0]?.to ?? effectiveDate,
      });
      // Beyond the first 56-day window the split leaves the old
      // template's stale planned rows in place; a scoped re-plan per
      // chunk clears them and materializes the successor template.
      // Chunk failures are follow-up only — the split already
      // succeeded, so warn and close instead of re-opening the form.
      let followUpOk = true;
      for (const chunk of chunks.slice(1)) {
        try {
          const result = await timetableService.replanWeek(
            chunk.from,
            chunk.to,
            groupId,
          );
          if (result.warnings.some((w) => w.code === "no_active_period")) {
            break;
          }
        } catch (chunkErr) {
          logger.error("series_replan_chunk_failed", {
            template_id: groupId,
            from: chunk.from,
            to: chunk.to,
            error:
              chunkErr instanceof Error ? chunkErr.message : String(chunkErr),
          });
          followUpOk = false;
          break;
        }
      }
      if (followUpOk) {
        toastSuccess(`Regeltermin ab ${formatDate(effectiveDate)} geändert`);
      } else {
        toastWarning(FOLLOW_UP_WARNING);
      }
      onSaved({ kind: "series", seriesId: split.newTemplateId });
    } else {
      await timetableService.updateTemplate(groupId, body);
      if (await replanTemplateFuture(groupId, periodEnd)) {
        toastSuccess("Regeltermin gespeichert");
      } else {
        toastWarning(FOLLOW_UP_WARNING);
      }
      onSaved({ kind: "series", seriesId: groupId });
    }
  };

  const continuePreparedSeriesEdit = async ({
    typedScope,
    roomId,
    template,
    periodEnd,
    groupId,
  }: {
    typedScope: "all" | "following";
    roomId: number;
    template: TimetableTemplate;
    periodEnd: string | undefined;
    groupId: string;
  }) => {
    if (!initialInstance) return;
    // #1875: before rebuilding the series, check whether single-occurrence
    // edits in the affected window would be discarded. "following" also
    // rematerializes individually-deleted occurrences; "all" preserves them.
    const editsFrom =
      typedScope === "following" ? initialInstance.date : berlinTodayISO();
    const editsTo = periodEnd ?? weekTo ?? editsFrom;
    let lost: EditedInWindowResult | null = null;
    try {
      const probe = await timetableService.countEditedInWindow(
        groupId,
        editsFrom,
        editsTo,
        typedScope === "following",
      );
      if (probe.count > 0) lost = probe;
    } catch (probeErr) {
      logger.warn("edited_in_window_probe_failed", {
        error: probeErr instanceof Error ? probeErr.message : String(probeErr),
      });
    }

    if (lost) {
      setPendingSeriesEdit(null);
      setLostEdits({
        scope: typedScope,
        result: lost,
        onConfirm: () =>
          performSeriesEdit(typedScope, roomId, template, periodEnd, groupId),
      });
      return;
    }

    await performSeriesEdit(typedScope, roomId, template, periodEnd, groupId);
    setPendingSeriesEdit(null);
    onClose();
  };

  const handleScopeSelect = async (scope: string) => {
    if (submitting) return;
    const pending = pendingSeriesEdit;
    const groupId = initialInstance?.activityGroupId;
    if (!pending || !initialInstance || !groupId) return;

    // Single-occurrence edit ("Nur diese Woche"): unchanged plain instance PUT.
    if (scope === "single") {
      setSubmitting(true);
      try {
        await checkCoverageBeforeSave(coverageProbe);
        const saved = await timetableService.update(
          initialInstance.id,
          instanceBody(pending.roomId, groupId),
        );
        toastSuccess("Termin gespeichert");
        onSaved({ kind: "instance", instance: saved });
        setPendingSeriesEdit(null);
        onClose();
      } catch (err) {
        handleScopeError(scope, err);
      } finally {
        setSubmitting(false);
      }
      return;
    }

    const typedScope = scope === "following" ? "following" : "all";
    setSubmitting(true);
    try {
      const template = await timetableService.getTemplate(groupId);
      const templateCalendarPeriodId =
        resolveTemplateCalendarPeriodId(template);
      const periodEnd =
        findPeriod(templateCalendarPeriodId)?.endDate ??
        findPeriod(form.calendarPeriodId)?.endDate;
      const body = templateBodyFromForm(template, pending.roomId);
      const scopeFrom =
        typedScope === "following" ? initialInstance.date : berlinTodayISO();
      const closingConflict = findScopeClosingDayConflict(
        template,
        body,
        scopeFrom,
      );
      if (closingConflict) {
        setScopeClosingDayWarning({
          conflict: closingConflict,
          scope: typedScope,
          roomId: pending.roomId,
          template,
          periodEnd,
          groupId,
        });
        return;
      }

      await continuePreparedSeriesEdit({
        typedScope,
        roomId: pending.roomId,
        template,
        periodEnd,
        groupId,
      });
    } catch (err) {
      handleScopeError(scope, err);
    } finally {
      setSubmitting(false);
    }
  };

  const confirmScopeClosingDay = async () => {
    if (submitting) return;
    const warning = scopeClosingDayWarning;
    if (!warning) return;
    setScopeClosingDayWarning(null);
    setSubmitting(true);
    try {
      await continuePreparedSeriesEdit({
        typedScope: warning.scope,
        roomId: warning.roomId,
        template: warning.template,
        periodEnd: warning.periodEnd,
        groupId: warning.groupId,
      });
    } catch (err) {
      handleScopeError(warning.scope, err);
    } finally {
      setSubmitting(false);
    }
  };

  const confirmLostEdits = async () => {
    if (submitting) return;
    const pending = lostEdits;
    if (!pending) return;
    setSubmitting(true);
    try {
      await pending.onConfirm();
      setLostEdits(null);
      onClose();
    } catch (err) {
      handleScopeError(pending.scope, err);
    } finally {
      setSubmitting(false);
    }
  };

  const openSeriesDeleteConfirm = () => {
    setDeleteEffectiveDate(berlinTodayISO());
    setDeleteError(null);
    setDeleteConfirmOpen(true);
  };

  const handleConfirmSeriesDelete = async () => {
    if (!initialSeries || !onDeleteSeries || deletingSeries) return;
    const minDate = berlinTodayISO();
    if (!deleteEffectiveDate) {
      setDeleteError("Bitte ein Datum auswählen.");
      return;
    }
    if (deleteEffectiveDate < minDate) {
      setDeleteError("Das Datum darf nicht in der Vergangenheit liegen.");
      return;
    }

    setDeletingSeries(true);
    setDeleteError(null);
    try {
      await onDeleteSeries(initialSeries, deleteEffectiveDate);
      setDeleteConfirmOpen(false);
      onClose();
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : "Regeltermin konnte nicht gelöscht werden";
      setDeleteError(msg);
    } finally {
      setDeletingSeries(false);
    }
  };

  // Bulk-add entries for the Kinder field: every distinct grade, class, and
  // group present in the loaded students, each carrying its member ids.
  const studentBulkOptions = useMemo(() => {
    const gradeLevels = new Map<string, string[]>();
    const classes = new Map<string, string[]>();
    const groupNames = new Map<string, string[]>();
    for (const student of students) {
      const schoolClass = student.schoolClass?.trim();
      if (schoolClass) {
        const gradeLevel = getSchoolYear(schoolClass);
        if (gradeLevel) {
          gradeLevels.set(gradeLevel, [
            ...(gradeLevels.get(gradeLevel) ?? []),
            student.id,
          ]);
        }
        classes.set(schoolClass, [
          ...(classes.get(schoolClass) ?? []),
          student.id,
        ]);
      }
      const groupName = student.groupName?.trim();
      if (groupName) {
        groupNames.set(groupName, [
          ...(groupNames.get(groupName) ?? []),
          student.id,
        ]);
      }
    }
    const byName = (a: [string, string[]], b: [string, string[]]) =>
      a[0].localeCompare(b[0], "de");
    return [
      ...[...gradeLevels.entries()].sort(byName).map(([name, memberIds]) => ({
        key: `grade:${name}`,
        label: `Jahrgang ${name}`,
        memberIds,
      })),
      ...[...classes.entries()].sort(byName).map(([name, memberIds]) => ({
        key: `class:${name}`,
        label: schoolClassLabel(name),
        memberIds,
      })),
      ...[...groupNames.entries()].sort(byName).map(([name, memberIds]) => ({
        key: `group:${name}`,
        label: `Gruppe ${name}`,
        memberIds,
      })),
    ];
  }, [students]);

  // Distinct school-class names for the Zielgruppe "Klasse" value picker
  // (issue #1838) — reuses the already-fetched students, no new fetch.
  const targetClassOptions = useMemo(() => {
    const options = new Set(
      students
        .map((student) => student.schoolClass?.trim())
        .filter((item): item is string => Boolean(item)),
    );
    const currentClass = form.targetSchoolClass.trim();
    if (currentClass !== "") options.add(currentClass);
    return [...options].sort((a, b) => a.localeCompare(b, "de"));
  }, [form.targetSchoolClass, students]);
  const targetClassDescriptionIDs = [
    fieldErrors.targetSchoolClass ? "event_target_school_class_error" : null,
    loadingStudents || studentLoadError
      ? "event_target_school_class_availability"
      : null,
  ]
    .filter((id): id is string => id !== null)
    .join(" ");

  const targetCohort = useMemo(() => {
    let label: string | null = null;
    let members: PersonOption[] = [];
    if (form.targetGroupType === "jahrgang" && form.targetGradeLevel) {
      label = `Jahrgang ${form.targetGradeLevel}`;
      members = students.filter(
        (student) =>
          getSchoolYear(student.schoolClass?.trim() ?? "") ===
          form.targetGradeLevel,
      );
    } else if (form.targetGroupType === "klasse" && form.targetSchoolClass) {
      label = schoolClassLabel(form.targetSchoolClass);
      members = students.filter(
        (student) => student.schoolClass?.trim() === form.targetSchoolClass,
      );
    } else if (form.targetGroupType === "gruppe" && form.educationGroupId) {
      const groupName = groups.find(
        (group) => group.id === form.educationGroupId,
      )?.name;
      if (groupName) {
        label = `Gruppe ${groupName}`;
        members = students.filter(
          (student) => student.groupId === form.educationGroupId,
        );
      }
    }
    return { label, memberIds: members.map((student) => student.id) };
  }, [
    form.educationGroupId,
    form.targetGradeLevel,
    form.targetGroupType,
    form.targetSchoolClass,
    groups,
    students,
  ]);
  const missingTargetCohortCount = useMemo(() => {
    const selected = new Set(form.studentIds);
    return targetCohort.memberIds.filter((id) => !selected.has(id)).length;
  }, [form.studentIds, targetCohort.memberIds]);
  const targetCohortButtonLabel = targetCohort.label
    ? targetCohortActionLabel(
        targetCohort.label,
        targetCohort.memberIds.length,
        missingTargetCohortCount,
      )
    : "";

  const addTargetCohort = () => {
    if (targetCohort.memberIds.length === 0) return;
    setForm((current) => ({
      ...current,
      studentIds: Array.from(
        new Set([...current.studentIds, ...targetCohort.memberIds]),
      ),
    }));
  };

  const retryStudentLoad = async () => {
    const studentSeq = ++studentLoadSeq.current;
    const isCurrentStudentLoad = () => studentLoadSeq.current === studentSeq;
    setLoadingStudents(true);
    setStudentLoadError(null);
    try {
      const studentData = await fetchAllStudentOptions();
      if (!isCurrentStudentLoad()) return;
      setStudents(sortPeople(studentData));
      setValidationError(null);
    } catch (err) {
      logger.error("students_retry_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (!isCurrentStudentLoad()) return;
      setStudents([]);
      setStudentLoadError(STUDENT_LOAD_ERROR);
    } finally {
      if (isCurrentStudentLoad()) setLoadingStudents(false);
    }
  };

  const retryStaffLoad = async () => {
    const staffSeq = ++staffLoadSeq.current;
    const isCurrentStaffLoad = () => staffLoadSeq.current === staffSeq;
    setLoadingStaff(true);
    setStaffLoadError(null);
    try {
      const staffData = await staffService.getAllStaff();
      if (!isCurrentStaffLoad()) return;
      setStaff(
        sortPeople(staffData.map((item) => ({ id: item.id, name: item.name }))),
      );
      setValidationError(null);
    } catch (err) {
      logger.error("staff_retry_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (!isCurrentStaffLoad()) return;
      setStaff([]);
      setStaffLoadError(STAFF_LOAD_ERROR);
    } finally {
      if (isCurrentStaffLoad()) setLoadingStaff(false);
    }
  };

  // Datum and Tagesnotiz only apply to the single-instance scope — series-wide
  // scopes write the template, which carries the Wochennotiz instead of the
  // per-occurrence Tagesnotiz.
  const dateChanged =
    initialInstance !== null && form.date !== initialInstance.date;
  const notesChanged =
    initialInstance !== null && form.notes !== (initialInstance.notes ?? "");

  const title = isEditingSeries
    ? "Regeltermin bearbeiten"
    : isEditingInstance
      ? "Termin bearbeiten"
      : isConverting
        ? "Termin wiederholen"
        : "Termin";

  /**
   * Re-reads the category list after the Kategorie-verwalten dialog wrote
   * something (#2131). When a category was just created, it is selected right
   * away — the user opened the dialog because the one they needed was missing.
   */
  const refreshCategories = useCallback(async (selectId?: string) => {
    try {
      const data = await fetchPlannerActivityCategories();
      setCategories(
        [...data].sort((a, b) => a.name.localeCompare(b.name, "de")),
      );
      if (selectId) {
        setForm((prev) => ({ ...prev, categoryId: selectId }));
      }
    } catch (err: unknown) {
      logger.error("categories_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }, []);

  return {
    form,
    update,
    updateRepeat,
    toggleWeekday,
    changeTargetGroupType,
    fieldErrors,
    validationError,
    rooms,
    categories,
    refreshCategories,
    groups,
    students,
    staff,
    loadingRefs,
    loadingStudents,
    studentLoadError,
    loadingStaff,
    staffLoadError,
    retryStudentLoad,
    retryStaffLoad,
    submitting,
    handleSubmit,
    validateForm,
    lastValidationErrors,
    deleteConfirmOpen,
    setDeleteConfirmOpen,
    deleteEffectiveDate,
    setDeleteEffectiveDate,
    deleteError,
    setDeleteError,
    deletingSeries,
    openSeriesDeleteConfirm,
    handleConfirmSeriesDelete,
    expanded,
    choiceDialogOpen,
    setPendingSeriesEdit,
    handleScopeSelect,
    scopeClosingDayWarning,
    setScopeClosingDayWarning,
    confirmScopeClosingDay,
    lostEdits,
    setLostEdits,
    confirmLostEdits,
    conflictWarnings,
    coverageWarnings,
    coverageWarningCount,
    coverageCheckError,
    isEditingInstance,
    isEditingSeries,
    isSeriesFlow,
    canDeleteSeries,
    gradeLevelMax,
    targetGradeOptions,
    preservesGradeAboveTenantCap,
    biweeklyUnavailable,
    abWeekHint,
    studentBulkOptions,
    targetClassOptions,
    targetClassDescriptionIDs,
    targetCohort,
    missingTargetCohortCount,
    targetCohortButtonLabel,
    addTargetCohort,
    dateWeekday,
    dateWeekdayName,
    quickPreset,
    handleQuickPresetChange,
    dateChanged,
    notesChanged,
    title,
    requiredStaffTouched,
    staffRosterTouched,
    listKindTouched,
    manualWeekPattern,
  };
}
