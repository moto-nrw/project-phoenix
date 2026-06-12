"use client";

import { ChevronDown, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { useModal } from "~/components/dashboard/modal-context";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { Input } from "~/components/ui/input";
import { renderModalErrorAlert } from "~/components/ui/modal-utils";
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
import { formatDate, todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import { staffService } from "~/lib/staff-api";
import { timetableService } from "~/lib/timetable-api";
import { useDebounce } from "~/lib/use-debounce";
import {
  chunkDateRange,
  getActivityColor,
  getGermanWeekdayLong,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import type {
  ActivityType,
  ConflictWarningItem,
  CreateInstanceBody,
  CreateTemplateBody,
  EnrichedInstance,
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
  groupName?: string;
}

interface GroupOption {
  id: string;
  name: string;
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
  weekdays: number[];
  calendarPeriodId: string;
  studentIds: string[];
  staffIds: string[];
  primaryStaffId: string;
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
  defaultRepeat?: RepeatMode;
  /**
   * "quick" starts collapsed with only Titel, Datum/Zeiten, Raum and a
   * plain-language repeat select (US-1 quick create). "Benutzerdefiniert"
   * in that select expands to the full form. Default "full".
   */
  variant?: "full" | "quick";
  defaultStartTime?: string;
  defaultEndTime?: string;
}

const logger = createLogger({ component: "TimetableEventModal" });
const FORM_SELECT_CLASS =
  "moto-select block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500";
const FORM_SEARCH_CLASS =
  "block h-10 w-full rounded-lg border-0 bg-white py-2 pr-3 pl-9 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400";

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

/**
 * Backend cap for a single materialization window
 * (MaxMaterializationWindowDays in services/schedule/materialization_service.go).
 * Whole-period runs are split into windows of this size.
 */
const MATERIALIZE_CHUNK_DAYS = 56;

/** Plain-language repeat presets shown in the quick variant. */
type QuickRepeatPreset =
  | "einmalig"
  | "woechentlich-am"
  | "jeden-wochentag"
  | "benutzerdefiniert";

const TYPE_OPTIONS: Array<{
  value: ActivityType;
  label: string;
  hint: string;
}> = [
  { value: "care", label: "Betreuung", hint: "Mensa, Lernzeit, Freispiel" },
  { value: "activity", label: "AG", hint: "Yoga, Bouldern, ..." },
  { value: "external", label: "Extern", hint: "DAZ, Musikschule" },
];

const REPEAT_OPTIONS: Array<{ value: RepeatMode; label: string }> = [
  { value: "none", label: "Nie" },
  { value: "weekly", label: "Jede Woche" },
  { value: "biweekly", label: "Alle 2 Wochen" },
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
    weekdays: weekday >= 1 && weekday <= 5 ? [weekday] : [1],
    calendarPeriodId: defaultCalendarPeriodId ?? "",
    studentIds: [],
    staffIds: [],
    primaryStaffId: "",
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
    weekdays: weekday >= 1 && weekday <= 5 ? [weekday] : [1],
    calendarPeriodId: defaultCalendarPeriodId ?? "",
    studentIds: instance.studentIds,
    staffIds: instance.staff.map((item) => item.staffId),
    primaryStaffId:
      instance.staff.find((item) => item.isPrimary)?.staffId ?? "",
  };
}

function formFromSeries(
  series: TimetableTemplate,
  defaultDate: string,
  defaultCalendarPeriodId?: string | null,
): EventFormState {
  const firstSchedule = series.schedules[0];
  const weekdays = series.schedules.map((schedule) => schedule.weekday);
  const repeat = firstSchedule?.weekPattern === 2 ? "biweekly" : "weekly";
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
    weekdays: weekdays.length > 0 ? weekdays : [1],
    calendarPeriodId:
      firstSchedule?.calendarPeriodId ?? defaultCalendarPeriodId ?? "",
    studentIds: series.studentIds,
    staffIds: series.staffIds,
    primaryStaffId: series.primaryStaffId ?? "",
  };
}

function seriesWeekPattern(repeat: RepeatMode): number {
  return repeat === "biweekly" ? 2 : 0;
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
  defaultRepeat = "none",
  variant = "full",
  defaultStartTime,
  defaultEndTime,
}: TimetableEventModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
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
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [groups, setGroups] = useState<GroupOption[]>([]);
  const [students, setStudents] = useState<PersonOption[]>([]);
  const [staff, setStaff] = useState<PersonOption[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [expanded, setExpanded] = useState(variant === "full");
  const [moreOpen, setMoreOpen] = useState(false);
  // Validated room id stashed while the Dreifach-Frage dialog (US-5) is open.
  const [pendingSeriesEdit, setPendingSeriesEdit] = useState<{
    roomId: number;
  } | null>(null);
  const [conflictWarnings, setConflictWarnings] = useState<
    ConflictWarningItem[]
  >([]);
  // Monotonically increasing probe id so stale responses are dropped.
  const probeSeq = useRef(0);

  const isEditingInstance = initialInstance !== null;
  const isEditingSeries = initialSeries !== null;
  const isConverting = convertInstance !== null;
  const isSeriesFlow = form.repeat !== "none" || isEditingSeries;
  const choiceDialogOpen = pendingSeriesEdit !== null;

  useEffect(() => {
    if (!isOpen) return;
    setForm(
      initialSeries
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
              ),
    );
    setValidationError(null);
    setFieldErrors({});
    setExpanded(variant === "full");
    setMoreOpen(false);
    setPendingSeriesEdit(null);
    setConflictWarnings([]);
    setLoadingRefs(true);

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
      fetchStudents({ page_size: 500 })
        .then((res) =>
          res.students.map((student) => ({
            id: student.id,
            name: student.name,
            schoolClass: student.school_class,
            groupName: student.group_name,
          })),
        )
        .catch((err: unknown) => {
          logger.error("students_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as PersonOption[];
        }),
      staffService
        .getAllStaff()
        .then((items) => items.map((s) => ({ id: s.id, name: s.name })))
        .catch((err: unknown) => {
          logger.error("staff_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as PersonOption[];
        }),
    ])
      .then(([roomData, categoryData, groupData, studentData, staffData]) => {
        const sortedRooms = [...roomData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        const sortedCategories = [...categoryData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        const sortedGroups = [...groupData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        setRooms(sortedRooms);
        setCategories(sortedCategories);
        setGroups(sortedGroups);
        setStudents(sortPeople(studentData));
        setStaff(sortPeople(staffData));
        setForm((prev) =>
          prev.categoryId || sortedCategories.length === 0
            ? prev
            : { ...prev, categoryId: sortedCategories[0]?.id ?? "" },
        );
      })
      .finally(() => setLoadingRefs(false));
  }, [
    convertInstance,
    defaultCalendarPeriodId,
    defaultDate,
    defaultEndTime,
    defaultRepeat,
    defaultStartTime,
    initialInstance,
    initialSeries,
    isOpen,
    variant,
  ]);

  // ---------------------------------------------------------------------
  // Inline conflict probe (QA M7). Advisory only — warnings never disable
  // Speichern. For series (weekly) drafts the backend check is date-based,
  // so we probe with the form's single date (the instance date / first
  // occurrence); acceptable for Phase 3.
  // ---------------------------------------------------------------------
  const probeKey = JSON.stringify({
    date: form.date,
    startTime: form.startTime,
    endTime: form.endTime,
    roomId: form.roomId,
    staffIds: form.staffIds,
    studentIds: form.studentIds,
  });
  const debouncedProbeKey = useDebounce(probeKey, 500);
  const excludeInstanceId = initialInstance?.id;

  useEffect(() => {
    if (!isOpen) return;
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
        if (probeSeq.current === seq) setConflictWarnings([]);
      });
  }, [debouncedProbeKey, excludeInstanceId, isOpen]);

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
    }
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) {
      // Quick mode hides the series controls; expand so an inline error on
      // a hidden field (Kategorie, Planungszeitraum, Wochentage) is visible.
      if (
        !expanded &&
        (errors.categoryId ?? errors.calendarPeriodId ?? errors.weekdays)
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
        update("repeat", "none");
        break;
      case "woechentlich-am":
        update("repeat", "weekly");
        update(
          "weekdays",
          dateWeekday >= 1 && dateWeekday <= 5 ? [dateWeekday] : [1],
        );
        break;
      case "jeden-wochentag":
        update("repeat", "weekly");
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
      staff_ids: form.staffIds.map(Number),
      student_ids: form.studentIds.map(Number),
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
    calendar_period_id: Number(form.calendarPeriodId),
    week_pattern: seriesWeekPattern(form.repeat),
    student_ids: form.studentIds.map(Number),
    staff_ids: form.staffIds.map(Number),
    primary_staff_id: form.primaryStaffId
      ? Number(form.primaryStaffId)
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
    const weekdays = template.schedules.map((schedule) => schedule.weekday);
    const primaryStaffId = form.primaryStaffId || template.primaryStaffId;
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
      max_participants:
        template.maxParticipants > 0 ? template.maxParticipants : undefined,
      week_pattern: firstSchedule?.weekPattern ?? 0,
      calendar_period_id: firstSchedule?.calendarPeriodId
        ? Number(firstSchedule.calendarPeriodId)
        : form.calendarPeriodId
          ? Number(form.calendarPeriodId)
          : undefined,
      student_ids: form.studentIds.map(Number),
      staff_ids: form.staffIds.map(Number),
      primary_staff_id:
        primaryStaffId && form.staffIds.includes(primaryStaffId)
          ? Number(primaryStaffId)
          : undefined,
    };
  };

  /**
   * Rebuilds a template's future planned instances: chunked scoped
   * re-plan from today through the period end (each window stays within
   * the backend's 56-day materialization cap). Returns the created count.
   */
  const replanTemplateFuture = async (
    templateId: string,
    periodEndISO?: string,
  ): Promise<number> => {
    const endISO =
      periodEndISO ?? findPeriod(form.calendarPeriodId)?.endDate ?? weekTo;
    if (!endISO) return 0;
    const chunks = chunkDateRange(todayISO(), endISO, MATERIALIZE_CHUNK_DAYS);
    let created = 0;
    for (const chunk of chunks) {
      const result = await timetableService.replanWeek(
        chunk.from,
        chunk.to,
        templateId,
      );
      created += result.instancesCreated;
      // A precondition like "no_active_period" applies to every chunk.
      if (result.warnings.some((w) => w.code === "no_active_period")) break;
    }
    return created;
  };

  /**
   * Creates the template materializing the whole selected period (US-1
   * Phase 3). The backend caps one materialization window at 56 days, so
   * the create call carries only the first chunk; the rest follow as
   * separate materialize calls. Falls back to the visible week when no
   * period is resolvable.
   */
  const createSeriesForPeriod = async (
    body: CreateTemplateBody,
  ): Promise<{ templateId: string; totalCreated: number }> => {
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
      };
    }
    const created = await timetableService.createTemplate({
      ...body,
      materialize_from: firstChunk.from,
      materialize_to: firstChunk.to,
    });
    let totalCreated = created.instancesCreated ?? 0;
    for (const chunk of restChunks) {
      const result = await timetableService.materialize(chunk.from, chunk.to);
      totalCreated += result.instancesCreated;
      if (result.warnings.some((w) => w.code === "no_active_period")) break;
    }
    return { templateId: created.templateId, totalCreated };
  };

  /** Materializes the whole selected period after linking a converted instance. */
  const materializePeriodAfterConvert = async (): Promise<void> => {
    const period = findPeriod(form.calendarPeriodId);
    const chunks = period
      ? chunkDateRange(period.startDate, period.endDate, MATERIALIZE_CHUNK_DAYS)
      : [];
    if (chunks.length === 0) {
      if (weekFrom && weekTo) {
        await timetableService.materialize(weekFrom, weekTo);
      }
      return;
    }
    for (const chunk of chunks) {
      const result = await timetableService.materialize(chunk.from, chunk.to);
      if (result.warnings.some((w) => w.code === "no_active_period")) break;
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
        await replanTemplateFuture(initialSeries.id);
        toastSuccess("Regeltermin gespeichert");
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
        await materializePeriodAfterConvert();
        toastSuccess("Termin wiederholt");
        onSaved({
          kind: "series",
          seriesId: created.templateId,
          linkedInstanceId: convertInstance.id,
        });
      } else {
        const { templateId, totalCreated } = await createSeriesForPeriod(
          seriesBody(parsed.roomId, parsed.categoryId),
        );
        toastSuccess(
          totalCreated > 0
            ? `Regeltermin angelegt: ${totalCreated} Termin${totalCreated === 1 ? "" : "e"} eingetragen`
            : "Regeltermin angelegt",
        );
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
  const handleScopeSelect = async (scope: string) => {
    if (submitting) return;
    const pending = pendingSeriesEdit;
    const groupId = initialInstance?.activityGroupId;
    if (!pending || !initialInstance || !groupId) return;
    setSubmitting(true);
    try {
      if (scope === "single") {
        const saved = await timetableService.update(
          initialInstance.id,
          instanceBody(pending.roomId, groupId),
        );
        toastSuccess("Termin gespeichert");
        onSaved({ kind: "instance", instance: saved });
      } else {
        const template = await timetableService.getTemplate(groupId);
        const periodEnd =
          findPeriod(template.schedules[0]?.calendarPeriodId)?.endDate ??
          findPeriod(form.calendarPeriodId)?.endDate;
        const body = templateBodyFromForm(template, pending.roomId);
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
          for (const chunk of chunks.slice(1)) {
            const result = await timetableService.replanWeek(
              chunk.from,
              chunk.to,
              groupId,
            );
            if (result.warnings.some((w) => w.code === "no_active_period")) {
              break;
            }
          }
          toastSuccess(`Regeltermin ab ${formatDate(effectiveDate)} geändert`);
          onSaved({ kind: "series", seriesId: split.newTemplateId });
        } else {
          await timetableService.updateTemplate(groupId, body);
          await replanTemplateFuture(groupId, periodEnd);
          toastSuccess("Regeltermin gespeichert");
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
      const msg =
        err instanceof Error
          ? err.message
          : "Termin konnte nicht gespeichert werden";
      setPendingSeriesEdit(null);
      setValidationError(msg);
      toastError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  // Bulk-add entries for the Kinder field: every distinct class and group
  // present in the loaded students, each carrying its member ids.
  const studentBulkOptions = useMemo(() => {
    const classes = new Map<string, string[]>();
    const groupNames = new Map<string, string[]>();
    for (const student of students) {
      const schoolClass = student.schoolClass?.trim();
      if (schoolClass) {
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
      ...[...classes.entries()].sort(byName).map(([name, memberIds]) => ({
        key: `class:${name}`,
        label: `Klasse ${name}`,
        memberIds,
      })),
      ...[...groupNames.entries()].sort(byName).map(([name, memberIds]) => ({
        key: `group:${name}`,
        label: `Gruppe ${name}`,
        memberIds,
      })),
    ];
  }, [students]);

  const dateChanged =
    initialInstance !== null && form.date !== initialInstance.date;

  const title = isEditingSeries
    ? "Regeltermin bearbeiten"
    : isEditingInstance
      ? "Termin bearbeiten"
      : isConverting
        ? "Termin wiederholen"
        : "Termin";

  // Personal renders before Kinder in every mode (Streichliste 8); the
  // quick variant tucks all of this behind the "Weitere Optionen" row.
  const peopleFields = (
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

      <MultiSelectField
        label="Kinder"
        options={students}
        value={form.studentIds}
        onChange={(ids) => update("studentIds", ids)}
        metadata="student"
        bulkOptions={studentBulkOptions}
      />

      {!isSeriesFlow && (
        <Field label="Notiz" htmlFor="event_notes">
          <textarea
            id="event_notes"
            value={form.notes}
            onChange={(event) => update("notes", event.target.value)}
            rows={3}
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-200 focus:outline-none"
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
        // fire. See issue #1358.
        onInteractOutside={(event) => {
          if (isModalOpen || choiceDialogOpen) event.preventDefault();
        }}
        onEscapeKeyDown={(event) => {
          if (isModalOpen || choiceDialogOpen) event.preventDefault();
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
            {initialInstance &&
              initialInstance.status !== "planned" &&
              renderModalErrorAlert({
                message: "Nur geplante Termine können bearbeitet werden.",
              })}

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
                  {loadingRefs ? "Lade Räume ..." : "Raum auswählen ..."}
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
                  Wiederholen
                </span>
                <Tabs
                  value={form.repeat}
                  onValueChange={(value) => {
                    const nextRepeat = value as RepeatMode;
                    update("repeat", nextRepeat);
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
                  <option value="woechentlich-am">
                    {`Wöchentlich am ${dateWeekdayName}`}
                  </option>
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
                    Typ <span className="ml-0.5 text-[#FF3130]">*</span>
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
                          className={`flex flex-col items-start gap-0.5 rounded-md border px-3 py-2 text-left transition-colors ${
                            isActive
                              ? "border-2 bg-white"
                              : "border border-gray-200 bg-gray-50 hover:bg-white"
                          }`}
                          style={isActive ? { borderColor: color } : undefined}
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
                    Wochentage <span className="ml-0.5 text-[#FF3130]">*</span>
                  </span>
                  <div className="flex flex-wrap gap-1.5">
                    {WEEKDAYS.map((iso) => {
                      const isActive = form.weekdays.includes(iso);
                      return (
                        <button
                          key={iso}
                          type="button"
                          onClick={() => toggleWeekday(iso)}
                          className={`min-w-[44px] rounded-md border px-3 py-1.5 text-sm font-semibold transition-colors ${
                            isActive
                              ? "border-gray-900 bg-gray-900 text-white"
                              : "border-gray-300 bg-white text-gray-600 hover:bg-gray-50"
                          }`}
                          aria-pressed={isActive}
                        >
                          {weekdayLabel(iso)}
                        </button>
                      );
                    })}
                  </div>
                  {fieldErrors.weekdays && (
                    <p role="alert" className="mt-1 text-xs text-red-600">
                      {fieldErrors.weekdays}
                    </p>
                  )}
                </div>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
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
                        {loadingRefs
                          ? "Lade Kategorien ..."
                          : "Kategorie wählen ..."}
                      </option>
                      {categories.map((category) => (
                        <option key={category.id} value={category.id}>
                          {category.name}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Klassengruppe" htmlFor="event_education_group">
                    <select
                      id="event_education_group"
                      value={form.educationGroupId}
                      onChange={(event) =>
                        update("educationGroupId", event.target.value)
                      }
                      disabled={loadingRefs}
                      className={FORM_SELECT_CLASS}
                    >
                      <option value="">Keine Zuordnung</option>
                      {groups.map((group) => (
                        <option key={group.id} value={group.id}>
                          {group.name}
                        </option>
                      ))}
                    </select>
                  </Field>
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
                        <option value="">Zeitraum auswählen ...</option>
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
                  className="flex w-fit items-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900"
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

            {isSeriesFlow &&
              calendarPeriods.length === 0 &&
              renderModalErrorAlert({
                message:
                  "Für diese Woche gibt es keinen aktiven Planungszeitraum. Lege zuerst oben im Plan einen Zeitraum an.",
              })}

            {validationError &&
              renderModalErrorAlert({ message: validationError })}
          </div>
        </form>

        {conflictWarnings.length > 0 && (
          // Advisory pre-save hints (QA M7): visible in quick and expanded
          // mode, pinned above the footer. Never disables Speichern.
          <div className="flex flex-col gap-2 border-t border-slate-200 px-5 py-3">
            {conflictWarnings.map((warning, index) => (
              <Alert
                key={`${warning.kind}-${warning.resourceId}-${index}`}
                type="warning"
                message={`Hinweis: ${warning.message}`}
              />
            ))}
          </div>
        )}

        <SlideOverFooter className="flex-row items-center justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="timetable-event-form"
            variant="primary"
            size="md"
            isLoading={submitting}
            loadingText="Speichere ..."
            disabled={
              submitting ||
              (isEditingInstance && initialInstance?.status !== "planned")
            }
          >
            Speichern
          </Button>
        </SlideOverFooter>

        {initialInstance && (
          <ChoiceModal
            isOpen={choiceDialogOpen}
            onClose={() => setPendingSeriesEdit(null)}
            title="Wiederholenden Termin ändern"
            description={
              `Der Termin am ${formatDate(initialInstance.date)} gehört zu einem Regeltermin.` +
              (dateChanged
                ? ' Das geänderte Datum gilt nur bei "Nur dieser Termin".'
                : "")
            }
            options={[
              {
                value: "single",
                label: "Nur dieser Termin",
                description: `Die Änderung gilt nur am ${formatDate(form.date)}.`,
              },
              {
                value: "following",
                label: "Dieser und alle folgenden",
                description: `Teilt den Regeltermin ab ${formatDate(initialInstance.date)}; frühere Termine bleiben unverändert.`,
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
        {required && <span className="ml-0.5 text-[#FF3130]">*</span>}
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
          classFilter === "all" || option.schoolClass === classFilter;
        const matchesGroup =
          groupFilter === "all" || option.groupName === groupFilter;
        return matchesQuery && matchesClass && matchesGroup;
      }),
    [classFilter, groupFilter, normalizedQuery, options],
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
    query.trim() !== "" || classFilter !== "all" || groupFilter !== "all";

  const selectVisible = () => {
    const next = Array.from(new Set([...value, ...visibleIds]));
    onChange(next);
  };

  const clearVisible = () => {
    const visibleSet = new Set(visibleIds);
    onChange(value.filter((id) => !visibleSet.has(id)));
  };

  return (
    <div className="flex flex-col gap-2 rounded-2xl border border-gray-200 bg-gray-50 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-700">{label}</span>
        <span className="text-[11px] text-gray-500">
          {value.length} ausgewählt
        </span>
      </div>

      <div className="grid gap-2 sm:grid-cols-[1fr_auto_auto]">
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
            placeholder={`${label} suchen ...`}
            className={FORM_SEARCH_CLASS}
          />
        </label>
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
          className="rounded-md border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50"
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
            aria-label="Klasse oder Gruppe komplett hinzufügen"
            className="moto-select rounded-md border border-gray-200 bg-white py-1.5 pr-7 pl-2.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-100 focus:outline-none"
          >
            <option value="" disabled>
              Klasse/Gruppe komplett hinzufügen …
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
            className="rounded-md px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800"
          >
            Auswahl leeren
          </button>
        )}
        {hasFilters && (
          <button
            type="button"
            onClick={() => {
              setQuery("");
              setClassFilter("all");
              setGroupFilter("all");
            }}
            className="rounded-md px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800"
          >
            Filter zurücksetzen
          </button>
        )}
      </div>

      <div className="max-h-72 overflow-y-auto rounded-2xl border border-gray-200 bg-white p-2 shadow-sm">
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
                className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm text-gray-700 hover:bg-gray-50"
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
