"use client";

import { ChevronDown, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useModal } from "~/components/dashboard/modal-context";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useToast } from "~/contexts/ToastContext";
import type { ActivityCategory } from "~/lib/activity-helpers";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import { getSchoolYear } from "~/lib/student-helpers";
import { staffService } from "~/lib/staff-api";
import { useTenant } from "~/lib/tenant-context";
import { timetableService } from "~/lib/timetable-api";
import { useDebounce } from "~/lib/use-debounce";
import {
  chunkDateRange,
  getActivityColor,
  getGermanWeekdayLong,
  getGermanWeekdayShort,
  resolveTemplateCalendarPeriodId,
  weekdayDatesInRange,
} from "~/lib/timetable-helpers";
import {
  timetableMutedSurface,
  timetableNestedSurface,
  timetableRequiredMark,
  timetableSearchClass,
  timetableSelectClass,
  timetableTextAreaClass,
} from "./timetable-style";
import type {
  ActivityType,
  ConflictWarningItem,
  CreateInstanceBody,
  CreateTemplateBody,
  EnrichedInstance,
  ShiftCoverageCheckParams,
  ShiftCoverageWarningItem,
  TargetGroupType,
  TimetableTemplate,
  UpdateTemplateBody,
} from "~/lib/timetable-types";

interface RoomOption {
  id: number;
  name: string;
  building?: string;
}

interface PersonOption {
  id: string;
  name: string;
  schoolClass?: string;
  groupId?: string;
  groupName?: string;
}

interface GroupOption {
  id: string;
  name: string;
}

const STUDENT_PAGE_SIZE = 500;
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

interface BackendRoomsEnvelope {
  data?: Array<{
    id: number;
    name: string;
    building?: string;
  }>;
}

interface BackendGroupsEnvelope {
  data?: Array<{
    id: number;
    name: string;
  }>;
}

type RepeatMode = "none" | "weekly" | "biweekly";

interface EventFormState {
  title: string;
  date: string;
  startTime: string;
  endTime: string;
  roomId: string;
  type: ActivityType;
  categoryId: string;
  educationGroupId: string;
  notes: string;
  repeat: RepeatMode;
  weekPattern: 0 | 1 | 2;
  weekdays: number[];
  calendarPeriodId: string;
  studentIds: string[];
  staffIds: string[];
  primaryStaffId: string;
  /**
   * Manual Personalbedarf override (issue #1839). Empty = automatic: a single
   * occurrence inherits the series value, otherwise the Betreuungsschlüssel
   * derivation applies. A non-negative integer overrides both (on a single
   * occurrence it becomes a pin that survives series edits). Held as a string
   * because it is an <input> value; parsed via parseRequiredStaffOverride.
   */
  requiredStaff: string;
  /**
   * Zielgruppe (target group, issue #1838). "gruppe" reuses educationGroupId
   * above as its value rather than a separate field — switching away from
   * "gruppe" clears educationGroupId so the two never disagree.
   */
  targetGroupType: TargetGroupType;
  targetGradeLevel: string; // "" | configured grade, only meaningful for "jahrgang"
  targetSchoolClass: string; // "", only meaningful for "klasse"
}

type TimetableEventModalResult =
  | { kind: "instance"; instance: EnrichedInstance }
  | { kind: "series"; seriesId: string; linkedInstanceId?: string };

interface TimetableEventModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (result: TimetableEventModalResult) => void;
  defaultDate: string;
  weekFrom?: string;
  weekTo?: string;
  calendarPeriods: CalendarPeriod[];
  defaultCalendarPeriodId?: string | null;
  showPeriodField?: boolean;
  initialInstance?: EnrichedInstance | null;
  initialSeries?: TimetableTemplate | null;
  convertInstance?: EnrichedInstance | null;
  onDeleteSeries?: (
    template: TimetableTemplate,
    effectiveDate: string,
  ) => Promise<void>;
  defaultRepeat?: RepeatMode;
  /**
   * "quick" starts collapsed with only Titel, Datum/Zeiten, Raum and a
   * plain-language repeat select (US-1 quick create). "Benutzerdefiniert"
   * in that select expands to the full form. Default "full".
   */
  variant?: "full" | "quick";
  defaultStartTime?: string;
  defaultEndTime?: string;
  canCheckShiftCoverage: boolean;
}

const logger = createLogger({ component: "TimetableEventModal" });
const FORM_SELECT_CLASS = timetableSelectClass;
const FORM_SEARCH_CLASS = timetableSearchClass;

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

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

const TYPE_OPTIONS: Array<{
  value: ActivityType;
  label: string;
  hint: string;
}> = [
  { value: "care", label: "Betreuung", hint: "Mensa, Lernzeit, Freispiel" },
  { value: "activity", label: "AG", hint: "Yoga, Bouldern, …" },
  { value: "external", label: "Extern", hint: "DAZ, Musikschule" },
];

const REPEAT_OPTIONS: Array<{ value: RepeatMode; label: string }> = [
  { value: "none", label: "Nie" },
  { value: "weekly", label: "Jede Woche" },
  { value: "biweekly", label: "Alle 2 Wochen" },
];

/** Zielgruppe (target group) tab options, issue #1838. */
const TARGET_GROUP_OPTIONS: Array<{ value: TargetGroupType; label: string }> = [
  { value: "none", label: "Keine" },
  { value: "jahrgang", label: "Jahrgang" },
  { value: "klasse", label: "Klasse" },
  { value: "gruppe", label: "Gruppe" },
  { value: "angebot", label: "Angebotsauswahl" },
];

function isoWeekday(dateISO: string): number {
  const d = new Date(`${dateISO}T00:00:00`);
  const day = d.getDay();
  if (day === 0) return 7;
  return day;
}

function weekdayLabel(iso: number): string {
  const ref = new Date(2024, 0, 1);
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
}

function emptyForm(
  defaultDate: string,
  defaultCalendarPeriodId?: string | null,
  defaultRepeat: RepeatMode = "none",
  defaultStartTime = "12:00",
  defaultEndTime = "13:00",
): EventFormState {
  const weekday = isoWeekday(defaultDate);
  return {
    title: "",
    date: defaultDate,
    startTime: defaultStartTime,
    endTime: defaultEndTime,
    roomId: "",
    type: "care",
    categoryId: "",
    educationGroupId: "",
    notes: "",
    repeat: defaultRepeat,
    weekPattern: defaultRepeat === "biweekly" ? 2 : 0,
    weekdays: weekday >= 1 && weekday <= 5 ? [weekday] : [1],
    calendarPeriodId: defaultCalendarPeriodId ?? "",
    studentIds: [],
    staffIds: [],
    primaryStaffId: "",
    requiredStaff: "",
    targetGroupType: "none",
    targetGradeLevel: "",
    targetSchoolClass: "",
  };
}

