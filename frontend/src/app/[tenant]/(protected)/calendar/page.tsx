"use client";

import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { CalendarPlus, Trash2 } from "lucide-react";

import { PersonalCalendar } from "~/components/calendar/personal-calendar";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { toISODate } from "~/lib/date-helpers";
import {
  createStaffAppointment,
  getCalendarRecipientOptions,
  getStaffCalendar,
  respondStaffCalendar,
  type CalendarDeliveryMode,
  type CalendarRecipientOptions,
  type CalendarResponse,
  type CalendarTarget,
  type CalendarTargetType,
} from "~/lib/personal-calendar-api";
import { useSWRAuth } from "~/lib/swr";
import { getWeekRange } from "~/lib/timetable-helpers";

type RecurrenceFrequency = "none" | "daily" | "weekly" | "monthly" | "yearly";

interface DraftTarget extends CalendarTarget {
  readonly key: string;
  readonly label: string;
}

interface Choice {
  readonly id?: number;
  readonly value?: string;
  readonly label: string;
}

const emptyRecipientOptions: CalendarRecipientOptions = {
  staff: [],
  parents: [],
  groups: [],
  classes: [],
  students: [],
};

const targetTypeLabels: Record<CalendarTargetType, string> = {
  staff: "Mitarbeiter",
  guardian_profile: "Elternteil",
  all_staff: "Alle Mitarbeiter",
  parents_by_class: "Eltern einer Klasse",
  parents_by_group: "Eltern einer Gruppe",
  parents_by_student: "Eltern eines Kindes",
};

const weekdays = [
  { value: "monday", label: "Mo" },
  { value: "tuesday", label: "Di" },
  { value: "wednesday", label: "Mi" },
  { value: "thursday", label: "Do" },
  { value: "friday", label: "Fr" },
  { value: "saturday", label: "Sa" },
  { value: "sunday", label: "So" },
];

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function startOfCurrentWeek(): Date {
  return getWeekRange(new Date()).from;
}

function weekdayName(dateISO: string): string {
  const date = new Date(`${dateISO}T00:00:00`);
  return date.toLocaleDateString("en-US", { weekday: "long" }).toLowerCase();
}

function choicesForTarget(
  type: CalendarTargetType,
  options: CalendarRecipientOptions,
): Choice[] {
  switch (type) {
    case "staff":
      return options.staff.map((staff) => ({
        id: staff.id,
        label: staff.name,
      }));
    case "guardian_profile":
      return options.parents.map((parent) => ({
        id: parent.id,
        label: parent.name,
      }));
    case "parents_by_group":
      return options.groups.map((group) => ({
        id: group.id,
        label: group.name,
      }));
    case "parents_by_class":
      return options.classes.map((schoolClass) => ({
        value: schoolClass,
        label: schoolClass,
      }));
    case "parents_by_student":
      return options.students.map((student) => ({
        id: student.id,
        label: student.school_class
          ? `${student.name} · ${student.school_class}`
          : student.name,
      }));
    default:
      return [];
  }
}

function serializeTarget(target: DraftTarget): CalendarTarget {
  if (target.type === "parents_by_class") {
    return { type: target.type, value: target.value };
  }
  if (target.type === "all_staff") {
    return { type: target.type };
  }
  return { type: target.type, id: target.id };
}

