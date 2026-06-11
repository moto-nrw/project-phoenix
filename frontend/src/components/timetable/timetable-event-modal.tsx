"use client";

import { Search } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
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
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import { staffService } from "~/lib/staff-api";
import { timetableService } from "~/lib/timetable-api";
import {
  getActivityColor,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import type {
  ActivityType,
  CreateInstanceBody,
  CreateTemplateBody,
  EnrichedInstance,
  TimetableTemplate,
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
}

const logger = createLogger({ component: "TimetableEventModal" });
const FORM_SELECT_CLASS =
  "moto-select block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500";
const FORM_SEARCH_CLASS =
  "block h-10 w-full rounded-lg border-0 bg-white py-2 pr-3 pl-9 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400";

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

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
): EventFormState {
  const weekday = isoWeekday(defaultDate);
  return {
    title: "",
    date: defaultDate,
    startTime: "12:00",
    endTime: "13:00",
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
}: TimetableEventModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<EventFormState>(() =>
    emptyForm(defaultDate, defaultCalendarPeriodId, defaultRepeat),
  );
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [groups, setGroups] = useState<GroupOption[]>([]);
  const [students, setStudents] = useState<PersonOption[]>([]);
  const [staff, setStaff] = useState<PersonOption[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  const isEditingInstance = initialInstance !== null;
  const isEditingSeries = initialSeries !== null;
  const isConverting = convertInstance !== null;
  const isSeriesFlow = form.repeat !== "none" || isEditingSeries;

  useEffect(() => {
    if (!isOpen) return;
    setForm(
      initialSeries
        ? formFromSeries(initialSeries, defaultDate, defaultCalendarPeriodId)
        : convertInstance
          ? formFromInstance(convertInstance, defaultCalendarPeriodId, "weekly")
          : initialInstance
            ? formFromInstance(initialInstance, defaultCalendarPeriodId)
            : emptyForm(defaultDate, defaultCalendarPeriodId, defaultRepeat),
    );
    setValidationError(null);
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
    defaultRepeat,
    initialInstance,
    initialSeries,
    isOpen,
  ]);

  const update = <K extends keyof EventFormState>(
    key: K,
    value: EventFormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
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
  };

  const canSubmit = useMemo(() => {
    if (submitting) return false;
    if (form.title.trim() === "") return false;
    if (form.date === "" || form.startTime === "" || form.endTime === "") {
      return false;
    }
    if (form.roomId === "") return false;
    if (isSeriesFlow && form.categoryId === "") return false;
    if (isSeriesFlow && form.weekdays.length === 0) return false;
    if (isSeriesFlow && form.calendarPeriodId === "") return false;
    if (isEditingInstance && initialInstance?.status !== "planned")
      return false;
    return true;
  }, [
    form,
    initialInstance?.status,
    isEditingInstance,
    isSeriesFlow,
    submitting,
  ]);

  const validateShared = (): { roomId: number; categoryId?: number } | null => {
    if (form.endTime <= form.startTime) {
      setValidationError("Endzeit muss nach der Startzeit liegen.");
      return null;
    }
    const roomId = Number.parseInt(form.roomId, 10);
    if (!Number.isFinite(roomId) || roomId <= 0) {
      setValidationError("Bitte einen Raum auswählen.");
      return null;
    }
    if (!isSeriesFlow) return { roomId };
    const categoryId = Number.parseInt(form.categoryId, 10);
    if (!Number.isFinite(categoryId) || categoryId <= 0) {
      setValidationError("Bitte eine Kategorie auswählen.");
      return null;
    }
    const calendarPeriodId = Number.parseInt(form.calendarPeriodId, 10);
    if (!Number.isFinite(calendarPeriodId) || calendarPeriodId <= 0) {
      setValidationError("Bitte einen Planungszeitraum auswählen.");
      return null;
    }
    if (form.weekdays.length === 0) {
      setValidationError("Bitte mindestens einen Wochentag auswählen.");
      return null;
    }
    return { roomId, categoryId };
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

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!canSubmit) return;
    const parsed = validateShared();
    if (!parsed) return;

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
        setValidationError("Bitte eine Kategorie auswählen.");
        return;
      }

      if (initialSeries) {
        await timetableService.updateTemplate(
          initialSeries.id,
          seriesBody(parsed.roomId, parsed.categoryId),
        );
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
        if (weekFrom && weekTo) {
          await timetableService.materialize(weekFrom, weekTo);
        }
        toastSuccess("Termin wiederholt");
        onSaved({
          kind: "series",
          seriesId: created.templateId,
          linkedInstanceId: convertInstance.id,
        });
      } else {
        const created = await timetableService.createTemplate({
          ...seriesBody(parsed.roomId, parsed.categoryId),
          materialize_from: weekFrom,
          materialize_to: weekTo,
        });
        const count = created.instancesCreated ?? 0;
        toastSuccess(
          count > 0
            ? `Regeltermin angelegt: ${count} Termin${count === 1 ? "" : "e"} eingetragen`
            : "Regeltermin angelegt",
        );
        onSaved({ kind: "series", seriesId: created.templateId });
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

  const title = isEditingSeries
    ? "Regeltermin bearbeiten"
    : isEditingInstance
      ? "Termin bearbeiten"
      : isConverting
        ? "Termin wiederholen"
        : "Termin";

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[760px]">
        <SlideOverHeader>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <SlideOverTitle>{title}</SlideOverTitle>
              <SlideOverDescription>
                {isSeriesFlow
                  ? "Regelmäßigen Termin mit Kindern und Personal planen."
                  : "Einmaligen Termin im Dienstplan anlegen."}
              </SlideOverDescription>
            </div>
            <SlideOverCloseButton />
          </div>
        </SlideOverHeader>

        <form
          id="timetable-event-form"
          onSubmit={(event) => void handleSubmit(event)}
          className="flex-1 overflow-y-auto px-5 py-4"
        >
          <div className="flex flex-col gap-5">
            {initialInstance &&
              initialInstance.status !== "planned" &&
              renderModalErrorAlert({
                message: "Nur geplante Termine können bearbeitet werden.",
              })}

            <Field label="Titel" htmlFor="event_title" required>
              <Input
                id="event_title"
                value={form.title}
                onChange={(event) => update("title", event.target.value)}
                placeholder="z. B. Mensa, Lernzeit 1a, Yoga AG"
                maxLength={255}
                controlSize="compact"
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
                  onChange={(event) => {
                    const nextDate = event.target.value;
                    const nextWeekday = isoWeekday(nextDate);
                    update("date", nextDate);
                    if (!isSeriesFlow && nextWeekday >= 1 && nextWeekday <= 5) {
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
                  onChange={(event) => update("endTime", event.target.value)}
                  required
                />
              </Field>
            </div>

            <Field label="Raum" htmlFor="event_room" required>
              <select
                id="event_room"
                value={form.roomId}
                onChange={(event) => update("roomId", event.target.value)}
                disabled={loadingRefs}
                required
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

            {isSeriesFlow && (
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
                </div>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Field label="Kategorie" htmlFor="event_category" required>
                    <select
                      id="event_category"
                      value={form.categoryId}
                      onChange={(event) =>
                        update("categoryId", event.target.value)
                      }
                      required
                      disabled={loadingRefs}
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
                    >
                      <select
                        id="event_period"
                        value={form.calendarPeriodId}
                        onChange={(event) =>
                          update("calendarPeriodId", event.target.value)
                        }
                        required
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
                    </div>
                  )}
                </div>
              </>
            )}

            <MultiSelectField
              label="Kinder"
              options={students}
              value={form.studentIds}
              onChange={(ids) => update("studentIds", ids)}
              metadata="student"
            />

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
                  onChange={(event) =>
                    update("primaryStaffId", event.target.value)
                  }
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
            disabled={!canSubmit}
          >
            Speichern
          </Button>
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  );
}

function Field({
  label,
  htmlFor,
  required = false,
  children,
}: {
  label: string;
  htmlFor: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <label htmlFor={htmlFor} className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-gray-700">
        {label}
        {required && <span className="ml-0.5 text-[#FF3130]">*</span>}
      </span>
      {children}
    </label>
  );
}

function MultiSelectField({
  label,
  options,
  value,
  onChange,
  metadata,
}: {
  label: string;
  options: PersonOption[];
  value: string[];
  onChange: (value: string[]) => void;
  metadata: "student" | "staff";
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