function formFromInstance(
  instance: EnrichedInstance,
  defaultCalendarPeriodId?: string | null,
  repeat: RepeatMode = "none",
): EventFormState {
  const weekday = isoWeekday(instance.date);
  return {
    title: instance.title,
    date: instance.date,
    startTime: instance.startTime,
    endTime: instance.endTime,
    roomId: instance.roomId,
    type: instance.activityType,
    categoryId: "",
    educationGroupId: "",
    notes: instance.notes ?? "",
    repeat,
    weekPattern: repeat === "biweekly" ? 2 : 0,
    weekdays: weekday >= 1 && weekday <= 5 ? [weekday] : [1],
    calendarPeriodId: defaultCalendarPeriodId ?? "",
    studentIds: instance.studentIds,
    staffIds: instance.staff.map((item) => item.staffId),
    primaryStaffId:
      instance.staff.find((item) => item.isPrimary)?.staffId ?? "",
    requiredStaff:
      instance.requiredStaffOverride !== undefined
        ? String(instance.requiredStaffOverride)
        : "",
    targetGroupType: "none",
    targetGradeLevel: "",
    targetSchoolClass: "",
  };
}

function formFromSeries(
  series: TimetableTemplate,
  defaultDate: string,
  defaultCalendarPeriodId?: string | null,
): EventFormState {
  const firstSchedule = series.schedules[0];
  const weekdays = series.schedules.map((schedule) => schedule.weekday);
  const weekPattern =
    firstSchedule?.weekPattern === 1 || firstSchedule?.weekPattern === 2
      ? firstSchedule.weekPattern
      : 0;
  const repeat = weekPattern === 2 ? "biweekly" : "weekly";
  return {
    title: series.name,
    date: defaultDate,
    startTime: firstSchedule?.startTime ?? "12:00",
    endTime: firstSchedule?.endTime ?? "13:00",
    roomId: series.roomId ?? "",
    type: series.type,
    categoryId: series.categoryId,
    educationGroupId: series.educationGroupId ?? "",
    notes: "",
    repeat,
    weekPattern,
    weekdays: weekdays.length > 0 ? weekdays : [1],
    calendarPeriodId:
      resolveTemplateCalendarPeriodId(series) ?? defaultCalendarPeriodId ?? "",
    studentIds: series.studentIds,
    staffIds: series.staffIds,
    primaryStaffId: series.primaryStaffId ?? "",
    requiredStaff:
      series.requiredStaffOverride !== undefined
        ? String(series.requiredStaffOverride)
        : "",
    targetGroupType: series.targetGroupType,
    targetGradeLevel:
      series.targetGradeLevel !== undefined && series.targetGradeLevel !== null
        ? String(series.targetGradeLevel)
        : "",
    targetSchoolClass: series.targetSchoolClass?.trim() ?? "",
  };
}

// parseRequiredStaffOverride maps the "Benötigtes Personal" input to the
// override wire value (#1839): empty/invalid → null (clear the override, derive
// from the Betreuungsschlüssel); a whole non-negative integer → manual
// override. Only plain digit strings are accepted — Number.parseInt would
// silently coerce "1e2" → 1 or "2.5" → 2 and persist the wrong requirement.
function parseRequiredStaffOverride(value: string): number | null {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  const parsed = Number(trimmed);
  return Number.isSafeInteger(parsed) ? parsed : null;
}

function sortPeople<T extends PersonOption>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    const classCompare = (a.schoolClass ?? "").localeCompare(
      b.schoolClass ?? "",
      "de",
    );
    if (classCompare !== 0) return classCompare;
    const groupCompare = (a.groupName ?? "").localeCompare(
      b.groupName ?? "",
      "de",
    );
    if (groupCompare !== 0) return groupCompare;
    return a.name.localeCompare(b.name, "de");
  });
}

function schoolClassLabel(schoolClass: string): string {
  const trimmed = schoolClass.trim();
  return /^klasse(?:\s|$)/i.test(trimmed) ? trimmed : `Klasse ${trimmed}`;
}

function targetCohortActionLabel(
  label: string,
  memberCount: number,
  missingMemberCount: number,
): string {
  if (memberCount === 0) return `Keine Kinder aus ${label} gefunden`;
  if (missingMemberCount === 0) {
    return `Alle Kinder aus ${label} übernommen`;
  }
  const childLabel = memberCount === 1 ? "Kind" : "Kinder";
  return `Alle ${memberCount} ${childLabel} aus ${label} übernehmen`;
}

function initialStudentIDs(
  initialInstance: EnrichedInstance | null,
  initialSeries: TimetableTemplate | null,
  convertInstance: EnrichedInstance | null,
): string[] {
  if (initialSeries) return initialSeries.studentIds;
  if (convertInstance) return convertInstance.studentIds;
  if (initialInstance) return initialInstance.studentIds;
  return [];
}

function initialStaffIDs(
  initialInstance: EnrichedInstance | null,
  initialSeries: TimetableTemplate | null,
  convertInstance: EnrichedInstance | null,
): string[] {
  if (initialSeries) return initialSeries.staffIds;
  const instance = convertInstance ?? initialInstance;
  return instance?.staff.map((item) => item.staffId) ?? [];
}

function initialPrimaryStaffID(
  initialInstance: EnrichedInstance | null,
  initialSeries: TimetableTemplate | null,
  convertInstance: EnrichedInstance | null,
): string {
  if (initialSeries) return initialSeries.primaryStaffId ?? "";
  const instance = convertInstance ?? initialInstance;
  return instance?.staff.find((item) => item.isPrimary)?.staffId ?? "";
}

async function fetchAllStudentOptions(): Promise<PersonOption[]> {
  const firstPage = await fetchStudents({
    page: 1,
    page_size: STUDENT_PAGE_SIZE,
  });
  const totalPages = Math.max(1, firstPage.pagination?.total_pages ?? 1);
  const remainingPages =
    totalPages > 1
      ? await Promise.all(
          Array.from({ length: totalPages - 1 }, (_, index) =>
            fetchStudents({
              page: index + 2,
              page_size: STUDENT_PAGE_SIZE,
            }),
          ),
        )
      : [];

  const byID = new Map<string, PersonOption>();
  for (const page of [firstPage, ...remainingPages]) {
    for (const student of page.students) {
      byID.set(student.id, {
        id: student.id,
        name: student.name,
        schoolClass: student.school_class,
        groupId: student.group_id,
        groupName: student.group_name,
      });
    }
  }
  return [...byID.values()];
}