export default function StaffCalendarPage() {
  const toast = useToast();
  const [weekStart, setWeekStart] = useState(startOfCurrentWeek);
  const [formOpen, setFormOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [location, setLocation] = useState("");
  const [startDate, setStartDate] = useState(toISODate(new Date()));
  const [endDate, setEndDate] = useState(toISODate(new Date()));
  const [startTime, setStartTime] = useState("09:00");
  const [endTime, setEndTime] = useState("10:00");
  const [allDay, setAllDay] = useState(false);
  const [deliveryMode, setDeliveryMode] =
    useState<CalendarDeliveryMode>("rsvp_required");
  const [targetType, setTargetType] = useState<CalendarTargetType>("staff");
  const [targetValue, setTargetValue] = useState("");
  const [targetSearch, setTargetSearch] = useState("");
  const [targets, setTargets] = useState<DraftTarget[]>([]);
  const [frequency, setFrequency] = useState<RecurrenceFrequency>("none");
  const [intervalCount, setIntervalCount] = useState(1);
  const [weeklyDays, setWeeklyDays] = useState<string[]>([]);
  const [endsOn, setEndsOn] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    number | null
  >(null);

  const range = useMemo(() => getWeekRange(weekStart), [weekStart]);
  const calendarKey = `staff-calendar-${toISODate(range.from)}-${toISODate(range.to)}`;
  const {
    data,
    error: calendarError,
    isLoading,
    mutate,
  } = useSWRAuth<CalendarResponse>(calendarKey, () =>
    getStaffCalendar(range.from, range.to),
  );

  const { data: recipientOptions = emptyRecipientOptions } =
    useSWRAuth<CalendarRecipientOptions>(
      formOpen ? `calendar-recipient-options-${targetSearch}` : null,
      () => getCalendarRecipientOptions(targetSearch),
    );

  const targetChoices = useMemo(
    () => choicesForTarget(targetType, recipientOptions),
    [recipientOptions, targetType],
  );

  const addTarget = () => {
    if (targetType === "all_staff") {
      if (targets.some((target) => target.type === "all_staff")) return;
      setTargets((current) => [
        ...current,
        {
          key: "all_staff",
          type: "all_staff",
          label: targetTypeLabels.all_staff,
        },
      ]);
      return;
    }

    const selected = targetChoices.find((choice) =>
      choice.id
        ? String(choice.id) === targetValue
        : choice.value === targetValue,
    );
    if (!selected) {
      toast.warning("Bitte zuerst ein Ziel auswählen.");
      return;
    }

    const key =
      targetType === "parents_by_class"
        ? `${targetType}:${selected.value}`
        : `${targetType}:${selected.id}`;
    if (targets.some((target) => target.key === key)) return;
    setTargets((current) => [
      ...current,
      {
        key,
        type: targetType,
        id: selected.id,
        value: selected.value,
        label: `${targetTypeLabels[targetType]}: ${selected.label}`,
      },
    ]);
    setTargetValue("");
  };

  const removeTarget = (key: string) => {
    setTargets((current) => current.filter((target) => target.key !== key));
  };

  const handleRespond = async (
    recipientId: number,
    status: "accepted" | "declined",
  ) => {
    setRespondingRecipientId(recipientId);
    try {
      await respondStaffCalendar(recipientId, status);
      await mutate();
      toast.success(
        status === "accepted" ? "Termin zugesagt." : "Termin abgesagt.",
      );
    } catch (err) {
      toast.error(
        errorMessage(err, "Antwort konnte nicht gespeichert werden."),
      );
    } finally {
      setRespondingRecipientId(null);
    }
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!title.trim()) {
      toast.warning("Bitte einen Titel eintragen.");
      return;
    }
    if (targets.length === 0) {
      toast.warning("Bitte mindestens ein Ziel hinzufügen.");
      return;
    }

    const recurrence =
      frequency === "none"
        ? undefined
        : {
            frequency,
            interval_count: Math.max(1, intervalCount),
            weekdays:
              frequency === "weekly"
                ? weeklyDays.length > 0
                  ? weeklyDays
                  : [weekdayName(startDate)]
                : undefined,
            ends_on: endsOn || undefined,
          };

    setSubmitting(true);
    try {
      await createStaffAppointment({
        title: title.trim(),
        description: description.trim() || undefined,
        location: location.trim() || undefined,
        start_date: startDate,
        end_date: endDate || startDate,
        start_time: allDay ? "00:00" : startTime,
        end_time: allDay ? "23:59" : endTime,
        all_day: allDay,
        delivery_mode: deliveryMode,
        recurrence,
        targets: targets.map(serializeTarget),
      });
      toast.success("Termin wurde erstellt.");
      setTitle("");
      setDescription("");
      setLocation("");
      setTargets([]);
      setFrequency("none");
      setEndsOn("");
      setFormOpen(false);
      await mutate();
    } catch (err) {
      toast.error(errorMessage(err, "Termin konnte nicht erstellt werden."));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <PersonalCalendar
        title="Mein Kalender"
        subtitle="Deine Termine, Einladungen und zugewiesenen Betreuungsangebote."
        events={data?.events ?? []}
        weekStart={range.from}
        loading={isLoading}
        error={
          calendarError
            ? errorMessage(
                calendarError,
                "Kalender konnte nicht geladen werden.",
              )
            : null
        }
        onWeekChange={setWeekStart}
        onCreate={() => setFormOpen((open) => !open)}
        onRespond={handleRespond}
        respondingRecipientId={respondingRecipientId}
      />

      {formOpen ? (
        <form
          className="mt-6 space-y-5 rounded-lg border border-gray-200 bg-white p-4 shadow-sm"
          onSubmit={handleSubmit}
        >
          <div className="flex items-center gap-2 border-b border-gray-200 pb-3">
            <CalendarPlus className="h-5 w-5 text-gray-600" aria-hidden />
            <h2 className="text-lg font-semibold text-gray-900">
              Termin erstellen
            </h2>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <Input
              label="Titel"
              name="calendar-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              disabled={submitting}
              required
            />
            <Input
              label="Ort"
              name="calendar-location"
              value={location}
              onChange={(event) => setLocation(event.target.value)}
              disabled={submitting}
            />
            <Input
              label="Startdatum"
              name="calendar-start-date"
              type="date"
              value={startDate}
              onChange={(event) => {
                setStartDate(event.target.value);
                if (endDate < event.target.value)
                  setEndDate(event.target.value);
              }}
              disabled={submitting}
              required
            />
            <Input
              label="Enddatum"
              name="calendar-end-date"
              type="date"
              value={endDate}
              min={startDate}
              onChange={(event) => setEndDate(event.target.value)}
              disabled={submitting}
              required
            />
            <Input
              label="Startzeit"
              name="calendar-start-time"
              type="time"
              value={startTime}
              onChange={(event) => setStartTime(event.target.value)}
              disabled={submitting || allDay}
              required
            />
            <Input
              label="Endzeit"
              name="calendar-end-time"
              type="time"
              value={endTime}
              onChange={(event) => setEndTime(event.target.value)}
              disabled={submitting || allDay}
              required
            />
          </div>

          <label className="flex items-center gap-2 text-sm font-medium text-gray-700">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-gray-300"
              checked={allDay}
              onChange={(event) => setAllDay(event.target.checked)}
              disabled={submitting}
            />
            Ganztägig
          </label>

          <label className="block">
            <span className="mb-2 block text-sm font-medium text-gray-700">
              Beschreibung
            </span>
            <textarea
              className="block min-h-24 w-full rounded-lg border-0 bg-white px-4 py-3 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              disabled={submitting}
            />
          </label>

          <div className="grid gap-4 lg:grid-cols-[1fr_1fr_auto]">
            <label className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Versandart
              </span>
              <select
                className="block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
                value={deliveryMode}
                onChange={(event) =>
                  setDeliveryMode(event.target.value as CalendarDeliveryMode)
                }
                disabled={submitting}
              >
                <option value="rsvp_required">
                  Einladung mit Zusage/Absage
                </option>
                <option value="informational">
                  Direkt eintragen ohne Antwort
                </option>
              </select>
            </label>
            <Input
              label="Ziel suchen"
              name="calendar-target-search"
              controlSize="compact"
              value={targetSearch}
              onChange={(event) => setTargetSearch(event.target.value)}
              disabled={submitting}
              placeholder="Name, Klasse oder Gruppe"
            />
            <div className="flex items-end">
              <Button
                type="button"
                variant="outline"
                size="md"
                className="w-full"
                onClick={addTarget}
                disabled={submitting}
              >
                Ziel hinzufügen
              </Button>
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Zieltyp
              </span>
              <select
                className="block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
                value={targetType}
                onChange={(event) => {
                  setTargetType(event.target.value as CalendarTargetType);
                  setTargetValue("");
                }}
                disabled={submitting}
              >
                {Object.entries(targetTypeLabels).map(([type, label]) => (
                  <option key={type} value={type}>
                    {label}
                  </option>
                ))}
              </select>
            </label>

            {targetType !== "all_staff" ? (
              <label className="block">
                <span className="mb-2 block text-sm font-medium text-gray-700">
                  Ziel
                </span>
                <select
                  className="block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
                  value={targetValue}
                  onChange={(event) => setTargetValue(event.target.value)}
                  disabled={submitting || targetChoices.length === 0}
                >
                  <option value="">
                    {targetChoices.length === 0
                      ? "Keine Treffer"
                      : "Bitte auswählen"}
                  </option>
                  {targetChoices.map((choice) => {
                    const value = choice.id ? String(choice.id) : choice.value;
                    return (
                      <option key={value} value={value}>
                        {choice.label}
                      </option>
                    );
                  })}
                </select>
              </label>
            ) : null}
          </div>

          {targets.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {targets.map((target) => (
                <span
                  key={target.key}
                  className="inline-flex items-center gap-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 text-sm text-gray-700"
                >
                  {target.label}
                  <button
                    type="button"
                    className="rounded p-0.5 text-gray-500 hover:bg-gray-200 hover:text-gray-900"
                    onClick={() => removeTarget(target.key)}
                    disabled={submitting}
                    aria-label={`${target.label} entfernen`}
                  >
                    <Trash2 className="h-3.5 w-3.5" aria-hidden />
                  </button>
                </span>
              ))}
            </div>
          ) : null}

          <div className="grid gap-4 md:grid-cols-3">
            <label className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Wiederholung
              </span>
              <select
                className="block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
                value={frequency}
                onChange={(event) =>
                  setFrequency(event.target.value as RecurrenceFrequency)
                }
                disabled={submitting}
              >
                <option value="none">Keine</option>
                <option value="daily">Täglich</option>
                <option value="weekly">Wöchentlich</option>
                <option value="monthly">Monatlich</option>
                <option value="yearly">Jährlich</option>
              </select>
            </label>
            <Input
              label="Intervall"
              name="calendar-recurrence-interval"
              type="number"
              min={1}
              value={intervalCount}
              onChange={(event) =>
                setIntervalCount(Number.parseInt(event.target.value, 10) || 1)
              }
              disabled={submitting || frequency === "none"}
            />
            <Input
              label="Endet am"
              name="calendar-recurrence-end"
              type="date"
              value={endsOn}
              min={startDate}
              onChange={(event) => setEndsOn(event.target.value)}
              disabled={submitting || frequency === "none"}
            />
          </div>

          {frequency === "weekly" ? (
            <div className="flex flex-wrap gap-2">
              {weekdays.map((day) => (
                <label
                  key={day.value}
                  className="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2 py-1 text-sm text-gray-700"
                >
                  <input
                    type="checkbox"
                    className="h-4 w-4 rounded border-gray-300"
                    checked={weeklyDays.includes(day.value)}
                    onChange={(event) => {
                      setWeeklyDays((current) =>
                        event.target.checked
                          ? [...current, day.value]
                          : current.filter((value) => value !== day.value),
                      );
                    }}
                    disabled={submitting}
                  />
                  {day.label}
                </label>
              ))}
            </div>
          ) : null}

          <div className="flex flex-wrap justify-end gap-2 border-t border-gray-200 pt-4">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setFormOpen(false)}
              disabled={submitting}
            >
              Abbrechen
            </Button>
            <Button
              type="submit"
              size="md"
              isLoading={submitting}
              loadingText="Speichert..."
            >
              Termin speichern
            </Button>
          </div>
        </form>
      ) : null}
    </main>
  );
}
