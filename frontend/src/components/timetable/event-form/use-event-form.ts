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
import {
  planningTrackService,
  type PlanningTrack,
} from "~/lib/planning-track-api";
import { staffService } from "~/lib/staff-api";
import { timetableSeriesErrorMessage } from "./scope-error-message";
import { getSchoolYear } from "~/lib/student-helpers";
import { useTenant } from "~/lib/tenant-context";
import { timetableService } from "~/lib/timetable-api";
import {
  chunkDateRange,
  getGermanWeekdayLong,
  latestISODate,
  materializedRecurrenceDates,
  offeringPhaseStartWarning,
  resolveTemplateCalendarPeriodId,
  weekdayDatesInRange,
} from "~/lib/timetable-helpers";
import { useDebounce } from "~/lib/use-debounce";
import {
  changePerWeekdayRosterMode,
  changeRosterWeekdays,
  emptyForm,
  fetchAllStudentOptions,
  formFromInstance,
  formFromSeries,
  hasPerWeekdayStaffDeviation,
  initialPrimaryStaffID,
  initialStaffIDs,
  initialStudentIDs,
  isoWeekday,
  parseRequiredStaffOverride,
  rosterForWeekday,
  rosterSeedForWeekday,
  schoolClassLabel,
  seedWeekdayRosters,
  sortPeople,
  targetCohortActionLabel,
} from "./form-model";
import type {
  EventFormState,
  PersonOption,
  RepeatMode,
  WeekdayRosterState,
} from "./form-model";
import type {
  ConflictWarningItem,
  CreateInstanceBody,
  CreateTemplateBody,
  EditedInWindowResult,
  EnrichedInstance,
  CombinedOfferingCounts,
  OfferingSourceOption,
  ShiftCoverageCheckParams,
  ShiftCoverageWarningItem,
  TargetGroupType,
  TimetableTemplate,
  UpdateTemplateBody,
  WeekdayAssignmentBody,
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

function sameIDSelection(left: string[], right: string[]): boolean {
  if (left.length !== right.length) return false;
  const rightIDs = new Set(right);
  return left.every((id) => rightIDs.has(id));
}

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
 * Keeps a category selection only while the refreshed picker still offers it.
 * A newly created category is authoritative even if the following refetch is
 * stale; an archived category is cleared so saving cannot submit its old ID.
 */
export function reconcileCategoryId(
  currentId: string,
  categories: readonly Pick<ActivityCategory, "id">[],
  createdId?: string,
): string {
  if (createdId) return createdId;
  if (currentId && !categories.some((category) => category.id === currentId)) {
    return "";
  }
  return currentId;
}

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

type SeriesEditScope = "single" | "following" | "all";

const isSeriesEditScope = (value: string): value is SeriesEditScope =>
  value === "single" || value === "following" || value === "all";

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
  const [planningTracks, setPlanningTracks] = useState<PlanningTrack[]>([]);
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
  const [selectedInstanceScope, setSelectedInstanceScope] =
    useState<SeriesEditScope | null>(null);
  const [scopedSeries, setScopedSeries] = useState<TimetableTemplate | null>(
    null,
  );
  const [scopeClosingDayWarning, setScopeClosingDayWarning] = useState<{
    conflict: ClosingDayConflict;
    scope: "all" | "following";
    roomId: number;
    template: TimetableTemplate;
    periodEnd: string | undefined;
    // The RESOLVED series template id (#2187): after a split chain
    // resolution this is the living successor, not the clicked occurrence's
    // own activityGroupId.
    seriesTemplateId: string;
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
  const categoryLoadSeq = useRef(0);
  const studentLoadSeq = useRef(0);
  const staffLoadSeq = useRef(0);
  const invalidateReferenceLoads = useCallback(() => {
    referenceLoadSeq.current++;
    categoryLoadSeq.current++;
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
  // Loading a materialized occurrence's child snapshot must not turn that
  // snapshot into a series edit. Track an actual picker change separately so
  // all/following writes preserve the fetched template roster until then.
  const studentRosterTouched = useRef(false);
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

  const effectiveSeries = initialSeries ?? scopedSeries;
  const isEditingInstance = initialInstance !== null;
  const isEditingSeries = effectiveSeries !== null;
  const isConverting = convertInstance !== null;
  const isSeriesFlow = form.repeat !== "none" || isEditingSeries;
  const scopeSelectionRequired = Boolean(
    initialInstance?.activityGroupId &&
    !initialSeries &&
    selectedInstanceScope === null,
  );
  const choiceDialogOpen = scopeSelectionRequired || pendingSeriesEdit !== null;
  const canDeleteSeries = initialSeries !== null && onDeleteSeries;
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
    for (const selected of form.targetGradeLevels) {
      if (options.some((option) => option.value === selected)) continue;
      const gradeLevel = Number(selected);
      const supported =
        Number.isInteger(gradeLevel) &&
        gradeLevel >= 1 &&
        gradeLevel <= MAX_SUPPORTED_TARGET_GRADE_LEVEL;
      options.push({
        value: selected,
        label: `Jahrgang ${selected} (${supported ? "bestehend" : "ungültig"})`,
        disabled: !supported,
      });
    }
    return options;
  }, [form.targetGradeLevels, gradeLevelMax]);
  const initialGradeTargets = new Set(
    (effectiveSeries?.targets ?? []).flatMap((target) =>
      target.gradeLevel === undefined ? [] : [String(target.gradeLevel)],
    ),
  );
  if (effectiveSeries?.targetGradeLevel !== undefined) {
    initialGradeTargets.add(String(effectiveSeries.targetGradeLevel));
  }
  const preservesGradeAboveTenantCap =
    gradeLevelMax !== undefined &&
    form.targetGradeLevels.some((grade) => Number(grade) > gradeLevelMax);
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
    const categorySeq = ++categoryLoadSeq.current;
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
    studentRosterTouched.current = false;
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
    setSelectedInstanceScope(null);
    setScopedSeries(null);
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
      planningTrackService.list().catch((err: unknown) => {
        logger.error("planning_tracks_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return [] as PlanningTrack[];
      }),
    ])
      .then(([roomData, categoryData, groupData, planningTrackData]) => {
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
          setGroups(sortedGroups);
          setPlanningTracks(planningTrackData);
          if (categoryLoadSeq.current === categorySeq) {
            setCategories(sortedCategories);
            setForm((prev) =>
              prev.categoryId || sortedCategories.length === 0
                ? prev
                : { ...prev, categoryId: sortedCategories[0]?.id ?? "" },
            );
          }
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
  // The conflict probe asks about ONE date, so it must use the people who are
  // actually there on that date's weekday (#2129) — otherwise a series that
  // staffs Tuesday differently reports Monday's staff as double-booked.
  const probeRoster = form.perWeekdayRoster
    ? rosterForWeekday(form, isoWeekday(form.date))
    : { staffIds: staffIDsForSave, studentIds: studentIDsForSave };
  const probeKey = JSON.stringify({
    date: form.date,
    startTime: form.startTime,
    endTime: form.endTime,
    roomId: form.roomId,
    staffIds: probeRoster.staffIds,
    studentIds: probeRoster.studentIds,
  });
  const debouncedProbeKey = useDebounce(probeKey, 500);
  // The convert flow edits an existing instance too — its own slot must not
  // self-conflict, exactly like the regular instance edit.
  const excludeInstanceId = initialInstance?.id ?? convertInstance?.id;

  // One probe per distinct staff list (#2129). A series that staffs Monday
  // with Anna and Tuesday with Bea must be checked with Monday's dates against
  // Anna and Tuesday's against Bea — probing the union would report Bea as
  // missing from every Monday shift she was never assigned to.
  const coverageProbes = useMemo((): ShiftCoverageCheckParams[] => {
    if (!form.startTime || !form.endTime) return [];
    if (!isSeriesFlow) {
      if (staffIDsForSave.length === 0) return [];
      return [
        {
          dates: form.date ? [form.date] : [],
          startTime: form.startTime,
          endTime: form.endTime,
          staffIds: staffIDsForSave,
          excludeInstanceId: initialInstance?.id,
        },
      ];
    }

    const period = calendarPeriods.find(
      (candidate) => candidate.id === form.calendarPeriodId,
    );
    if (!period) return [];
    const today = berlinTodayISO();
    // #2135: a series starts at its start date, not the period start. For a
    // new series that is the picked Datum; a stored series keeps the
    // schedules' validFrom. Never probe dates the materializer skips.
    const from = effectiveSeries
      ? latestISODate(
          period.startDate,
          today,
          effectiveSeries.schedules[0]?.validFrom ?? "",
        )
      : latestISODate(period.startDate, form.date || "");
    const shared = {
      startTime: form.startTime,
      endTime: form.endTime,
      excludeInstanceId: convertInstance?.id,
      concreteInstanceDate: convertInstance ? form.date : undefined,
      replanActivityGroupId: effectiveSeries?.id,
      calendarPeriodId: period.id,
      weekPattern: form.weekPattern,
    };

    const usePerWeekday = form.perWeekdayRoster && form.weekdays.length >= 2;
    const weekdayGroups = usePerWeekday
      ? form.weekdays.map((weekday) => ({
          weekdays: [weekday],
          staffIds: rosterForWeekday(form, weekday).staffIds,
        }))
      : [{ weekdays: form.weekdays, staffIds: staffIDsForSave }];

    const probes: ShiftCoverageCheckParams[] = [];
    for (const group of weekdayGroups) {
      if (group.staffIds.length === 0) continue;
      const dates = weekdayDatesInRange(from, period.endDate, group.weekdays);
      if (
        convertInstance &&
        form.date &&
        (!usePerWeekday || group.weekdays.includes(isoWeekday(form.date))) &&
        !dates.includes(form.date)
      ) {
        dates.push(form.date);
        dates.sort((left, right) => left.localeCompare(right));
      }
      if (dates.length === 0) continue;
      probes.push({ ...shared, dates, staffIds: group.staffIds });
    }
    return probes;
  }, [
    calendarPeriods,
    convertInstance,
    effectiveSeries,
    form,
    initialInstance?.id,
    isSeriesFlow,
    staffIDsForSave,
  ]);
  const coverageProbeKey = JSON.stringify(coverageProbes);
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
    const probes = JSON.parse(
      debouncedCoverageProbeKey,
    ) as ShiftCoverageCheckParams[];
    if (probes.length === 0) {
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
    Promise.all(
      probes.map((probe) =>
        checkShiftCoverageWithSignal(probe, controller.signal),
      ),
    )
      .then((results) => {
        if (coverageProbeSeq.current !== seq) return;
        setCoverageWarnings(results.flatMap((r) => r.coverageWarnings));
        setCoverageWarningCount(
          results.reduce(
            (total, r) =>
              total + (r.coverageWarningCount ?? r.coverageWarnings.length),
            0,
          ),
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
    probeOrProbes: ShiftCoverageCheckParams | ShiftCoverageCheckParams[] | null,
  ): Promise<void> => {
    const probes = (
      Array.isArray(probeOrProbes) ? probeOrProbes : [probeOrProbes]
    ).filter((probe): probe is ShiftCoverageCheckParams =>
      Boolean(probe && probe.dates.length > 0),
    );
    if (!canCheckShiftCoverage || probes.length === 0) return;
    const controller = new AbortController();
    const timeoutID = window.setTimeout(
      () => controller.abort(),
      COVERAGE_CHECK_TIMEOUT_MS,
    );
    try {
      const results = await Promise.all(
        probes.map((probe) =>
          checkShiftCoverageWithSignal(probe, controller.signal),
        ),
      );
      const result = {
        coverageWarnings: results.flatMap((item) => item.coverageWarnings),
        coverageWarningCount: results.reduce(
          (total, item) =>
            total + (item.coverageWarningCount ?? item.coverageWarnings.length),
          0,
        ),
      };
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
    if (key === "studentIds") studentRosterTouched.current = true;
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

  /**
   * Period selection for the series wizard. #2135 made the picked Datum the
   * series start, but schools pre-plan the next school year while the
   * wizard's Datum still defaults to today — hard-blocking on the wizard's
   * own default would be hostile. So when the chosen period does not contain
   * the current Datum, move it to the first materializing occurrence inside
   * the period (WYSIWYG: the field keeps showing the real series start). A Datum
   * the user afterwards moves out of the period still fails validation.
   * Series edits keep their stored start; the convert seed date is the
   * instance's own date and must never be moved implicitly.
   */
  const selectCalendarPeriod = (nextId: string) => {
    update("calendarPeriodId", nextId);
    if (isEditingSeries || convertInstance) return;
    const period = calendarPeriods.find((candidate) => candidate.id === nextId);
    if (
      !period ||
      !form.date ||
      (form.date >= period.startDate && form.date <= period.endDate)
    ) {
      return;
    }
    // The A/B predicate must apply too: with "Alle 2 Wochen" the first
    // selected weekday can sit in the other week slot, and a start_date the
    // materializer skips would start the series a week after the shown date.
    const firstOccurrence = materializedRecurrenceDates({
      period,
      fromISO: period.startDate,
      weekdays: form.weekdays,
      weekPattern: form.weekPattern,
    })[0];
    update("date", firstOccurrence ?? period.startDate);
  };

  const toggleWeekday = (iso: number) => {
    setForm((prev) => {
      const has = prev.weekdays.includes(iso);
      const next = has
        ? prev.weekdays.filter((day) => day !== iso)
        : [...prev.weekdays, iso].sort((a, b) => a - b);
      return changeRosterWeekdays(prev, next, iso);
    });
    setValidationError(null);
    clearFieldError("weekdays");
  };

  // --- Per-weekday rosters (#2129) -----------------------------------------

  const [activeRosterWeekday, setActiveRosterWeekday] = useState<number>(
    () => form.weekdays[0] ?? 1,
  );

  // Keep the selected weekday tab on a weekday the series actually runs on:
  // dropping Tuesday from the recurrence must not leave the roster editor
  // pointing at it.
  useEffect(() => {
    if (form.weekdays.length === 0) return;
    if (!form.weekdays.includes(activeRosterWeekday)) {
      setActiveRosterWeekday(form.weekdays[0]!);
    }
  }, [form.weekdays, activeRosterWeekday]);

  const setPerWeekdayRoster = (enabled: boolean) => {
    staffRosterTouched.current = true;
    studentRosterTouched.current = true;
    setForm((prev) =>
      changePerWeekdayRosterMode(prev, enabled, activeRosterWeekday),
    );
    setValidationError(null);
  };

  const setWeekdayRoster = (weekday: number, roster: WeekdayRosterState) => {
    const previous = rosterForWeekday(form, weekday);
    if (
      !sameIDSelection(previous.staffIds, roster.staffIds) ||
      previous.primaryStaffId !== roster.primaryStaffId
    ) {
      staffRosterTouched.current = true;
    }
    if (!sameIDSelection(previous.studentIds, roster.studentIds)) {
      studentRosterTouched.current = true;
    }
    setForm((prev) => ({
      ...prev,
      weekdayRosters: { ...prev.weekdayRosters, [weekday]: roster },
    }));
    setValidationError(null);
  };

  const applyActiveWeekdayRosterToAll = () => {
    staffRosterTouched.current = true;
    studentRosterTouched.current = true;
    setForm((prev) => {
      const source = rosterForWeekday(prev, activeRosterWeekday);
      const next: Record<number, WeekdayRosterState> = {};
      for (const weekday of prev.weekdays) {
        next[weekday] =
          weekday === activeRosterWeekday
            ? {
                staffIds: [...source.staffIds],
                primaryStaffId: source.primaryStaffId,
                studentIds: [...source.studentIds],
              }
            : rosterSeedForWeekday(prev, source, weekday);
      }
      return { ...prev, weekdayRosters: next };
    });
    setValidationError(null);
  };

  /**
   * The `weekday_assignments` payload, or undefined when the series shares one
   * roster. Sending it for every weekday (not only the deviating ones) keeps
   * the wire shape self-describing: a reader does not have to merge it with
   * the shared lists to know who is there on a given day.
   */
  const weekdayAssignmentsBody = (): WeekdayAssignmentBody[] | undefined => {
    // An offering-sourced roster is server-managed; the backend rejects
    // weekday_assignments next to a source (#2137 x #2129). The editor hides
    // the section, but a stale perWeekdayRoster flag from before the source
    // was picked must not leak into the payload.
    if (
      form.targetGroupType === "angebot" &&
      form.sourceCareOfferingIds.length > 0
    ) {
      return undefined;
    }
    if (!form.perWeekdayRoster || form.weekdays.length < 2) return undefined;
    return [...form.weekdays]
      .sort((a, b) => a - b)
      .map((weekday) => {
        const roster = rosterForWeekday(form, weekday);
        const primary =
          roster.primaryStaffId &&
          roster.staffIds.includes(roster.primaryStaffId)
            ? Number(roster.primaryStaffId)
            : undefined;
        return {
          weekday,
          staff_ids: roster.staffIds.map(Number),
          student_ids: roster.studentIds.map(Number),
          primary_staff_id: primary,
        };
      });
  };

  const changeTargetGroupType = (nextType: TargetGroupType) => {
    // Read outside the updater so it stays pure (mirrors applySourceOfferingIds).
    const restoredStudentIds = preSourceStudentIdsRef.current;
    setForm((current) => ({
      ...current,
      targetGroupType: nextType,
      targetGradeLevel: nextType === "jahrgang" ? current.targetGradeLevel : "",
      targetSchoolClass: nextType === "klasse" ? current.targetSchoolClass : "",
      targetGradeLevels:
        nextType === "jahrgang" ? current.targetGradeLevels : [],
      targetSchoolClasses:
        nextType === "klasse" ? current.targetSchoolClasses : [],
      educationGroupId: nextType === "gruppe" ? current.educationGroupId : "",
      educationGroupIds: nextType === "gruppe" ? current.educationGroupIds : [],
      sourceCareOfferingIds:
        nextType === "angebot" ? current.sourceCareOfferingIds : [],
      sourceGradeLevels:
        nextType === "angebot" ? current.sourceGradeLevels : [],
      // Leaving "angebot" clears the source above, so the manual roster
      // stashed when the source was picked must come back with it, exactly
      // like clearing the source in place. Keeping the emptied list would
      // make the next save wipe the participants (#2147 review round 13).
      studentIds:
        nextType !== "angebot" && current.sourceCareOfferingIds.length > 0
          ? restoredStudentIds
          : current.studentIds,
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
      // #2135: the picked Datum is the series start for new series; it must
      // lie inside the selected period. Series edits keep their stored start.
      if (!isEditingSeries && form.date !== "") {
        const period = calendarPeriods.find(
          (candidate) => candidate.id === form.calendarPeriodId,
        );
        if (
          period &&
          (form.date < period.startDate || form.date > period.endDate)
        ) {
          errors.date = "Das Datum muss im gewählten Planungszeitraum liegen.";
        }
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
        const invalidGrade = form.targetGradeLevels.some((value) => {
          const gradeLevel = Number(value);
          return (
            !Number.isInteger(gradeLevel) ||
            gradeLevel < 1 ||
            gradeLevel > MAX_SUPPORTED_TARGET_GRADE_LEVEL ||
            gradeLevelMax === undefined ||
            (gradeLevel > gradeLevelMax && !initialGradeTargets.has(value))
          );
        });
        if (form.targetGradeLevels.length === 0) {
          errors.targetGradeLevel = "Bitte einen Jahrgang auswählen.";
        } else if (invalidGrade) {
          errors.targetGradeLevel = "Bitte einen gültigen Jahrgang auswählen.";
        }
      }
      if (
        form.targetGroupType === "klasse" &&
        form.targetSchoolClasses.length === 0
      ) {
        errors.targetSchoolClass = "Bitte eine Klasse auswählen.";
      }
      if (
        form.targetGroupType === "gruppe" &&
        form.educationGroupIds.length === 0
      ) {
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
        setForm((prev) =>
          changeRosterWeekdays(
            prev,
            dateWeekday >= 1 && dateWeekday <= 5 ? [dateWeekday] : [1],
            dateWeekday,
          ),
        );
        clearFieldError("weekdays");
        break;
      case "jeden-wochentag":
        updateRepeat("weekly");
        setForm((prev) =>
          changeRosterWeekdays(prev, [...WEEKDAYS], dateWeekday),
        );
        clearFieldError("weekdays");
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
    planning_track_id: form.planningTrackId
      ? Number(form.planningTrackId)
      : null,
    notes: form.seriesNotes.trim() || undefined,
    education_group_id:
      form.targetGroupType === "gruppe"
        ? form.educationGroupIds.length > 0
          ? Number(form.educationGroupIds[0])
          : form.educationGroupId
            ? Number(form.educationGroupId)
            : undefined
        : form.educationGroupId
          ? Number(form.educationGroupId)
          : undefined,
    target_group_type: form.targetGroupType,
    target_grade_level:
      form.targetGroupType === "jahrgang" && form.targetGradeLevels.length > 0
        ? Number(form.targetGradeLevels[0])
        : undefined,
    target_school_class:
      form.targetGroupType === "klasse" && form.targetSchoolClasses.length > 0
        ? form.targetSchoolClasses[0]?.trim()
        : undefined,
    // Explicit null, never undefined: the update endpoint is presence-aware
    // (#2147 review round 12) — an omitted field keeps the stored source, so
    // clearing it in the editor MUST send null to be honored. Create treats
    // null and omitted alike.
    source_care_offering_ids:
      form.targetGroupType === "angebot" &&
      form.sourceCareOfferingIds.length > 0
        ? form.sourceCareOfferingIds.map(Number)
        : null,
    source_grade_levels:
      form.targetGroupType === "angebot" &&
      form.sourceCareOfferingIds.length > 0 &&
      form.sourceGradeLevels.length > 0
        ? [...form.sourceGradeLevels].sort((a, b) => a - b)
        : null,
    targets:
      form.targetGroupType === "jahrgang"
        ? form.targetGradeLevels.map((value) => ({
            type: "jahrgang" as const,
            grade_level: Number(value),
          }))
        : form.targetGroupType === "klasse"
          ? form.targetSchoolClasses.map((value) => ({
              type: "klasse" as const,
              school_class: value.trim(),
            }))
          : form.targetGroupType === "gruppe"
            ? form.educationGroupIds.map((value) => ({
                type: "gruppe" as const,
                education_group_id: Number(value),
              }))
            : [],
    calendar_period_id: Number(form.calendarPeriodId),
    week_pattern: form.weekPattern,
    required_staff: parseRequiredStaffOverride(form.requiredStaff),
    // A sourced roster is server-managed — the backend rejects student_ids
    // next to a source.
    student_ids:
      form.targetGroupType === "angebot" &&
      form.sourceCareOfferingIds.length > 0
        ? []
        : studentIDsForSave.map(Number),
    staff_ids: staffIDsForSave.map(Number),
    primary_staff_id: primaryStaffIDForSave
      ? Number(primaryStaffIDForSave)
      : undefined,
    weekday_assignments: weekdayAssignmentsBody(),
  });

  const findPeriod = (id?: string): CalendarPeriod | undefined =>
    id ? calendarPeriods.find((period) => period.id === id) : undefined;

  /**
   * Applies what the user changed in THIS session (next vs. before) on top of
   * a base roster (#2187). Used when the fetched template is the split
   * chain's living successor rather than the clicked occurrence's own
   * template: the form's list describes the PREDECESSOR's occurrence, so
   * writing it wholesale would overwrite the successor's roster and promote
   * per-occurrence overrides into the permanent series. Keep = base minus the
   * ids the user removed; plus the ids the user added.
   */
  const applyRosterDelta = (
    base: readonly string[],
    next: readonly string[],
    before: readonly string[],
  ): string[] => {
    const removed = new Set(before.filter((id) => !next.includes(id)));
    const kept = base.filter((id) => !removed.has(id));
    const added = next.filter(
      (id) => !before.includes(id) && !kept.includes(id),
    );
    return [...kept, ...added];
  };

  /**
   * The ids whose membership actually changed in this session (#2187). This
   * is what the backend needs to reconcile the split chain's capped
   * predecessors: the submitted roster describes the LIVING segment, whose
   * membership may legitimately differ from a predecessor's, so it is not the
   * predecessor's target set — reconciling against it would drop children who
   * only ever stood on the predecessor.
   */
  const changedRosterIDs = (
    next: readonly string[],
    before: readonly string[],
  ): number[] => {
    const nextSet = new Set(next);
    const beforeSet = new Set(before);
    const changed = new Set<string>();
    for (const id of next) if (!beforeSet.has(id)) changed.add(id);
    for (const id of before) if (!nextSet.has(id)) changed.add(id);
    return [...changed]
      .map(Number)
      .filter((id) => Number.isFinite(id) && id > 0);
  };

  /**
   * Whether the user changed the Hauptbetreuung in this session. The staff
   * roster is marked "touched" by any membership edit (#2187 review), so on
   * the chain path this narrower predicate decides whether the form's value
   * may overwrite the successor's own choice.
   */
  const primaryStaffTouched = () =>
    form.primaryStaffId !== initialPrimaryStaffIDSnapshot;

  /**
   * Staff scope for the chain reconciliation: membership changes plus both
   * sides of a changed Hauptbetreuung, whose is_primary flag has to be
   * rewritten on the predecessor rows as well.
   */
  const changedStaffIDs = (): number[] => {
    const changed = changedRosterIDs(form.staffIds, initialStaffIDsSnapshot);
    if (primaryStaffTouched()) {
      for (const id of [form.primaryStaffId, initialPrimaryStaffIDSnapshot]) {
        const numeric = Number(id);
        if (
          id &&
          Number.isFinite(numeric) &&
          numeric > 0 &&
          !changed.includes(numeric)
        ) {
          changed.push(numeric);
        }
      }
    }
    return changed;
  };

  /**
   * Scalar fields to echo back from the fetched template on the chain path
   * (#2187). The form was seeded from a PREDECESSOR occurrence while the PUT
   * targets the living successor, which the split may have deliberately given
   * a different time, room, name or Planungsschiene. Only fields the user
   * actually edited may overwrite the successor's values; the rest are sent
   * back unchanged so a roster save cannot silently revert that future change.
   * (Predecessor windows keep their own values either way — non-roster fields
   * never retro-apply.)
   */
  const chainPreservedScalars = (
    template: TimetableTemplate,
  ): { preserved: Partial<UpdateTemplateBody>; edited: boolean } => {
    if (!initialInstance) return { preserved: {}, edited: false };
    const schedule = template.schedules[0];
    const preserved: Partial<UpdateTemplateBody> = {};
    let edited = false;
    const keep = <K extends keyof UpdateTemplateBody>(
      untouched: boolean,
      key: K,
      value: UpdateTemplateBody[K],
    ) => {
      if (untouched) {
        preserved[key] = value;
      } else {
        edited = true;
      }
    };
    keep(
      form.title.trim() === initialInstance.title.trim(),
      "name",
      template.name,
    );
    if (schedule) {
      keep(
        form.startTime === initialInstance.startTime,
        "start_time",
        schedule.startTime,
      );
      keep(
        form.endTime === initialInstance.endTime,
        "end_time",
        schedule.endTime,
      );
    }
    if (template.roomId) {
      keep(
        form.roomId === initialInstance.roomId,
        "room_id",
        Number(template.roomId),
      );
    }
    keep(
      (form.planningTrackId || "") === (initialInstance.planningTrackId ?? ""),
      "planning_track_id",
      template.planningTrackId ? Number(template.planningTrackId) : null,
    );
    return { preserved, edited };
  };

  /**
   * Weekday assignments to send from the occurrence-scope editor (#2129).
   *
   * That editor shows ONE roster because it was opened on one occurrence, so
   * it cannot describe the other weekdays. Returning the template's stored
   * assignments preserves them; when the user actually edited the roster, only
   * the edited occurrence's own weekday is replaced. undefined for a series
   * that shares one roster, which keeps the pre-#2129 behaviour untouched.
   */
  const preservedWeekdayAssignments = (
    template: TimetableTemplate,
    staffIDs: string[],
    studentIDs: string[],
    primaryStaffId: string | undefined,
  ): WeekdayAssignmentBody[] | undefined => {
    if (template.weekdayAssignments.length === 0) return undefined;
    const editedWeekday = initialInstance
      ? isoWeekday(initialInstance.date)
      : undefined;
    const staffWasEdited = staffRosterTouched.current;
    const studentsWereEdited = studentRosterTouched.current;
    // #2187: against a chain-resolved successor the form lists describe the
    // predecessor's occurrence — apply only the user's delta on top of the
    // successor's own weekday roster instead of replacing it.
    const chainResolved = template.resolvedFromTemplateId !== undefined;
    return template.weekdayAssignments.map((assignment) => {
      if (
        (!staffWasEdited && !studentsWereEdited) ||
        assignment.weekday !== editedWeekday
      ) {
        return {
          weekday: assignment.weekday,
          staff_ids: assignment.staffIds.map(Number),
          student_ids: assignment.studentIds.map(Number),
          primary_staff_id: assignment.primaryStaffId
            ? Number(assignment.primaryStaffId)
            : undefined,
        };
      }
      const weekdayStaffIDs = staffWasEdited
        ? chainResolved
          ? applyRosterDelta(
              assignment.staffIds,
              form.staffIds,
              initialStaffIDsSnapshot,
            )
          : staffIDs
        : assignment.staffIds;
      const weekdayStudentIDs = studentsWereEdited
        ? chainResolved
          ? applyRosterDelta(
              assignment.studentIds,
              form.studentIds,
              initialStudentIDsSnapshot,
            )
          : studentIDs
        : assignment.studentIds;
      // #2187 review: same rule as for the shared roster — an untouched
      // Hauptbetreuung must not travel from the predecessor's occurrence onto
      // this weekday of the successor.
      const weekdayPrimary =
        chainResolved && !primaryStaffTouched()
          ? assignment.primaryStaffId
          : primaryStaffId;
      return {
        weekday: assignment.weekday,
        staff_ids: weekdayStaffIDs.map(Number),
        student_ids: weekdayStudentIDs.map(Number),
        primary_staff_id: staffWasEdited
          ? weekdayPrimary && weekdayStaffIDs.includes(weekdayPrimary)
            ? Number(weekdayPrimary)
            : undefined
          : assignment.primaryStaffId
            ? Number(assignment.primaryStaffId)
            : undefined,
      };
    });
  };

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
    // #2187: when the template was chain-resolved to the living successor,
    // the form lists were seeded from the PREDECESSOR's occurrence — apply
    // only the user's delta on top of the successor's roster.
    const chainResolved = template.resolvedFromTemplateId !== undefined;
    const templateStudentIDs =
      studentRosterEditable && studentRosterTouched.current
        ? chainResolved
          ? applyRosterDelta(
              template.studentIds,
              form.studentIds,
              initialStudentIDsSnapshot,
            )
          : form.studentIds
        : template.studentIds;
    const templateStaffIDs =
      staffRosterEditable && staffRosterTouched.current
        ? chainResolved
          ? applyRosterDelta(
              template.staffIds,
              form.staffIds,
              initialStaffIDsSnapshot,
            )
          : form.staffIds
        : template.staffIds;
    // #2187 review: the Hauptbetreuung is a scalar, not a delta — and the form
    // holds the PREDECESSOR occurrence's value. Any staff membership edit sets
    // staffRosterTouched, so without the narrower "did they change the primary"
    // check a chain save would silently revert a Hauptbetreuung the split set
    // deliberately (or clear it, when the old primary is no longer on the
    // successor's roster).
    const primaryStaffId =
      staffRosterEditable && staffRosterTouched.current
        ? chainResolved && !primaryStaffTouched()
          ? template.primaryStaffId
          : form.primaryStaffId || template.primaryStaffId
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
      planning_track_id: form.planningTrackId
        ? Number(form.planningTrackId)
        : null,
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
      // Explicit null mirrors seriesBody: the presence-aware PUT keeps an
      // omitted source, so preserving "no source" needs null (#2147 r12).
      source_care_offering_ids:
        template.sourceCareOfferingIds &&
        template.sourceCareOfferingIds.length > 0
          ? template.sourceCareOfferingIds.map(Number)
          : null,
      source_grade_levels:
        template.sourceCareOfferingIds &&
        template.sourceCareOfferingIds.length > 0 &&
        template.sourceGradeLevels &&
        template.sourceGradeLevels.length > 0
          ? template.sourceGradeLevels
          : null,
      targets: template.targets?.map((target) => ({
        type: target.type,
        grade_level: target.gradeLevel,
        school_class: target.schoolClass,
        education_group_id: target.educationGroupId
          ? Number(target.educationGroupId)
          : undefined,
      })),
      max_participants: template.maxParticipants,
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
      // A sourced template's studentIds ARE the sourced rows; echoing them
      // next to the source would be rejected by the backend.
      student_ids:
        template.sourceCareOfferingIds &&
        template.sourceCareOfferingIds.length > 0
          ? []
          : templateStudentIDs.map(Number),
      staff_ids: templateStaffIDs.map(Number),
      primary_staff_id:
        primaryStaffId && templateStaffIDs.includes(primaryStaffId)
          ? Number(primaryStaffId)
          : undefined,
      // #2129: an occurrence-scope edit must not silently flatten a series
      // that staffs each weekday differently. Carry the template's weekday
      // assignments through, applying an edited roster to the weekday of the
      // occurrence the user opened — the only weekday this form describes.
      weekday_assignments: preservedWeekdayAssignments(
        template,
        templateStaffIDs,
        templateStudentIDs,
        primaryStaffId,
      ),
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
    // #2135: the picked Datum is stamped as the schedules' valid_from, which
    // makes the materializer skip this series before its start on its own.
    // The windows still span the whole period because materialization is
    // tenant-wide — narrowing them would stop backfilling other templates
    // that begin earlier in the period.
    const chunks = period
      ? chunkDateRange(period.startDate, period.endDate, MATERIALIZE_CHUNK_DAYS)
      : [];
    const [firstChunk, ...restChunks] = chunks;
    if (!firstChunk) {
      const created = await timetableService.createTemplate({
        ...body,
        start_date: form.date || undefined,
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
      start_date: form.date || undefined,
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
    // #2135: the converted instance's date is the series start; the
    // schedules' valid_from skips earlier dates for this series, while the
    // tenant-wide window keeps backfilling other templates from the period
    // start.
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

    // US-5 Dreifach-Frage: editing an instance that belongs to a series first
    // asks for the scope instead of writing immediately. Also when the user
    // flips Wiederholung to "Jede Woche" while editing — previously that made
    // isSeriesFlow true, skipped this gate, and created a second series next
    // to the existing one (Franziska / Schule am Berg).
    if (
      isEditingInstance &&
      initialInstance?.activityGroupId &&
      !initialSeries &&
      selectedInstanceScope !== null
    ) {
      if (selectedInstanceScope === "single") {
        setSubmitting(true);
        try {
          await checkCoverageBeforeSave(coverageProbes);
          const saved = await timetableService.update(
            initialInstance.id,
            instanceBody(parsed.roomId, initialInstance.activityGroupId),
          );
          toastSuccess("Termin gespeichert");
          onSaved({ kind: "instance", instance: saved });
          onClose();
        } catch (err) {
          handleScopeError("single", err);
        } finally {
          setSubmitting(false);
        }
        return;
      }
      await handleScopeSelect(selectedInstanceScope, { roomId: parsed.roomId });
      return;
    }

    setSubmitting(true);
    try {
      await checkCoverageBeforeSave(coverageProbes);
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

      // Seed for "turn this occurrence into a series": explicit Wiederholen
      // (convertInstance) or editing a one-off and switching Wiederholung to
      // weekly. Both must UPDATE the existing row, not create a parallel series
      // that leaves the old Termin in place.
      const seriesSeedInstance =
        convertInstance ??
        (isEditingInstance &&
        initialInstance &&
        !initialInstance.activityGroupId
          ? initialInstance
          : null);

      if (seriesSeedInstance) {
        const created = await timetableService.convertInstanceToSeries(
          seriesSeedInstance.id,
          {
            ...seriesBody(parsed.roomId, parsed.categoryId),
            // #2135: the repeated instance's date is the series start.
            start_date: form.date,
            instance_notes: form.notes.trim() || undefined,
          },
        );
        if (await materializePeriodAfterConvert()) {
          toastSuccess(
            convertInstance
              ? "Termin wiederholt"
              : "Termin als Serie gespeichert",
          );
        } else {
          toastWarning(FOLLOW_UP_WARNING);
        }
        onSaved({
          kind: "series",
          seriesId: created.templateId,
          linkedInstanceId: seriesSeedInstance.id,
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
  const seriesCoverageProbes = (
    template: TimetableTemplate,
    body: UpdateTemplateBody,
    fromISO: string,
    replanActivityGroupId?: string,
  ): ShiftCoverageCheckParams[] => {
    const calendarPeriodId =
      resolveTemplateCalendarPeriodId(template) ?? form.calendarPeriodId;
    const period = findPeriod(calendarPeriodId);
    if (
      !period ||
      body.weekdays.length === 0 ||
      !body.start_time ||
      !body.end_time
    ) {
      return [];
    }
    // #2135: never probe before the segment's own series start (validFrom).
    // #2187: a chain-resolved save also covers the predecessor window BEFORE
    // the successor's validFrom — clamping to it would skip exactly the days
    // being edited.
    const from = latestISODate(
      period.startDate,
      fromISO,
      template.resolvedFromTemplateId !== undefined
        ? ""
        : (template.schedules[0]?.validFrom ?? ""),
    );
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
    const sharedStaffIDs = (body.staff_ids ?? []).map(String);
    const assignmentsByWeekday = new Map(
      (body.weekday_assignments ?? []).map((assignment) => [
        assignment.weekday,
        assignment,
      ]),
    );
    const weekdayGroups =
      assignmentsByWeekday.size > 0
        ? body.weekdays.map((weekday) => ({
            weekdays: [weekday],
            staffIds: (
              assignmentsByWeekday.get(weekday)?.staff_ids ??
              body.staff_ids ??
              []
            ).map(String),
          }))
        : [{ weekdays: body.weekdays, staffIds: sharedStaffIDs }];

    const probes: ShiftCoverageCheckParams[] = [];
    for (const group of weekdayGroups) {
      if (group.staffIds.length === 0) continue;
      let dates = weekdayDatesInRange(from, period.endDate, group.weekdays);
      if (latestValidUntil) {
        const boundary = latestValidUntil;
        dates = dates.filter((date) => date < boundary);
      }
      if (dates.length === 0) continue;
      probes.push({
        dates,
        startTime: body.start_time,
        endTime: body.end_time,
        staffIds: group.staffIds,
        replanActivityGroupId,
        calendarPeriodId: period.id,
        weekPattern: body.week_pattern ?? 0,
      });
    }
    return probes;
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
      // #2187: a chain-resolved save also covers the predecessor window
      // before the successor's validFrom.
      validFrom:
        template.resolvedFromTemplateId !== undefined
          ? undefined
          : validity?.validFrom,
      validUntil: validity?.validUntil,
    });
    return findFirstClosingDayConflict(closingDayRanges, dates);
  };

  const handleScopeError = (scope: string, err: unknown) => {
    logger.error("series_scope_save_failed", {
      scope,
      error: err instanceof Error ? err.message : String(err),
    });
    const msg = timetableSeriesErrorMessage(
      err,
      "Termin konnte nicht gespeichert werden",
    );
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
    seriesTemplateId: string,
  ) => {
    if (!initialInstance) return;
    const chainResolved = template.resolvedFromTemplateId !== undefined;
    // #2187: on the chain path the write targets the living successor, whose
    // untouched fields must survive the save — see chainPreservedScalars.
    const chainScalars = chainResolved
      ? chainPreservedScalars(template)
      : { preserved: {}, edited: false };
    const body = scopedSeries
      ? seriesBody(roomId, Number(form.categoryId))
      : chainResolved
        ? {
            ...templateBodyFromForm(template, roomId),
            ...chainScalars.preserved,
          }
        : templateBodyFromForm(template, roomId);
    const scopeProbes = seriesCoverageProbes(
      template,
      body,
      typedScope === "following" ? initialInstance.date : berlinTodayISO(),
      typedScope === "all" ? seriesTemplateId : undefined,
    );
    await checkCoverageBeforeSave(scopeProbes);
    // #2187: the clicked occurrence belonged to a capped predecessor segment
    // and the backend resolved the template to the living successor. That
    // successor cannot be split at the occurrence date (it lies before the
    // successor's valid_from), and it does not need to be: "from D on" IS the
    // whole remaining series. Update it in place; a changed roster addition-
    // ally carries series_roster_from plus the ids that actually changed, so
    // the backend mirrors exactly that change onto the predecessor's
    // remaining occurrences (the clicked one included) and leaves everyone
    // else on those occurrences alone.
    if (chainResolved) {
      const rosterFrom =
        typedScope === "following" ? initialInstance.date : berlinTodayISO();
      const perWeekdaySeries = template.weekdayAssignments.length > 0;
      const studentScope = new Set<number>();
      const staffScope = new Set<number>();
      const scopeWeekdays: number[] = [];
      let primaryChanged = false;

      if (scopedSeries && perWeekdaySeries) {
        for (const assignment of template.weekdayAssignments) {
          const next = form.weekdayRosters[assignment.weekday];
          if (!next) continue;
          const changedStudents = changedRosterIDs(
            next.studentIds,
            assignment.studentIds,
          );
          const changedStaff = changedRosterIDs(
            next.staffIds,
            assignment.staffIds,
          );
          const weekdayPrimaryChanged =
            next.primaryStaffId !== assignment.primaryStaffId;
          if (
            changedStudents.length > 0 ||
            changedStaff.length > 0 ||
            weekdayPrimaryChanged
          ) {
            scopeWeekdays.push(assignment.weekday);
            changedStudents.forEach((id) => studentScope.add(id));
            changedStaff.forEach((id) => staffScope.add(id));
            for (const id of [next.primaryStaffId, assignment.primaryStaffId]) {
              const numeric = Number(id);
              if (id && Number.isFinite(numeric) && numeric > 0) {
                staffScope.add(numeric);
              }
            }
            primaryChanged ||= weekdayPrimaryChanged;
          }
        }
      } else {
        changedRosterIDs(form.studentIds, initialStudentIDsSnapshot).forEach(
          (id) => studentScope.add(id),
        );
        changedStaffIDs().forEach((id) => staffScope.add(id));
        primaryChanged = primaryStaffTouched();
      }
      const studentScopeIDs = [...studentScope];
      const staffScopeIDs = [...staffScope];
      const rosterChanged =
        studentScopeIDs.length > 0 || staffScopeIDs.length > 0;
      await timetableService.updateTemplate(seriesTemplateId, {
        ...body,
        ...(rosterChanged
          ? {
              series_roster_from: rosterFrom,
              ...(studentScopeIDs.length > 0
                ? { series_roster_scope_student_ids: studentScopeIDs }
                : {}),
              ...(staffScopeIDs.length > 0
                ? { series_roster_scope_staff_ids: staffScopeIDs }
                : {}),
              ...(scopeWeekdays.length > 0
                ? { series_roster_scope_weekdays: scopeWeekdays }
                : {}),
              // primary_staff_id names the successor's Hauptbetreuung; it may
              // only reach a predecessor row when the user moved it.
              ...(primaryChanged
                ? { series_roster_primary_changed: true }
                : {}),
            }
          : {}),
      });
      if (await replanTemplateFuture(seriesTemplateId, periodEnd)) {
        // The roster change reaches back to rosterFrom, but title, time, room
        // and Planungsschiene belong to the successor's own window and start
        // where it starts. Naming rosterFrom for all of them would claim more
        // than was written, so an edited scalar says which date it takes
        // effect from (#2187 review) and everything else stays generic.
        const successorFrom = template.schedules[0]?.validFrom;
        toastSuccess(
          chainScalars.edited && successorFrom
            ? `Regeltermin gespeichert. Zeit, Raum und Titel gelten ab ${formatDate(successorFrom)}.`
            : "Regeltermin gespeichert",
        );
      } else {
        toastWarning(FOLLOW_UP_WARNING);
      }
      onSaved({ kind: "series", seriesId: seriesTemplateId });
      return;
    }
    if (typedScope === "following") {
      const effectiveDate = initialInstance.date;
      const chunks = chunkDateRange(
        effectiveDate,
        periodEnd ?? weekTo ?? effectiveDate,
        MATERIALIZE_CHUNK_DAYS,
      );
      const split = await timetableService.splitTemplate(seriesTemplateId, {
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
            seriesTemplateId,
          );
          if (result.warnings.some((w) => w.code === "no_active_period")) {
            break;
          }
        } catch (chunkErr) {
          logger.error("series_replan_chunk_failed", {
            template_id: seriesTemplateId,
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
      await timetableService.updateTemplate(seriesTemplateId, body);
      if (await replanTemplateFuture(seriesTemplateId, periodEnd)) {
        toastSuccess("Regeltermin gespeichert");
      } else {
        toastWarning(FOLLOW_UP_WARNING);
      }
      onSaved({ kind: "series", seriesId: seriesTemplateId });
    }
  };

  const continuePreparedSeriesEdit = async ({
    typedScope,
    roomId,
    template,
    periodEnd,
    seriesTemplateId,
  }: {
    typedScope: "all" | "following";
    roomId: number;
    template: TimetableTemplate;
    periodEnd: string | undefined;
    seriesTemplateId: string;
  }) => {
    if (!initialInstance) return;
    // #1875: before rebuilding the series, check whether single-occurrence
    // edits in the affected window would be discarded. "following" also
    // rematerializes individually-deleted occurrences; "all" preserves them.
    // A chain-resolved save (#2187) is a same-template update, not a split —
    // it preserves individually-deleted occurrences, so counting them would
    // warn about losses that cannot happen.
    const editsFrom =
      typedScope === "following" ? initialInstance.date : berlinTodayISO();
    const editsTo = periodEnd ?? weekTo ?? editsFrom;
    let lost: EditedInWindowResult | null = null;
    try {
      const probe = await timetableService.countEditedInWindow(
        seriesTemplateId,
        editsFrom,
        editsTo,
        typedScope === "following" &&
          template.resolvedFromTemplateId === undefined,
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
          performSeriesEdit(
            typedScope,
            roomId,
            template,
            periodEnd,
            seriesTemplateId,
          ),
      });
      return;
    }

    await performSeriesEdit(
      typedScope,
      roomId,
      template,
      periodEnd,
      seriesTemplateId,
    );
    setPendingSeriesEdit(null);
    onClose();
  };

  const handleInitialScopeSelect = async (scope: string) => {
    if (
      submitting ||
      !initialInstance?.activityGroupId ||
      !isSeriesEditScope(scope)
    ) {
      return;
    }
    if (scope === "single") {
      setSelectedInstanceScope("single");
      return;
    }

    const typedScope = scope === "following" ? "following" : "all";
    setSubmitting(true);
    setValidationError(null);
    try {
      const template = await timetableService.getTemplate(
        initialInstance.activityGroupId,
        form.calendarPeriodId,
      );
      const nextForm = formFromSeries(
        template,
        initialInstance.date,
        defaultCalendarPeriodId,
      );
      setScopedSeries(template);
      setForm(nextForm);
      setInitialStudentIDsSnapshot([...nextForm.studentIds]);
      setInitialStaffIDsSnapshot([...nextForm.staffIds]);
      setInitialPrimaryStaffIDSnapshot(nextForm.primaryStaffId);
      requiredStaffTouched.current = false;
      studentRosterTouched.current = false;
      staffRosterTouched.current = false;
      listKindTouched.current = false;
      setSelectedInstanceScope(typedScope);
    } catch (err) {
      const message = timetableSeriesErrorMessage(
        err,
        "Regeltermin konnte nicht geladen werden",
      );
      setValidationError(message);
      toastError(message);
    } finally {
      setSubmitting(false);
    }
  };

  async function handleScopeSelect(
    scope: string,
    pendingOverride?: { roomId: number },
  ) {
    if (submitting || !isSeriesEditScope(scope)) return;
    const pending = pendingOverride ?? pendingSeriesEdit;
    const groupId = initialInstance?.activityGroupId;
    if (!pending || !initialInstance || !groupId) return;

    // Single-occurrence edit ("Nur diese Woche"): unchanged plain instance PUT.
    if (scope === "single") {
      setSubmitting(true);
      try {
        await checkCoverageBeforeSave(coverageProbes);
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
      const template =
        scopedSeries ??
        (await timetableService.getTemplate(groupId, form.calendarPeriodId));
      // #2187: the occurrence may belong to a capped predecessor of a split
      // chain; the backend then returns the living successor. Every series-
      // scope write from here on targets THAT template, never the
      // occurrence's own activityGroupId.
      const seriesTemplateId = template.id;
      if (template.resolvedFromTemplateId !== undefined) {
        logger.info("series_template_chain_resolved", {
          occurrence_template_id: groupId,
          resolved_template_id: seriesTemplateId,
        });
      }
      const templateCalendarPeriodId =
        resolveTemplateCalendarPeriodId(template);
      const periodEnd =
        findPeriod(templateCalendarPeriodId)?.endDate ??
        findPeriod(form.calendarPeriodId)?.endDate;
      const body = scopedSeries
        ? seriesBody(pending.roomId, Number(form.categoryId))
        : templateBodyFromForm(template, pending.roomId);
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
          seriesTemplateId,
        });
        return;
      }

      await continuePreparedSeriesEdit({
        typedScope,
        roomId: pending.roomId,
        template,
        periodEnd,
        seriesTemplateId,
      });
    } catch (err) {
      handleScopeError(scope, err);
    } finally {
      setSubmitting(false);
    }
  }

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
        seriesTemplateId: warning.seriesTemplateId,
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
    for (const currentClass of form.targetSchoolClasses) {
      if (currentClass.trim() !== "") options.add(currentClass.trim());
    }
    return [...options].sort((a, b) => a.localeCompare(b, "de"));
  }, [form.targetSchoolClasses, students]);

  const targetCohort = useMemo(() => {
    let label: string | null = null;
    let members: PersonOption[] = [];
    if (
      form.targetGroupType === "jahrgang" &&
      form.targetGradeLevels.length > 0
    ) {
      const selected = new Set(form.targetGradeLevels);
      label =
        selected.size === 1
          ? `Jahrgang ${form.targetGradeLevels[0]}`
          : `${selected.size} Jahrgänge`;
      members = students.filter((student) => {
        const gradeLevel = getSchoolYear(student.schoolClass?.trim() ?? "");
        return gradeLevel !== null && selected.has(gradeLevel);
      });
    } else if (
      form.targetGroupType === "klasse" &&
      form.targetSchoolClasses.length > 0
    ) {
      const selected = new Set(
        form.targetSchoolClasses.map((value) =>
          value.trim().toLocaleLowerCase("de"),
        ),
      );
      label =
        selected.size === 1
          ? schoolClassLabel(form.targetSchoolClasses[0] ?? "")
          : `${selected.size} Klassen`;
      members = students.filter((student) =>
        selected.has(
          (student.schoolClass?.trim() ?? "").toLocaleLowerCase("de"),
        ),
      );
    } else if (
      form.targetGroupType === "gruppe" &&
      form.educationGroupIds.length > 0
    ) {
      const selected = new Set(form.educationGroupIds);
      const singleGroup = groups.find(
        (group) => group.id === form.educationGroupIds[0],
      );
      label =
        selected.size === 1 && singleGroup
          ? `Gruppe ${singleGroup.name}`
          : `${selected.size} Gruppen`;
      members = students.filter((student) =>
        selected.has(student.groupId ?? ""),
      );
    }
    return { label, memberIds: members.map((student) => student.id) };
  }, [
    form.educationGroupIds,
    form.targetGradeLevels,
    form.targetGroupType,
    form.targetSchoolClasses,
    groups,
    students,
  ]);
  const missingTargetCohortCount = useMemo(() => {
    if (form.perWeekdayRoster && form.weekdays.length >= 2) {
      return targetCohort.memberIds.filter((id) =>
        form.weekdays.some(
          (weekday) => !rosterForWeekday(form, weekday).studentIds.includes(id),
        ),
      ).length;
    }
    const selected = new Set(form.studentIds);
    return targetCohort.memberIds.filter((id) => !selected.has(id)).length;
  }, [form, targetCohort.memberIds]);
  const targetCohortButtonLabel = targetCohort.label
    ? targetCohortActionLabel(
        targetCohort.label,
        targetCohort.memberIds.length,
        missingTargetCohortCount,
      )
    : "";
  const addTargetCohort = () => {
    if (targetCohort.memberIds.length === 0) return;
    studentRosterTouched.current = true;
    setForm((current) => {
      const studentIds = Array.from(
        new Set([...current.studentIds, ...targetCohort.memberIds]),
      );
      if (!current.perWeekdayRoster || current.weekdays.length < 2) {
        return { ...current, studentIds };
      }
      const weekdayRosters = seedWeekdayRosters(current, current.weekdays);
      for (const weekday of current.weekdays) {
        const roster = weekdayRosters[weekday]!;
        weekdayRosters[weekday] = {
          ...roster,
          studentIds: Array.from(
            new Set([...roster.studentIds, ...targetCohort.memberIds]),
          ),
        };
      }
      return { ...current, studentIds, weekdayRosters };
    });
  };

  // -------------------------------------------------------------------
  // Offering source (#2137): Betreuungsangebot als dynamische Kinderquelle
  // eines Regeltermins. Loaded lazily when the Angebot tab is active;
  // per-Jahrgang counts drive the live filter preview and the overlap
  // Hinweis against other Termine sourcing the same offering.
  // -------------------------------------------------------------------
  const [offeringSources, setOfferingSources] = useState<
    OfferingSourceOption[] | null
  >(null);
  const [offeringSourcesError, setOfferingSourcesError] = useState<
    string | null
  >(null);
  // The manual shared roster as it was before a source was selected in this
  // session — restored when the source is cleared again (#2147 review).
  const preSourceStudentIdsRef = useRef<string[]>([]);
  const wantsOfferingSources =
    expanded && isSeriesFlow && form.targetGroupType === "angebot";
  useEffect(() => {
    if (!wantsOfferingSources) return;
    let cancelled = false;
    setOfferingSourcesError(null);
    timetableService
      .getOfferingSources(form.calendarPeriodId || undefined)
      .then((options) => {
        if (!cancelled) setOfferingSources(options);
      })
      .catch((err: unknown) => {
        logger.error("offering_sources_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          setOfferingSources([]);
          setOfferingSourcesError(
            "Betreuungsangebote konnten nicht geladen werden.",
          );
        }
      });
    return () => {
      cancelled = true;
    };
  }, [wantsOfferingSources, form.calendarPeriodId]);

  // The selected offerings in stored order — the order matters: it is the
  // union's subtraction order on the backend.
  const selectedOfferingSources = useMemo(() => {
    if (!offeringSources) return [] as OfferingSourceOption[];
    return form.sourceCareOfferingIds
      .map((id) => offeringSources.find((offering) => offering.id === id))
      .filter((offering): offering is OfferingSourceOption =>
        Boolean(offering),
      );
  }, [form.sourceCareOfferingIds, offeringSources]);

  // All source offerings must share one enrollment phase (backend rule);
  // the first selection locks the phase for the rest of the list.
  const sourcePhaseLockId = useMemo(
    () => selectedOfferingSources[0]?.phaseId ?? null,
    [selectedOfferingSources],
  );

  // Exact deduplicated counts across the selection: a child enrolled in two
  // selected offerings counts once. Loaded from the combined-counts endpoint;
  // until it answers, the per-offering sums serve as an optimistic preview.
  const [combinedSourceCounts, setCombinedSourceCounts] =
    useState<CombinedOfferingCounts | null>(null);
  const combinedCountsKey = form.sourceCareOfferingIds.join(",");
  useEffect(() => {
    // Reset FIRST: after adding or removing an offering the previous exact
    // counts describe the old selection — until the new answer lands, the
    // per-offering sums must serve as the preview again.
    setCombinedSourceCounts(null);
    if (!wantsOfferingSources || combinedCountsKey === "") {
      return;
    }
    let cancelled = false;
    timetableService
      .getCombinedOfferingCounts(
        combinedCountsKey.split(","),
        form.calendarPeriodId || undefined,
      )
      .then((counts) => {
        if (!cancelled) setCombinedSourceCounts(counts);
      })
      .catch((err: unknown) => {
        logger.error("combined_offering_counts_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) setCombinedSourceCounts(null);
      });
    return () => {
      cancelled = true;
    };
  }, [wantsOfferingSources, combinedCountsKey, form.calendarPeriodId]);

  // Per-Jahrgang counts shown next to the filter checkboxes: exact when the
  // combined endpoint answered, per-offering sums (upper bound) before that.
  const sourceGradeCounts = useMemo(() => {
    if (combinedSourceCounts) return combinedSourceCounts.gradeCounts;
    const sums: Record<number, number> = {};
    for (const offering of selectedOfferingSources) {
      for (const [grade, count] of Object.entries(offering.gradeCounts)) {
        const parsed = Number(grade);
        if (!Number.isNaN(parsed)) sums[parsed] = (sums[parsed] ?? 0) + count;
      }
    }
    return sums;
  }, [combinedSourceCounts, selectedOfferingSources]);

  // Jahrgänge offered for the filter: every grade with enrolled children plus
  // the already-selected ones (so a saved filter stays visible even when its
  // grade currently has no children).
  const sourceGradeOptions = useMemo(() => {
    const grades = new Set<number>(form.sourceGradeLevels);
    for (const offering of selectedOfferingSources) {
      for (const grade of Object.keys(offering.gradeCounts)) {
        const parsed = Number(grade);
        if (parsed > 0) grades.add(parsed);
      }
    }
    return [...grades].sort((a, b) => a - b);
  }, [form.sourceGradeLevels, selectedOfferingSources]);

  // Live count of children the current filter captures — deduplicated
  // across the selected offerings once the combined endpoint answered.
  const sourceFilteredCount = useMemo(() => {
    if (selectedOfferingSources.length === 0) return 0;
    if (form.sourceGradeLevels.length === 0) {
      if (combinedSourceCounts) return combinedSourceCounts.totalCount;
      return selectedOfferingSources.reduce(
        (sum, offering) => sum + offering.totalCount,
        0,
      );
    }
    return form.sourceGradeLevels.reduce(
      (sum, grade) => sum + (sourceGradeCounts[grade] ?? 0),
      0,
    );
  }, [
    form.sourceGradeLevels,
    selectedOfferingSources,
    combinedSourceCounts,
    sourceGradeCounts,
  ]);

  // OGS am Berg: series start before the phase service window leaves the first
  // occurrences empty of offering-fed children (enrollments clamp to phase start).
  const sourcePhaseKidsFromWarning = useMemo(() => {
    return offeringPhaseStartWarning(selectedOfferingSources[0], form.date);
  }, [selectedOfferingSources, form.date]);

  // Other Termine sourcing the same offering whose Jahrgang subsets overlap
  // the current filter (empty filter = alle Jahrgänge). Advisory only — the
  // save is never blocked, mirroring the conflict warnings.
  const sourceOverlapWarnings = useMemo(() => {
    if (selectedOfferingSources.length === 0) return [] as string[];
    const currentTemplateId = effectiveSeries?.id;
    const selected = form.sourceGradeLevels;
    const seen = new Set<string>();
    const warnings: string[] = [];
    for (const offering of selectedOfferingSources) {
      for (const template of offering.sourcedTemplates) {
        if (template.id === currentTemplateId || seen.has(template.id)) {
          continue;
        }
        const other = template.gradeLevels;
        if (
          selected.length > 0 &&
          other.length > 0 &&
          !other.some((grade) => selected.includes(grade))
        ) {
          continue;
        }
        seen.add(template.id);
        const scope =
          template.gradeLevels.length > 0
            ? `Jahrgang ${template.gradeLevels.join(", ")}`
            : "alle Jahrgänge";
        warnings.push(
          `„${template.name}“ nutzt dasselbe Angebot „${offering.name}“ (${scope}). Kinder mit gemeinsamem Jahrgang werden in beiden Regelterminen eingeplant.`,
        );
      }
    }
    return warnings;
  }, [form.sourceGradeLevels, effectiveSeries?.id, selectedOfferingSources]);

  const applySourceOfferingIds = (nextIds: string[]) => {
    // Selecting the first source clears the manual roster (server-managed).
    // Stash it so clearing the last source restores the admin's picks —
    // submitting the emptied array would wipe the shared manual assignments
    // on save (#2147 review). The ref is touched here, outside the updater,
    // so the updater stays pure.
    if (nextIds.length > 0 && form.sourceCareOfferingIds.length === 0) {
      preSourceStudentIdsRef.current = form.studentIds;
    }
    if (nextIds.length > 0 && form.perWeekdayRoster) {
      staffRosterTouched.current = true;
      studentRosterTouched.current = true;
    }
    const restoredStudentIds = preSourceStudentIdsRef.current;
    setForm((current) => {
      const next = {
        ...current,
        sourceCareOfferingIds: nextIds,
        // The filter applies to the UNION of the selected offerings, so it
        // survives adding/removing single offerings and falls only with the
        // last source.
        sourceGradeLevels: nextIds.length > 0 ? current.sourceGradeLevels : [],
        // The sourced roster is server-managed; clearing the last source
        // restores the manual roster picked before the first source was set.
        studentIds:
          nextIds.length > 0
            ? []
            : current.sourceCareOfferingIds.length > 0
              ? restoredStudentIds
              : current.studentIds,
      };
      if (nextIds.length === 0 || !current.perWeekdayRoster) return next;
      // A source knows only one shared Besetzung, so per-weekday mode ends
      // right here, visibly in the Personal step, not silently at save time.
      // Days that staff identically collapse into the shared controls. Days
      // that deviate are NOT aggregated into an all-weekdays union: nobody
      // chose that staffing, so the shared Besetzung starts empty and has to
      // be picked explicitly (#2147 review round 13).
      const baseline = rosterForWeekday(
        current,
        current.weekdays[0] ?? activeRosterWeekday,
      );
      const flattened = hasPerWeekdayStaffDeviation(current)
        ? { staffIds: [] as string[], primaryStaffId: "" }
        : {
            staffIds: [...baseline.staffIds],
            primaryStaffId: baseline.primaryStaffId,
          };
      return {
        ...next,
        ...flattened,
        perWeekdayRoster: false,
        weekdayRosters: {},
      };
    });
    setValidationError(null);
  };

  // Offering selection awaiting explicit confirmation because applying it
  // removes a deliberate per-weekday staffing (#2147 review): with a source
  // set, the payload carries only the shared staff list (the backend rejects
  // weekday_assignments next to a source). Confirming empties the shared
  // Besetzung, so the replacement staffing is an explicit choice, never an
  // implicit all-weekdays union (#2147 review round 13). A list because the
  // MultiCheckboxSelect's "Alle auswählen" can add several offerings at once.
  const [pendingSourceOfferingIds, setPendingSourceOfferingIds] = useState<
    string[] | null
  >(null);
  useEffect(() => {
    setPendingSourceOfferingIds(null);
    // A new modal session must not inherit the previous session's stashed
    // manual roster: clearing a source in a freshly opened, already sourced
    // template would otherwise restore (and save) another template's picks
    // (#2147 review round 10).
    preSourceStudentIdsRef.current = [];
  }, [isOpen]);

  const changeSourceOfferings = (nextIds: string[]) => {
    if (
      nextIds.length > 0 &&
      form.sourceCareOfferingIds.length === 0 &&
      hasPerWeekdayStaffDeviation(form)
    ) {
      setPendingSourceOfferingIds(nextIds);
      return;
    }
    applySourceOfferingIds(nextIds);
  };

  const confirmPendingSourceOffering = () => {
    if (pendingSourceOfferingIds === null) return;
    applySourceOfferingIds(pendingSourceOfferingIds);
    setPendingSourceOfferingIds(null);
  };

  const cancelPendingSourceOffering = () => setPendingSourceOfferingIds(null);

  const toggleSourceGradeLevel = (grade: number) => {
    setForm((current) => ({
      ...current,
      sourceGradeLevels: current.sourceGradeLevels.includes(grade)
        ? current.sourceGradeLevels.filter((item) => item !== grade)
        : [...current.sourceGradeLevels, grade].sort((a, b) => a - b),
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
    const categorySeq = ++categoryLoadSeq.current;
    if (selectId) {
      setForm((prev) => ({ ...prev, categoryId: selectId }));
    }
    try {
      const data = await fetchPlannerActivityCategories();
      const sorted = [...data].sort((a, b) =>
        a.name.localeCompare(b.name, "de"),
      );
      if (categoryLoadSeq.current !== categorySeq) return;
      setCategories(sorted);
      setForm((prev) => {
        const categoryId = reconcileCategoryId(
          prev.categoryId,
          sorted,
          selectId,
        );
        return categoryId === prev.categoryId ? prev : { ...prev, categoryId };
      });
    } catch (err: unknown) {
      logger.error("categories_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }, []);

  const refreshPlanningTracks = useCallback(async (selectId?: string) => {
    if (selectId) {
      setForm((prev) => ({ ...prev, planningTrackId: selectId }));
    }
    try {
      setPlanningTracks(await planningTrackService.list());
    } catch (err: unknown) {
      logger.error("planning_tracks_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }, []);

  return {
    form,
    update,
    updateRepeat,
    selectCalendarPeriod,
    toggleWeekday,
    changeTargetGroupType,
    fieldErrors,
    validationError,
    rooms,
    categories,
    refreshCategories,
    planningTracks,
    refreshPlanningTracks,
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
    scopeSelectionRequired,
    isScopedSeriesEdit: scopedSeries !== null,
    setPendingSeriesEdit,
    handleInitialScopeSelect,
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
    offeringSources,
    offeringSourcesError,
    selectedOfferingSources,
    sourcePhaseLockId,
    sourceGradeOptions,
    sourceGradeCounts,
    sourceFilteredCount,
    sourcePhaseKidsFromWarning,
    sourceOverlapWarnings,
    changeSourceOfferings,
    pendingSourceOfferingIds,
    confirmPendingSourceOffering,
    cancelPendingSourceOffering,
    toggleSourceGradeLevel,
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
    activeRosterWeekday,
    setActiveRosterWeekday,
    setPerWeekdayRoster,
    setWeekdayRoster,
    applyActiveWeekdayRosterToAll,
    listKindTouched,
    manualWeekPattern,
  };
}