export function TimetableEventModal({
  isOpen,
  onClose,
  onSaved,
  defaultDate,
  weekFrom,
  weekTo,
  calendarPeriods,
  defaultCalendarPeriodId,
  showPeriodField = false,
  initialInstance = null,
  initialSeries = null,
  convertInstance = null,
  onDeleteSeries,
  defaultRepeat = "none",
  variant = "full",
  defaultStartTime,
  defaultEndTime,
  canCheckShiftCoverage,
}: TimetableEventModalProps) {
  const { tenant } = useTenant();
  const {
    success: toastSuccess,
    error: toastError,
    warning: toastWarning,
  } = useToast();
  const { isModalOpen } = useModal();
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
  const [moreOpen, setMoreOpen] = useState(false);
  // Validated room id stashed while the Dreifach-Frage dialog (US-5) is open.
  const [pendingSeriesEdit, setPendingSeriesEdit] = useState<{
    roomId: number;
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

  const isEditingInstance = initialInstance !== null;
  const isEditingSeries = initialSeries !== null;
  const isConverting = convertInstance !== null;
  const isSeriesFlow = form.repeat !== "none" || isEditingSeries;
  const choiceDialogOpen = pendingSeriesEdit !== null;
  const canDeleteSeries = isEditingSeries && initialSeries && onDeleteSeries;
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
    setForm(nextForm);
    setValidationError(null);
    setFieldErrors({});
    setDeleteConfirmOpen(false);
    setDeleteEffectiveDate(berlinTodayISO());
    setDeleteError(null);
    setDeletingSeries(false);
    setExpanded(variant === "full");
    setMoreOpen(false);
    setPendingSeriesEdit(null);
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
      fetch("/api/rooms", { credentials: "include" })
        .then((r) => r.json() as Promise<BackendRoomsEnvelope>)
        .then((j): RoomOption[] =>
          (j.data ?? []).map((room) => ({
            id: room.id,
            name: room.name,
            building: room.building,
          })),
        )
        .catch((err: unknown) => {
          logger.error("rooms_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as RoomOption[];
        }),
      fetch("/api/activities/categories", { credentials: "include" })
        .then((r) => r.json() as Promise<{ data?: ActivityCategory[] }>)
        .then((j) => j.data ?? [])
        .catch((err: unknown) => {
          logger.error("categories_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as ActivityCategory[];
        }),
      fetch("/api/groups?page_size=1000", { credentials: "include" })
        .then((r) => r.json() as Promise<BackendGroupsEnvelope>)
        .then((j): GroupOption[] =>
          (j.data ?? []).map((group) => ({
            id: String(group.id),
            name: group.name,
          })),
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
        setMoreOpen(true);
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
        setMoreOpen(true);
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
  };

  const updateRepeat = (repeat: RepeatMode) => {
    setForm((prev) => ({
      ...prev,
      repeat,
      weekPattern: repeat === "biweekly" ? 2 : 0,
    }));
    setValidationError(null);
    clearFieldError("repeat");
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
    if (Object.keys(errors).length > 0) {
      // Quick mode hides the series controls; expand so an inline error on
      // a hidden field (Kategorie, Planungszeitraum, Wochentage) is visible.
      if (
        !expanded &&
        (errors.categoryId ??
          errors.calendarPeriodId ??
          errors.weekdays ??
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
    weekdays: form.weekdays,
    start_time: form.startTime,
    end_time: form.endTime,
    room_id: roomId,
    category_id: categoryId,
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
    const templateStaffIDs = staffRosterEditable
      ? form.staffIds
      : template.staffIds;
    const primaryStaffId = staffRosterEditable
      ? form.primaryStaffId || template.primaryStaffId
      : template.primaryStaffId;
    return {
      name: form.title.trim(),
      type: template.type,
      weekdays: weekdays.length > 0 ? weekdays : [1],
      start_time: form.startTime,
      end_time: form.endTime,
      room_id: roomId,
      category_id: Number(template.categoryId),
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
      // Personalbedarf override (#1839) IS editable in this form (unlike
      // max_participants), so an all/following save carries the form value
      // onto the template.
      required_staff: parseRequiredStaffOverride(form.requiredStaff),
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

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
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
        await timetableService.updateTemplate(
          initialSeries.id,
          seriesBody(parsed.roomId, parsed.categoryId),
        );
        if (await replanTemplateFuture(initialSeries.id)) {
          toastSuccess("Regeltermin gespeichert");
        } else {
          toastWarning(FOLLOW_UP_WARNING);
        }
        onSaved({ kind: "series", seriesId: initialSeries.id });
        onClose();
        return;
      }

      if (convertInstance) {
        const created = await timetableService.createTemplate(
          seriesBody(parsed.roomId, parsed.categoryId),
        );
        await timetableService.update(
          convertInstance.id,
          instanceBody(parsed.roomId, created.templateId),
        );
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

  const handleScopeSelect = async (scope: string) => {
    if (submitting) return;
    const pending = pendingSeriesEdit;
    const groupId = initialInstance?.activityGroupId;
    if (!pending || !initialInstance || !groupId) return;
    setSubmitting(true);
    try {
      if (scope === "single") {
        await checkCoverageBeforeSave(coverageProbe);
        const saved = await timetableService.update(
          initialInstance.id,
          instanceBody(pending.roomId, groupId),
        );
        toastSuccess("Termin gespeichert");
        onSaved({ kind: "instance", instance: saved });
      } else {
        const template = await timetableService.getTemplate(groupId);
        const templateCalendarPeriodId =
          resolveTemplateCalendarPeriodId(template);
        const periodEnd =
          findPeriod(templateCalendarPeriodId)?.endDate ??
          findPeriod(form.calendarPeriodId)?.endDate;
        const body = templateBodyFromForm(template, pending.roomId);
        const typedScope = scope === "following" ? "following" : "all";
        const scopeProbe = seriesCoverageProbe(
          template,
          body,
          typedScope === "following" ? initialInstance.date : berlinTodayISO(),
          typedScope === "all" ? groupId : undefined,
        );
        await checkCoverageBeforeSave(scopeProbe);
        if (scope === "following") {
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
                  chunkErr instanceof Error
                    ? chunkErr.message
                    : String(chunkErr),
              });
              followUpOk = false;
              break;
            }
          }
          if (followUpOk) {
            toastSuccess(
              `Regeltermin ab ${formatDate(effectiveDate)} geändert`,
            );
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
      }
      setPendingSeriesEdit(null);
      onClose();
    } catch (err) {
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
      setValidationError(msg);
      toastError(msg);
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
      setMoreOpen(true);
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
      setMoreOpen(true);
    } finally {
      if (isCurrentStaffLoad()) setLoadingStaff(false);
    }
  };

  // Datum and Notiz only apply to the single-instance scope — series-wide
  // scopes write the template, which carries neither field.
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

  let studentRosterField: React.ReactNode;
  if (loadingStudents) {
    studentRosterField = (
      <Alert
        type="info"
        message="Kinderliste wird geladen … Die bestehende Kinderzuordnung bleibt beim Speichern unverändert."
      />
    );
  } else if (studentLoadError) {
    studentRosterField = (
      <div className="flex flex-col gap-2">
        <Alert type="warning" message={studentLoadError} />
        <Button
          type="button"
          variant="outline"
          size="compact"
          className="self-start"
          onClick={() => void retryStudentLoad()}
        >
          Kinder erneut laden
        </Button>
      </div>
    );
  } else {
    studentRosterField = (
      <MultiSelectField
        label="Kinder"
        options={students}
        value={form.studentIds}
        onChange={(ids) => update("studentIds", ids)}
        metadata="student"
        bulkOptions={studentBulkOptions}
      />
    );
  }

  let staffRosterField: React.ReactNode;
  if (loadingStaff) {
    staffRosterField = (
      <Alert
        type="info"
        message="Personalliste wird geladen … Die bestehende Personalzuordnung bleibt beim Speichern unverändert."
      />
    );
  } else if (staffLoadError) {
    staffRosterField = (
      <div className="flex flex-col gap-2">
        <Alert type="warning" message={staffLoadError} />
        <Button
          type="button"
          variant="outline"
          size="compact"
          className="self-start"
          onClick={() => void retryStaffLoad()}
        >
          Personal erneut laden
        </Button>
      </div>
    );
  } else {
    staffRosterField = (
      <>
        <MultiSelectField
          label="Personal"
          options={staff}
          value={form.staffIds}
          onChange={(ids) => {
            update("staffIds", ids);
            if (form.primaryStaffId && !ids.includes(form.primaryStaffId)) {
              update("primaryStaffId", "");
            }
          }}
          metadata="staff"
        />

        {isSeriesFlow && form.staffIds.length > 0 && (
          <Field label="Zuständige Person" htmlFor="event_primary_staff">
            <select
              id="event_primary_staff"
              value={form.primaryStaffId}
              onChange={(event) => update("primaryStaffId", event.target.value)}
              className={FORM_SELECT_CLASS}
            >
              <option value="">Keine Auswahl</option>
              {staff
                .filter((person) => form.staffIds.includes(person.id))
                .map((person) => (
                  <option key={person.id} value={person.id}>
                    {person.name}
                  </option>
                ))}
            </select>
          </Field>
        )}
      </>
    );
  }

  // Personal renders before Kinder in every mode (Streichliste 8); the
  // quick variant tucks all of this behind the "Weitere Optionen" row.
  const peopleFields = (
    <>
      {staffRosterField}

      <Field label="Benötigtes Personal" htmlFor="event_required_staff">
        <Input
          id="event_required_staff"
          type="number"
          min={0}
          step={1}
          inputMode="numeric"
          value={form.requiredStaff}
          onChange={(event) => update("requiredStaff", event.target.value)}
          placeholder="automatisch aus Betreuungsschlüssel"
          controlSize="compact"
        />
        <p className="mt-1 text-xs text-gray-500">
          Leer = automatisch: Es gilt der Wert der Terminreihe, sonst die
          Berechnung aus dem Betreuungsschlüssel (Kinderzahl). Eine Zahl legt
          den Bedarf fest und überschreibt beides.
        </p>
      </Field>

      {studentRosterField}

      {!isSeriesFlow && (
        <Field label="Notiz" htmlFor="event_notes">
          <textarea
            id="event_notes"
            value={form.notes}
            onChange={(event) => update("notes", event.target.value)}
            rows={3}
            className={timetableTextAreaClass}
          />
        </Field>
      )}
    </>
  );

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent
        widthClass="sm:w-[760px]"
        // The ChoiceModal portals to document.body and lives outside the
        // drawer's DOM. Without these guards Vaul's DismissableLayer treats
        // every click inside the open dialog as an outside-click and closes
        // the slide-over, unmounting the dialog before its buttons can
        // fire. See issue #1358. `submitting` additionally blocks dismissal
        // mid-save (both the form submit and the scope flow set it).
        onInteractOutside={(event) => {
          if (isModalOpen || choiceDialogOpen || submitting) {
            event.preventDefault();
          }
        }}
        onEscapeKeyDown={(event) => {
          if (isModalOpen || choiceDialogOpen || submitting) {
            event.preventDefault();
          }
        }}
      >
        <SlideOverHeader>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <SlideOverTitle>{title}</SlideOverTitle>
              <SlideOverDescription>
                {isSeriesFlow
                  ? "Regelmäßigen Termin mit Kindern und Personal planen."
                  : isEditingInstance
                    ? "Termin im Betreuungsplan bearbeiten."
                    : "Einmaligen Termin im Betreuungsplan anlegen."}
              </SlideOverDescription>
            </div>
            <SlideOverCloseButton />
          </div>
        </SlideOverHeader>

        <form
          id="timetable-event-form"
          noValidate
          onSubmit={(event) => void handleSubmit(event)}
          className="flex-1 overflow-y-auto px-5 py-4"
        >
          <div className="flex flex-col gap-5">
            {initialInstance && initialInstance.status !== "planned" && (
              <Alert
                type="error"
                message="Nur geplante Termine können bearbeitet werden."
              />
            )}

            {isEditingSeries && (
              <p className="text-xs text-gray-500">
                Änderungen gelten für alle Termine dieser Serie.
              </p>
            )}

            <Field label="Titel" htmlFor="event_title" required>
              <Input
                id="event_title"
                value={form.title}
                onChange={(event) => update("title", event.target.value)}
                placeholder="z. B. Mensa, Lernzeit 1a, Yoga AG"
                maxLength={255}
                controlSize="compact"
                error={fieldErrors.title}
                autoFocus
                required
              />
            </Field>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Field label="Datum" htmlFor="event_date" required>
                <Input
                  id="event_date"
                  type="date"
                  value={form.date}
                  controlSize="compact"
                  error={fieldErrors.date}
                  onChange={(event) => {
                    const nextDate = event.target.value;
                    const nextWeekday = isoWeekday(nextDate);
                    update("date", nextDate);
                    // One-off events follow the date; the quick preset
                    // "Wöchentlich am <Tag>" retargets to the new weekday.
                    const followsDate =
                      !isSeriesFlow ||
                      (!expanded && quickPreset === "woechentlich-am");
                    if (followsDate && nextWeekday >= 1 && nextWeekday <= 5) {
                      update("weekdays", [nextWeekday]);
                    }
                  }}
                  required
                />
              </Field>
              <Field label="Start" htmlFor="event_start" required>
                <Input
                  id="event_start"
                  type="time"
                  value={form.startTime}
                  controlSize="compact"
                  error={fieldErrors.startTime}
                  onChange={(event) => update("startTime", event.target.value)}
                  required
                />
              </Field>
              <Field label="Ende" htmlFor="event_end" required>
                <Input
                  id="event_end"
                  type="time"
                  value={form.endTime}
                  controlSize="compact"
                  error={fieldErrors.endTime}
                  onChange={(event) => update("endTime", event.target.value)}
                  required
                />
              </Field>
            </div>

            <Field
              label="Raum"
              htmlFor="event_room"
              required
              error={fieldErrors.roomId}
            >
              <select
                id="event_room"
                value={form.roomId}
                onChange={(event) => update("roomId", event.target.value)}
                disabled={loadingRefs}
                required
                aria-invalid={fieldErrors.roomId ? true : undefined}
                aria-describedby={
                  fieldErrors.roomId ? "event_room_error" : undefined
                }
                className={FORM_SELECT_CLASS}
              >
                <option value="">
                  {loadingRefs ? "Lade Räume …" : "Raum auswählen …"}
                </option>
                {rooms.map((room) => (
                  <option key={room.id} value={room.id}>
                    {room.building
                      ? `${room.building} - ${room.name}`
                      : room.name}
                  </option>
                ))}
              </select>
            </Field>

            {expanded ? (
              <div className="flex flex-col gap-1">
                <span className="text-xs font-semibold text-gray-700">
                  Wiederholt sich
                </span>
                <Tabs
                  value={form.repeat}
                  onValueChange={(value) => {
                    const nextRepeat = value as RepeatMode;
                    updateRepeat(nextRepeat);
                    if (nextRepeat !== "none" && form.weekdays.length === 0) {
                      const weekday = isoWeekday(form.date);
                      update(
                        "weekdays",
                        weekday >= 1 && weekday <= 5 ? [weekday] : [1],
                      );
                    }
                  }}
                >
                  <TabsList aria-label="Wiederholung" className="w-fit">
                    {REPEAT_OPTIONS.map((option) => (
                      <TabsTrigger
                        key={option.value}
                        value={option.value}
                        disabled={isEditingSeries && option.value === "none"}
                      >
                        {option.label}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                </Tabs>
              </div>
            ) : (
              <Field label="Wiederholt sich" htmlFor="event_quick_repeat">
                <select
                  id="event_quick_repeat"
                  value={quickPreset}
                  onChange={(event) =>
                    handleQuickPresetChange(event.target.value)
                  }
                  className={FORM_SELECT_CLASS}
                >
                  <option value="einmalig">Einmalig</option>
                  {/* On Sa/So a weekly preset would silently save a Monday
                      series (weekdays are Mo–Fr) — omit it instead. */}
                  {dateWeekday >= 1 && dateWeekday <= 5 && (
                    <option value="woechentlich-am">
                      {`Wöchentlich am ${dateWeekdayName}`}
                    </option>
                  )}
                  <option value="jeden-wochentag">
                    Jeden Wochentag (Mo–Fr)
                  </option>
                  <option value="benutzerdefiniert">Benutzerdefiniert …</option>
                </select>
              </Field>
            )}

            {expanded && isSeriesFlow && (
              <>
                <div className="flex flex-col gap-1">
                  <span className="text-xs font-semibold text-gray-700">
                    Typ <span className={timetableRequiredMark}>*</span>
                  </span>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                    {TYPE_OPTIONS.map((option) => {
                      const isActive = form.type === option.value;
                      const color = getActivityColor(option.value);
                      return (
                        <button
                          key={option.value}
                          type="button"
                          onClick={() => update("type", option.value)}
                          className={`flex flex-col items-start gap-0.5 rounded-lg border border-l-[3px] px-3 py-2 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                            isActive
                              ? "border-gray-300 bg-white"
                              : "border-gray-200 bg-white hover:bg-gray-50"
                          }`}
                          style={{ borderLeftColor: color }}
                        >
                          <span
                            className="text-sm font-semibold"
                            style={{ color: isActive ? color : "#374151" }}
                          >
                            {option.label}
                          </span>
                          <span className="text-[10px] text-gray-500">
                            {option.hint}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div className="flex flex-col gap-1">
                  <span className="text-xs font-semibold text-gray-700">
                    Wochentage <span className={timetableRequiredMark}>*</span>
                  </span>
                  <div className="flex flex-wrap gap-1.5">
                    {WEEKDAYS.map((iso) => {
                      const isActive = form.weekdays.includes(iso);
                      return (
                        <Button
                          key={iso}
                          type="button"
                          variant={isActive ? "primary" : "outline"}
                          size="compact"
                          className="min-w-[44px]"
                          onClick={() => toggleWeekday(iso)}
                          aria-pressed={isActive}
                        >
                          {weekdayLabel(iso)}
                        </Button>
                      );
                    })}
                  </div>
                  {fieldErrors.weekdays && (
                    <p role="alert" className="mt-1 text-xs text-red-600">
                      {fieldErrors.weekdays}
                    </p>
                  )}
                </div>

                <Field
                  label="Kategorie"
                  htmlFor="event_category"
                  required
                  error={fieldErrors.categoryId}
                >
                  <select
                    id="event_category"
                    value={form.categoryId}
                    onChange={(event) =>
                      update("categoryId", event.target.value)
                    }
                    required
                    disabled={loadingRefs}
                    aria-invalid={fieldErrors.categoryId ? true : undefined}
                    aria-describedby={
                      fieldErrors.categoryId
                        ? "event_category_error"
                        : undefined
                    }
                    className={FORM_SELECT_CLASS}
                  >
                    <option value="">
                      {loadingRefs ? "Lade Kategorien …" : "Kategorie wählen …"}
                    </option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>
                        {category.name}
                      </option>
                    ))}
                  </select>
                </Field>

                <div className="flex flex-col gap-1">
                  <span className="text-xs font-semibold text-gray-700">
                    Zielgruppe
                  </span>
                  <Tabs
                    value={form.targetGroupType}
                    onValueChange={(value) =>
                      changeTargetGroupType(value as TargetGroupType)
                    }
                  >
                    <TabsList aria-label="Zielgruppe" className="w-fit">
                      {TARGET_GROUP_OPTIONS.map((option) => (
                        <TabsTrigger key={option.value} value={option.value}>
                          {option.label}
                        </TabsTrigger>
                      ))}
                    </TabsList>
                  </Tabs>

                  {form.targetGroupType === "jahrgang" && (
                    <div className="mt-1">
                      <Field
                        label="Jahrgang"
                        htmlFor="event_target_grade_level"
                        required
                        error={fieldErrors.targetGradeLevel}
                      >
                        <select
                          id="event_target_grade_level"
                          value={form.targetGradeLevel}
                          onChange={(event) =>
                            update("targetGradeLevel", event.target.value)
                          }
                          required
                          aria-invalid={
                            fieldErrors.targetGradeLevel ? true : undefined
                          }
                          aria-describedby={
                            fieldErrors.targetGradeLevel
                              ? "event_target_grade_level_error"
                              : undefined
                          }
                          className={FORM_SELECT_CLASS}
                        >
                          <option value="">Jahrgang wählen …</option>
                          {targetGradeOptions.map((option) => (
                            <option
                              key={option.value}
                              value={option.value}
                              disabled={option.disabled}
                            >
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </Field>
                      {preservesGradeAboveTenantCap ? (
                        <p className="mt-1 text-xs text-gray-500" role="status">
                          Jahrgang {form.targetGradeLevel} liegt über der
                          aktuell konfigurierten Höchststufe {gradeLevelMax}.
                          Die bestehende Zielgruppe bleibt beim Speichern
                          erhalten.
                        </p>
                      ) : null}
                    </div>
                  )}

                  {form.targetGroupType === "klasse" && (
                    <div className="mt-1">
                      <Field
                        label="Klasse"
                        htmlFor="event_target_school_class"
                        required
                        error={fieldErrors.targetSchoolClass}
                      >
                        <select
                          id="event_target_school_class"
                          value={form.targetSchoolClass}
                          onChange={(event) =>
                            update("targetSchoolClass", event.target.value)
                          }
                          disabled={
                            loadingStudents || studentLoadError !== null
                          }
                          required
                          aria-invalid={
                            fieldErrors.targetSchoolClass ? true : undefined
                          }
                          aria-describedby={
                            targetClassDescriptionIDs || undefined
                          }
                          className={FORM_SELECT_CLASS}
                        >
                          <option value="">Klasse wählen …</option>
                          {targetClassOptions.map((schoolClass) => (
                            <option key={schoolClass} value={schoolClass}>
                              {schoolClass}
                            </option>
                          ))}
                        </select>
                      </Field>
                      {loadingStudents || studentLoadError ? (
                        <p
                          id="event_target_school_class_availability"
                          className="mt-1 text-xs text-gray-500"
                          role="status"
                        >
                          {studentLoadError
                            ? "Die Klassenliste ist nicht verfügbar. Eine bestehende Klassen-Zielgruppe bleibt unverändert."
                            : "Klassenliste wird geladen …"}
                        </p>
                      ) : null}
                    </div>
                  )}

                  {form.targetGroupType === "gruppe" && (
                    <div className="mt-1">
                      <Field
                        label="Gruppe"
                        htmlFor="event_target_gruppe"
                        required
                        error={fieldErrors.educationGroupId}
                      >
                        <select
                          id="event_target_gruppe"
                          value={form.educationGroupId}
                          onChange={(event) =>
                            update("educationGroupId", event.target.value)
                          }
                          disabled={loadingRefs}
                          required
                          aria-invalid={
                            fieldErrors.educationGroupId ? true : undefined
                          }
                          aria-describedby={
                            fieldErrors.educationGroupId
                              ? "event_target_gruppe_error"
                              : undefined
                          }
                          className={FORM_SELECT_CLASS}
                        >
                          <option value="">Gruppe wählen …</option>
                          {groups.map((group) => (
                            <option key={group.id} value={group.id}>
                              {group.name}
                            </option>
                          ))}
                        </select>
                      </Field>
                    </div>
                  )}

                  {form.targetGroupType === "angebot" && (
                    <p className="mt-1 text-xs text-gray-500">
                      Kinder kommen automatisch über ein Betreuungsangebot
                      hinzu. Die Verknüpfung wird unter „Angebote“ beim
                      jeweiligen Angebot gepflegt (Feld „Regeltermin“).
                    </p>
                  )}

                  {targetCohort.label &&
                    !loadingStudents &&
                    !studentLoadError && (
                      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-xl border border-gray-200 bg-white/70 p-3">
                        <p className="min-w-0 flex-1 text-xs leading-5 text-gray-600">
                          Die Zielgruppe beschreibt den Regeltermin. Übernimm
                          die passenden Kinder mit einem Klick; die Auswahl
                          bleibt danach anpassbar.
                        </p>
                        <Button
                          type="button"
                          variant="outline"
                          size="compact"
                          onClick={addTargetCohort}
                          disabled={
                            targetCohort.memberIds.length === 0 ||
                            missingTargetCohortCount === 0
                          }
                        >
                          {targetCohortButtonLabel}
                        </Button>
                      </div>
                    )}
                </div>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {showPeriodField ? (
                    <Field
                      label="Planungszeitraum"
                      htmlFor="event_period"
                      required
                      error={fieldErrors.calendarPeriodId}
                    >
                      <select
                        id="event_period"
                        value={form.calendarPeriodId}
                        onChange={(event) =>
                          update("calendarPeriodId", event.target.value)
                        }
                        required
                        aria-invalid={
                          fieldErrors.calendarPeriodId ? true : undefined
                        }
                        aria-describedby={
                          fieldErrors.calendarPeriodId
                            ? "event_period_error"
                            : undefined
                        }
                        className={FORM_SELECT_CLASS}
                      >
                        <option value="">Zeitraum auswählen …</option>
                        {calendarPeriods.map((period) => (
                          <option key={period.id} value={period.id}>
                            {period.name}
                          </option>
                        ))}
                      </select>
                    </Field>
                  ) : (
                    <div className="flex flex-col justify-end gap-1">
                      <span className="text-xs font-semibold text-gray-700">
                        Planungszeitraum
                      </span>
                      <div className="flex h-10 items-center rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm text-gray-600">
                        <span className="truncate">
                          Gilt in{" "}
                          <span className="font-semibold text-gray-800">
                            {calendarPeriods.find(
                              (p) => p.id === form.calendarPeriodId,
                            )?.name ?? "dem aktuellen Planungszeitraum"}
                          </span>
                        </span>
                      </div>
                      {fieldErrors.calendarPeriodId && (
                        <p role="alert" className="mt-1 text-xs text-red-600">
                          {fieldErrors.calendarPeriodId}
                        </p>
                      )}
                    </div>
                  )}
                </div>
              </>
            )}

            {expanded ? (
              peopleFields
            ) : (
              <div className="flex flex-col gap-5">
                <button
                  type="button"
                  onClick={() => setMoreOpen((open) => !open)}
                  aria-expanded={moreOpen}
                  className="flex w-fit items-center gap-1.5 rounded-lg px-2 py-1.5 text-xs font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  <ChevronDown
                    className={`h-4 w-4 transition-transform ${moreOpen ? "rotate-180" : ""}`}
                    aria-hidden
                  />
                  Weitere Optionen
                </button>
                {moreOpen && peopleFields}
              </div>
            )}

            {isSeriesFlow && calendarPeriods.length === 0 && (
              <Alert
                type="error"
                message="Für diese Woche gibt es keinen aktiven Planungszeitraum. Lege zuerst oben im Plan einen Zeitraum an."
              />
            )}

            {validationError && (
              <Alert type="error" message={validationError} />
            )}
          </div>
        </form>

        {(conflictWarnings.length > 0 ||
          coverageWarnings.length > 0 ||
          coverageCheckError) && (
          // Advisory pre-save hints (QA M7): visible in quick and expanded
          // mode, pinned above the footer. Never disables Speichern.
          <div className="flex max-h-[40vh] flex-col gap-2 overflow-y-auto overscroll-contain border-t border-gray-200 px-5 py-3">
            <p className="sr-only" aria-live="polite">
              {`${conflictWarnings.length} Terminüberschneidungen und ${coverageWarningCount} Dienstplan-Lücken gefunden. Speichern ist weiterhin möglich.`}
            </p>
            {conflictWarnings.map((warning, index) => (
              <Alert
                key={`${warning.kind}-${warning.resourceId}-${index}`}
                type="warning"
                message={`Hinweis: ${warning.message}`}
                announce="off"
              />
            ))}
            {coverageWarningCount > 0 && (
              <Alert
                type="warning"
                message={`${coverageWarningCount} Dienstplan-${coverageWarningCount === 1 ? "Lücke" : "Lücken"} gefunden. Speichern ist weiterhin möglich.`}
                announce="off"
              />
            )}
            {coverageWarnings.slice(0, 3).map((warning, index) => (
              <p
                key={`shift-coverage-example-${warning.staffId}-${warning.date}-${warning.uncoveredStartTime}-${index}`}
                className="rounded-lg border border-yellow-100 bg-yellow-50/50 px-3 py-2 text-sm text-yellow-800"
              >
                {warning.message}
              </p>
            ))}
            {coverageWarningCount > 3 && (
              <details className="rounded-lg border border-yellow-100 bg-yellow-50/40 px-3 py-2 text-sm text-yellow-800">
                <summary className="cursor-pointer font-medium focus-visible:ring-2 focus-visible:ring-yellow-600 focus-visible:outline-none">
                  {coverageWarningCount - 3} weitere Lücken anzeigen
                </summary>
                <div className="mt-2 max-h-48 space-y-2 overflow-y-auto overscroll-contain pr-1">
                  {coverageWarnings.slice(3).map((warning, index) => (
                    <p
                      key={`shift-coverage-detail-${warning.staffId}-${warning.date}-${warning.uncoveredStartTime}-${index}`}
                      className="border-t border-yellow-100 pt-2 first:border-t-0 first:pt-0"
                    >
                      {warning.message}
                    </p>
                  ))}
                  {coverageWarningCount > coverageWarnings.length && (
                    <p className="border-t border-yellow-100 pt-2 font-medium">
                      Es werden höchstens 100 Beispiele angezeigt.
                    </p>
                  )}
                </div>
              </details>
            )}
            {coverageCheckError && (
              <Alert
                type="warning"
                message={`Hinweis: ${coverageCheckError}`}
                announce="off"
              />
            )}
          </div>
        )}

        <SlideOverFooter className="items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="order-2 sm:order-1">
            {canDeleteSeries && (
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                onClick={openSeriesDeleteConfirm}
                disabled={submitting || deletingSeries}
                className="w-full sm:w-auto"
              >
                <span className="inline-flex items-center gap-2">
                  <Trash2 className="h-4 w-4" aria-hidden />
                  Löschen
                </span>
              </Button>
            )}
          </div>
          <div className="order-1 flex items-center justify-end gap-3 sm:order-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={onClose}
              disabled={submitting || deletingSeries}
            >
              Abbrechen
            </Button>
            <Button
              type="submit"
              form="timetable-event-form"
              variant="primary"
              size="md"
              isLoading={submitting}
              loadingText="Speichere …"
              disabled={
                submitting ||
                deletingSeries ||
                (isEditingInstance && initialInstance?.status !== "planned")
              }
            >
              Speichern
            </Button>
          </div>
        </SlideOverFooter>

        {initialSeries && (
          <ConfirmationModal
            isOpen={deleteConfirmOpen}
            onClose={() => {
              if (!deletingSeries) setDeleteConfirmOpen(false);
            }}
            onConfirm={() => void handleConfirmSeriesDelete()}
            title="Regeltermin löschen?"
            confirmText="Löschen"
            cancelText="Abbrechen"
            isConfirmLoading={deletingSeries}
            confirmButtonClass="bg-red-600 hover:bg-red-700"
          >
            <div className="flex flex-col gap-4">
              <p className="text-sm leading-relaxed text-gray-600">
                Der Regeltermin wird ab dem gewählten Datum gelöscht. Frühere
                Termine bleiben erhalten.
              </p>
              <Field
                label="Ab Datum"
                htmlFor="series_delete_effective_date"
                required
                error={deleteError ?? undefined}
              >
                <Input
                  id="series_delete_effective_date"
                  type="date"
                  value={deleteEffectiveDate}
                  min={berlinTodayISO()}
                  controlSize="compact"
                  aria-invalid={deleteError ? true : undefined}
                  disabled={deletingSeries}
                  onChange={(event) => {
                    setDeleteEffectiveDate(event.target.value);
                    setDeleteError(null);
                  }}
                  required
                />
              </Field>
            </div>
          </ConfirmationModal>
        )}

        {initialInstance && (
          <ChoiceModal
            isOpen={choiceDialogOpen}
            onClose={() => setPendingSeriesEdit(null)}
            title="Wiederholenden Termin ändern"
            description={
              `Der Termin am ${formatDate(initialInstance.date)} gehört zu einem Regeltermin.` +
              (dateChanged || notesChanged
                ? " Geändertes Datum und Notiz gelten nur bei „Nur diese Woche“."
                : "")
            }
            options={[
              {
                value: "single",
                label: "Nur diese Woche",
                description: `Ändert nur den Termin am ${formatDate(form.date)}; der Regeltermin bleibt unverändert.`,
              },
              {
                value: "following",
                label: "Ab jetzt dauerhaft",
                description: `Ändert diesen und alle künftigen Termine ab dem ${formatDate(initialInstance.date)} dauerhaft; frühere Termine bleiben unverändert.`,
              },
              {
                value: "all",
                label: "Alle Termine der Serie",
                description:
                  "Ändert den Regeltermin und baut künftige geplante Termine neu auf.",
              },
            ]}
            onSelect={(value) => void handleScopeSelect(value)}
            isBusy={submitting}
          />
        )}
      </SlideOverContent>
    </SlideOver>
  );
}

function Field({
  label,
  htmlFor,
  required = false,
  error,
  children,
}: {
  label: string;
  htmlFor: string;
  required?: boolean;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={htmlFor} className="text-xs font-semibold text-gray-700">
        {label}
        {required && <span className={timetableRequiredMark}>*</span>}
      </label>
      {children}
      {error && (
        <p
          id={`${htmlFor}_error`}
          role="alert"
          className="mt-1 text-xs text-red-600"
        >
          {error}
        </p>
      )}
    </div>
  );
}

function MultiSelectField({
  label,
  options,
  value,
  onChange,
  metadata,
  bulkOptions,
}: {
  label: string;
  options: PersonOption[];
  value: string[];
  onChange: (value: string[]) => void;
  metadata: "student" | "staff";
  /**
   * Whole-cohort entries (e.g. "Klasse 1a") rendered as a select in the
   * action row; choosing one unions its memberIds into the selection.
   */
  bulkOptions?: Array<{ key: string; label: string; memberIds: string[] }>;
}) {
  const [query, setQuery] = useState("");
  const [gradeFilter, setGradeFilter] = useState("all");
  const [classFilter, setClassFilter] = useState("all");
  const [groupFilter, setGroupFilter] = useState("all");
  const selected = useMemo(() => new Set(value), [value]);
  const normalizedQuery = query.trim().toLocaleLowerCase("de");

  const classOptions = useMemo(
    () =>
      Array.from(
        new Set(
          options
            .map((option) => option.schoolClass?.trim())
            .filter((item): item is string => Boolean(item)),
        ),
      ).sort((a, b) => a.localeCompare(b, "de")),
    [options],
  );

  const gradeOptions = useMemo(
    () =>
      Array.from(
        new Set(
          options
            .map((option) => getSchoolYear(option.schoolClass?.trim() ?? ""))
            .filter((item): item is string => Boolean(item)),
        ),
      ).sort((a, b) => a.localeCompare(b, "de", { numeric: true })),
    [options],
  );

  const groupOptions = useMemo(
    () =>
      Array.from(
        new Set(
          options
            .map((option) => option.groupName?.trim())
            .filter((item): item is string => Boolean(item)),
        ),
      ).sort((a, b) => a.localeCompare(b, "de")),
    [options],
  );

  const filteredOptions = useMemo(
    () =>
      options.filter((option) => {
        const matchesQuery =
          normalizedQuery === "" ||
          [option.name, option.schoolClass, option.groupName]
            .filter(Boolean)
            .join(" ")
            .toLocaleLowerCase("de")
            .includes(normalizedQuery);
        const matchesClass =
          classFilter === "all" || option.schoolClass?.trim() === classFilter;
        const matchesGrade =
          gradeFilter === "all" ||
          getSchoolYear(option.schoolClass?.trim() ?? "") === gradeFilter;
        const matchesGroup =
          groupFilter === "all" || option.groupName?.trim() === groupFilter;
        return matchesQuery && matchesGrade && matchesClass && matchesGroup;
      }),
    [classFilter, gradeFilter, groupFilter, normalizedQuery, options],
  );

  const toggle = (id: string) => {
    const next = selected.has(id)
      ? value.filter((item) => item !== id)
      : [...value, id];
    onChange(next);
  };
  const visibleIds = filteredOptions.map((option) => option.id);
  const selectedVisibleCount = visibleIds.filter((id) =>
    selected.has(id),
  ).length;
  const allVisibleSelected =
    visibleIds.length > 0 && selectedVisibleCount === visibleIds.length;
  const hasFilters =
    query.trim() !== "" ||
    gradeFilter !== "all" ||
    classFilter !== "all" ||
    groupFilter !== "all";

  const selectVisible = () => {
    const next = Array.from(new Set([...value, ...visibleIds]));
    onChange(next);
  };

  const clearVisible = () => {
    const visibleSet = new Set(visibleIds);
    onChange(value.filter((id) => !visibleSet.has(id)));
  };

  return (
    <div className={`${timetableMutedSurface} flex flex-col gap-2 p-3`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-700">{label}</span>
        <span className="text-[11px] text-gray-500">
          {value.length} ausgewählt
        </span>
      </div>

      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-[1fr_auto_auto_auto]">
        <label className="relative">
          <span className="sr-only">{label} suchen</span>
          <Search
            className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400"
            aria-hidden
          />
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={`${label} suchen …`}
            className={FORM_SEARCH_CLASS}
          />
        </label>
        {metadata === "student" && gradeOptions.length > 0 && (
          <select
            value={gradeFilter}
            onChange={(event) => setGradeFilter(event.target.value)}
            className={FORM_SELECT_CLASS}
            aria-label="Nach Jahrgang filtern"
          >
            <option value="all">Alle Jahrgänge</option>
            {gradeOptions.map((gradeLevel) => (
              <option key={gradeLevel} value={gradeLevel}>
                Jahrgang {gradeLevel}
              </option>
            ))}
          </select>
        )}
        {metadata === "student" && classOptions.length > 0 && (
          <select
            value={classFilter}
            onChange={(event) => setClassFilter(event.target.value)}
            className={FORM_SELECT_CLASS}
            aria-label="Nach Klasse filtern"
          >
            <option value="all">Alle Klassen</option>
            {classOptions.map((schoolClass) => (
              <option key={schoolClass} value={schoolClass}>
                {schoolClass}
              </option>
            ))}
          </select>
        )}
        {metadata === "student" && groupOptions.length > 0 && (
          <select
            value={groupFilter}
            onChange={(event) => setGroupFilter(event.target.value)}
            className={FORM_SELECT_CLASS}
            aria-label="Nach Gruppe filtern"
          >
            <option value="all">Alle Gruppen</option>
            {groupOptions.map((groupName) => (
              <option key={groupName} value={groupName}>
                {groupName}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={allVisibleSelected ? clearVisible : selectVisible}
          disabled={visibleIds.length === 0}
          className="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          {allVisibleSelected ? "Sichtbare abwählen" : "Sichtbare auswählen"}
        </button>
        {bulkOptions && bulkOptions.length > 0 && (
          // Pinned to "" so picking an entry adds its members and the
          // select snaps back to the placeholder.
          <select
            value=""
            onChange={(event) => {
              const entry = bulkOptions.find(
                (option) => option.key === event.target.value,
              );
              if (!entry) return;
              onChange(Array.from(new Set([...value, ...entry.memberIds])));
            }}
            aria-label="Jahrgang, Klasse oder Gruppe komplett hinzufügen"
            className="moto-select rounded-lg border border-gray-200 bg-white py-1.5 pr-7 pl-2.5 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <option value="" disabled>
              Jahrgang/Klasse/Gruppe komplett hinzufügen …
            </option>
            {bulkOptions.map((option) => (
              <option key={option.key} value={option.key}>
                {option.label}
              </option>
            ))}
          </select>
        )}
        {value.length > 0 && (
          <button
            type="button"
            onClick={() => onChange([])}
            className="rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Auswahl leeren
          </button>
        )}
        {hasFilters && (
          <button
            type="button"
            onClick={() => {
              setQuery("");
              setGradeFilter("all");
              setClassFilter("all");
              setGroupFilter("all");
            }}
            className="rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Filter zurücksetzen
          </button>
        )}
      </div>

      <div className={`${timetableNestedSurface} max-h-72 overflow-y-auto p-2`}>
        {options.length === 0 ? (
          <div className="px-2 py-3 text-xs text-gray-500">
            Keine Einträge gefunden
          </div>
        ) : filteredOptions.length === 0 ? (
          <div className="px-2 py-3 text-xs text-gray-500">
            Keine passenden Einträge gefunden
          </div>
        ) : (
          <div className="grid gap-1 sm:grid-cols-2 lg:grid-cols-3">
            {filteredOptions.map((option) => (
              <label
                key={option.id}
                className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-gray-700 [contain-intrinsic-size:auto_36px] [content-visibility:auto] hover:bg-gray-50"
              >
                <Checkbox
                  checked={selected.has(option.id)}
                  onChange={() => toggle(option.id)}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{option.name}</span>
                  {(option.schoolClass || option.groupName) && (
                    <span className="block truncate text-[11px] text-gray-400">
                      {[option.schoolClass, option.groupName]
                        .filter(Boolean)
                        .join(" · ")}
                    </span>
                  )}
                </span>
              </label>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
