"use client";

import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
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
}

interface BackendRoomsEnvelope {
  data?: Array<{
    id: number;
    name: string;
    building?: string;
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
}

const logger = createLogger({ component: "TimetableEventModal" });

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
    notes: "",
    repeat: "none",
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
}: TimetableEventModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<EventFormState>(() =>
    emptyForm(defaultDate, defaultCalendarPeriodId),
  );
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
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
            : emptyForm(defaultDate, defaultCalendarPeriodId),
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
      fetchStudents({ page_size: 500 })
        .then((res) =>
          res.students.map((student) => ({
            id: student.id,
            name: student.name,
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
      .then(([roomData, categoryData, studentData, staffData]) => {
        const sortedRooms = [...roomData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        const sortedCategories = [...categoryData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        setRooms(sortedRooms);
        setCategories(sortedCategories);
        setStudents(studentData);
        setStaff(staffData);
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
      setValidationError("Bitte eine Planungsperiode auswählen.");
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
        toastSuccess("Serie gespeichert");
        onSaved({ kind: "series", seriesId: initialSeries.id });
        onClose();
        return;
      }

      const created = await timetableService.createTemplate({
        ...seriesBody(parsed.roomId, parsed.categoryId),
        materialize_from: weekFrom,
        materialize_to: weekTo,
      });

      if (convertInstance) {
        await timetableService.update(
          convertInstance.id,
          instanceBody(parsed.roomId, created.templateId),
        );
        toastSuccess("Termin wiederholt");
        onSaved({
          kind: "series",
          seriesId: created.templateId,
          linkedInstanceId: convertInstance.id,
        });
      } else {
        const count = created.instancesCreated ?? 0;
        toastSuccess(
          count > 0
            ? `Serientermin angelegt - ${count} Termin${count === 1 ? "" : "e"} eingeplant`
            : "Serientermin angelegt",
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
    ? "Serie bearbeiten"
    : isEditingInstance
      ? "Termin bearbeiten"
      : isConverting
        ? "Termin wiederholen"
        : "Termin";

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="timetable-event-form"
            variant="primary"
            size="sm"
            isLoading={submitting}
            loadingText="Speichere ..."
            disabled={!canSubmit}
          >
            Speichern
          </Button>
        </div>
      }
    >
      <form
        id="timetable-event-form"
        onSubmit={(event) => void handleSubmit(event)}
        className="flex flex-col gap-4"
      >
        {initialInstance && initialInstance.status !== "planned" && (
          <div className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]">
            Nur geplante Termine können bearbeitet werden.
          </div>
        )}

        <Field label="Titel" htmlFor="event_title" required>
          <Input
            id="event_title"
            value={form.title}
            onChange={(event) => update("title", event.target.value)}
            placeholder="z. B. Mensa, Lernzeit 1a, Yoga AG"
            maxLength={255}
            autoFocus
            required
          />
        </Field>

        <div className="grid grid-cols-3 gap-3">
          <Field label="Datum" htmlFor="event_date" required>
            <Input
              id="event_date"
              type="date"
              value={form.date}
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
              onChange={(event) => update("startTime", event.target.value)}
              required
            />
          </Field>
          <Field label="Ende" htmlFor="event_end" required>
            <Input
              id="event_end"
              type="time"
              value={form.endTime}
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
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-100"
          >
            <option value="">
              {loadingRefs ? "Lade Räume ..." : "Raum auswählen ..."}
            </option>
            {rooms.map((room) => (
              <option key={room.id} value={room.id}>
                {room.building ? `${room.building} - ${room.name}` : room.name}
              </option>
            ))}
          </select>
        </Field>

        <div className="flex flex-col gap-1">
          <span className="text-xs font-semibold text-slate-700">
            Wiederholen
          </span>
          <div className="inline-flex rounded-md bg-slate-100 p-0.5">
            {REPEAT_OPTIONS.map((option) => {
              const isActive = form.repeat === option.value;
              const disabled = isEditingSeries && option.value === "none";
              return (
                <button
                  key={option.value}
                  type="button"
                  disabled={disabled}
                  onClick={() => {
                    update("repeat", option.value);
                    if (option.value !== "none" && form.weekdays.length === 0) {
                      const weekday = isoWeekday(form.date);
                      update(
                        "weekdays",
                        weekday >= 1 && weekday <= 5 ? [weekday] : [1],
                      );
                    }
                  }}
                  className={`rounded px-3 py-1.5 text-[13px] font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${
                    isActive
                      ? "bg-white text-slate-900 shadow-sm"
                      : "text-slate-600 hover:text-slate-900"
                  }`}
                >
                  {option.label}
                </button>
              );
            })}
          </div>
        </div>

        {isSeriesFlow && (
          <>
            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold text-slate-700">
                Typ <span className="ml-0.5 text-[#FF3130]">*</span>
              </span>
              <div className="grid grid-cols-3 gap-2">
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
                          : "border border-slate-200 bg-slate-50 hover:bg-white"
                      }`}
                      style={isActive ? { borderColor: color } : undefined}
                    >
                      <span
                        className="text-sm font-semibold"
                        style={{ color: isActive ? color : "#374151" }}
                      >
                        {option.label}
                      </span>
                      <span className="text-[10px] text-slate-500">
                        {option.hint}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold text-slate-700">
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
                          ? "border-[#5080D8] bg-[#5080D8] text-white"
                          : "border-slate-300 bg-white text-slate-600 hover:bg-slate-50"
                      }`}
                      aria-pressed={isActive}
                    >
                      {weekdayLabel(iso)}
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <Field label="Kategorie" htmlFor="event_category" required>
                <select
                  id="event_category"
                  value={form.categoryId}
                  onChange={(event) => update("categoryId", event.target.value)}
                  required
                  disabled={loadingRefs}
                  className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-100"
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
              {showPeriodField ? (
                <Field label="Planungsperiode" htmlFor="event_period" required>
                  <select
                    id="event_period"
                    value={form.calendarPeriodId}
                    onChange={(event) =>
                      update("calendarPeriodId", event.target.value)
                    }
                    required
                    className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
                  >
                    <option value="">Periode auswählen ...</option>
                    {calendarPeriods.map((period) => (
                      <option key={period.id} value={period.id}>
                        {period.name}
                      </option>
                    ))}
                  </select>
                </Field>
              ) : (
                <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
                  Gilt in{" "}
                  <span className="font-semibold">
                    {calendarPeriods.find((p) => p.id === form.calendarPeriodId)
                      ?.name ?? "der aktuellen Planungsperiode"}
                  </span>
                  .
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
        />

        {isSeriesFlow && form.staffIds.length > 0 && (
          <Field label="Hauptverantwortlich" htmlFor="event_primary_staff">
            <select
              id="event_primary_staff"
              value={form.primaryStaffId}
              onChange={(event) => update("primaryStaffId", event.target.value)}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
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
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none"
            />
          </Field>
        )}

        {isSeriesFlow && calendarPeriods.length === 0 && (
          <div
            role="alert"
            className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]"
          >
            Für diese Woche gibt es keine aktive Planungsperiode. Lege zuerst
            eine Periode im Kopfbereich an.
          </div>
        )}

        {validationError && (
          <div
            role="alert"
            className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]"
          >
            {validationError}
          </div>
        )}
      </form>
    </Modal>
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
      <span className="text-xs font-semibold text-slate-700">
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
}: {
  label: string;
  options: PersonOption[];
  value: string[];
  onChange: (value: string[]) => void;
}) {
  const selected = new Set(value);
  const toggle = (id: string) => {
    const next = selected.has(id)
      ? value.filter((item) => item !== id)
      : [...value, id];
    onChange(next);
  };

  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-slate-700">{label}</span>
      <div className="max-h-40 overflow-y-auto rounded-lg border border-slate-200 bg-white p-2">
        {options.length === 0 ? (
          <div className="px-2 py-3 text-xs text-slate-500">
            Keine Einträge gefunden
          </div>
        ) : (
          <div className="grid gap-1 sm:grid-cols-2">
            {options.map((option) => (
              <label
                key={option.id}
                className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm text-slate-700 hover:bg-slate-50"
              >
                <input
                  type="checkbox"
                  checked={selected.has(option.id)}
                  onChange={() => toggle(option.id)}
                  className="h-4 w-4 rounded border-slate-300 text-[#5080D8] focus:ring-[#5080D8]"
                />
                <span className="min-w-0 truncate">{option.name}</span>
              </label>
            ))}
          </div>
        )}
      </div>
      {value.length > 0 && (
        <span className="text-[11px] text-slate-500">
          {value.length} ausgewählt
        </span>
      )}
    </div>
  );
}
