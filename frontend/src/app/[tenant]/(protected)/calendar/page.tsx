"use client";

import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { CalendarPlus, Trash2 } from "lucide-react";
import { useSession } from "next-auth/react";

import {
  CalendarOverviewList,
  PersonalCalendar,
  type CalendarViewMode,
} from "~/components/calendar/personal-calendar";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { toISODate } from "~/lib/date-helpers";
import {
  createStaffAppointment,
  getCalendarRecipientOptions,
  getStaffAppointmentOverview,
  getStaffCalendar,
  respondStaffCalendar,
  type CalendarAppointmentOverview,
  type CalendarDeliveryMode,
  type CalendarOverviewVisibility,
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
  readonly key: string;
  readonly type: CalendarTargetType;
  readonly id?: number;
  readonly value?: string;
  readonly label: string;
  readonly covered?: boolean;
}

const emptyRecipientOptions: CalendarRecipientOptions = {
  staff: [],
  parents: [],
  groups: [],
  classes: [],
  students: [],
};

const targetTypeLabels: Record<CalendarTargetType, string> = {
  staff: "Mitarbeitende",
  guardian_profile: "Einzelne Eltern",
  all_staff: "Alle Mitarbeitenden",
  parents_by_class: "Eltern nach Klasse",
  parents_by_group: "Eltern nach Gruppe",
  parents_by_student: "Eltern nach Kind",
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

function calendarRange(referenceDate: Date, viewMode: CalendarViewMode) {
  if (viewMode === "day") return { from: referenceDate, to: referenceDate };
  if (viewMode === "month") {
    const first = new Date(
      referenceDate.getFullYear(),
      referenceDate.getMonth(),
      1,
    );
    const last = new Date(
      referenceDate.getFullYear(),
      referenceDate.getMonth() + 1,
      0,
    );
    return { from: first, to: last };
  }
  return getWeekRange(referenceDate);
}

function targetKey(
  type: CalendarTargetType,
  id?: number,
  value?: string,
): string {
  if (type === "all_staff") return "all_staff";
  return `${type}:${id ?? value ?? ""}`;
}

function serializeTarget(target: DraftTarget): CalendarTarget {
  if (target.type === "parents_by_class") {
    return { type: target.type, value: target.value };
  }
  if (target.type === "all_staff") return { type: target.type };
  return { type: target.type, id: target.id };
}

function isCoveredByAggregate(
  choice: Choice,
  targets: readonly DraftTarget[],
  options: CalendarRecipientOptions,
): boolean {
  if (choice.type === "staff") {
    return targets.some((target) => target.type === "all_staff");
  }
  if (choice.type === "parents_by_student") {
    const student = options.students.find((item) => item.id === choice.id);
    if (!student) return false;
    return targets.some((target) => {
      if (target.type === "parents_by_class") {
        return target.value === student.school_class;
      }
      if (target.type === "parents_by_group") {
        return target.id === student.group_id;
      }
      return false;
    });
  }
  return false;
}

function buildTargetGroups(
  options: CalendarRecipientOptions,
  targets: readonly DraftTarget[],
): Array<{ type: CalendarTargetType; label: string; choices: Choice[] }> {
  const groups: Array<{
    type: CalendarTargetType;
    label: string;
    choices: Choice[];
  }> = [
    {
      type: "all_staff",
      label: targetTypeLabels.all_staff,
      choices: [
        {
          key: "all_staff",
          type: "all_staff",
          label: "Alle Mitarbeitenden",
        },
      ],
    },
    {
      type: "staff",
      label: targetTypeLabels.staff,
      choices: options.staff.map((staff) => ({
        key: targetKey("staff", staff.id),
        type: "staff",
        id: staff.id,
        label: staff.name,
      })),
    },
    {
      type: "parents_by_class",
      label: targetTypeLabels.parents_by_class,
      choices: options.classes.map((schoolClass) => ({
        key: targetKey("parents_by_class", undefined, schoolClass),
        type: "parents_by_class",
        value: schoolClass,
        label: schoolClass,
      })),
    },
    {
      type: "parents_by_group",
      label: targetTypeLabels.parents_by_group,
      choices: options.groups.map((group) => ({
        key: targetKey("parents_by_group", group.id),
        type: "parents_by_group",
        id: group.id,
        label: group.name,
      })),
    },
    {
      type: "parents_by_student",
      label: targetTypeLabels.parents_by_student,
      choices: options.students.map((student) => ({
        key: targetKey("parents_by_student", student.id),
        type: "parents_by_student",
        id: student.id,
        label: student.school_class
          ? `${student.name} · ${student.school_class}`
          : student.name,
      })),
    },
    {
      type: "guardian_profile",
      label: targetTypeLabels.guardian_profile,
      choices: options.parents.map((parent) => ({
        key: targetKey("guardian_profile", parent.id),
        type: "guardian_profile",
        id: parent.id,
        label: parent.name,
      })),
    },
  ];

  return groups.map((group) => ({
    ...group,
    choices: group.choices.map((choice) => ({
      ...choice,
      covered: isCoveredByAggregate(choice, targets, options),
    })),
  }));
}

export default function StaffCalendarPage() {
  const toast = useToast();
  const { data: session } = useSession();
  const canManageCalendar = hasPermission(session, "calendar:manage");
  const [referenceDate, setReferenceDate] = useState(startOfCurrentWeek);
  const [viewMode, setViewMode] = useState<CalendarViewMode>("week");
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
  const [overviewVisibility, setOverviewVisibility] =
    useState<CalendarOverviewVisibility>("organizer");
  const [targetSearch, setTargetSearch] = useState("");
  const [targets, setTargets] = useState<DraftTarget[]>([]);
  const [overview, setOverview] = useState<CalendarAppointmentOverview | null>(
    null,
  );
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [frequency, setFrequency] = useState<RecurrenceFrequency>("none");
  const [intervalCount, setIntervalCount] = useState(1);
  const [weeklyDays, setWeeklyDays] = useState<string[]>([]);
  const [endsOn, setEndsOn] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    number | null
  >(null);

  const range = useMemo(
    () => calendarRange(referenceDate, viewMode),
    [referenceDate, viewMode],
  );
  const calendarKey = `staff-calendar-${viewMode}-${toISODate(range.from)}-${toISODate(range.to)}`;
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
      formOpen && canManageCalendar
        ? `calendar-recipient-options-${targetSearch}`
        : null,
      () => getCalendarRecipientOptions(targetSearch),
    );

  const targetGroups = useMemo(
    () => buildTargetGroups(recipientOptions, targets),
    [recipientOptions, targets],
  );
  const selectedKeys = useMemo(
    () => new Set(targets.map((target) => target.key)),
    [targets],
  );

  const toggleTarget = (choice: Choice) => {
    const selected = selectedKeys.has(choice.key);
    if (selected) {
      setTargets((current) =>
        current.filter((target) => target.key !== choice.key),
      );
      return;
    }
    if (choice.covered) return;
    setTargets((current) => [
      ...current,
      {
        key: choice.key,
        type: choice.type,
        id: choice.id,
        value: choice.value,
        label: `${targetTypeLabels[choice.type]}: ${choice.label}`,
      },
    ]);
  };

  const removeTarget = (key: string) => {
    setTargets((current) => current.filter((target) => target.key !== key));
  };

  const resetForm = () => {
    setTitle("");
    setDescription("");
    setLocation("");
    setTargets([]);
    setFrequency("none");
    setEndsOn("");
    setWeeklyDays([]);
    setIntervalCount(1);
    setOverviewVisibility("organizer");
    setFormOpen(false);
  };

  const handleShowOverview = async (appointmentId: string | number) => {
    setOverviewLoading(true);
    try {
      setOverview(await getStaffAppointmentOverview(appointmentId));
    } catch (err) {
      toast.error(
        errorMessage(err, "Teilnehmerübersicht konnte nicht geladen werden."),
      );
    } finally {
      setOverviewLoading(false);
    }
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
      toast.warning("Bitte mindestens ein Ziel auswählen.");
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
        overview_visibility: overviewVisibility,
        recurrence,
        targets: targets.map(serializeTarget),
      });
      toast.success("Termin wurde erstellt.");
      resetForm();
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
        referenceDate={referenceDate}
        viewMode={viewMode}
        loading={isLoading}
        error={
          calendarError
            ? errorMessage(
                calendarError,
                "Kalender konnte nicht geladen werden.",
              )
            : null
        }
        onDateChange={setReferenceDate}
        onViewModeChange={setViewMode}
        onCreate={canManageCalendar ? () => setFormOpen(true) : undefined}
        onShowOverview={handleShowOverview}
        onRespond={handleRespond}
        respondingRecipientId={respondingRecipientId}
      />

      <Modal
        isOpen={formOpen && canManageCalendar}
        onClose={() => {
          if (!submitting) setFormOpen(false);
        }}
        title="Termin erstellen"
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-5xl"
      >
        <form className="space-y-5" onSubmit={handleSubmit}>
          <div className="flex items-center gap-2">
            <CalendarPlus className="h-5 w-5 text-gray-600" aria-hidden />
            <p className="text-sm text-gray-600">
              Lege Zeitpunkt, Antwortregel und Empfängergruppen fest.
            </p>
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

          <label
            htmlFor="calendar-all-day"
            className="flex items-center gap-2 text-sm font-medium text-gray-700"
          >
            <Checkbox
              id="calendar-all-day"
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

          <div className="grid gap-4 md:grid-cols-2">
            <label className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Antwortregel
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
                  Antwort erforderlich: Zusage oder Absage
                </option>
                <option value="informational">
                  Nur informieren: ohne Rückmeldung eintragen
                </option>
              </select>
            </label>
            <label className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Teilnehmerübersicht
              </span>
              <select
                className="block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
                value={overviewVisibility}
                onChange={(event) =>
                  setOverviewVisibility(
                    event.target.value as CalendarOverviewVisibility,
                  )
                }
                disabled={submitting}
              >
                <option value="organizer">Nur ich</option>
                <option value="staff">Mitarbeitende mit Termin</option>
                <option value="all">Alle Eingeladenen</option>
              </select>
            </label>
            <Input
              label="Ziele suchen"
              name="calendar-target-search"
              controlSize="compact"
              value={targetSearch}
              onChange={(event) => setTargetSearch(event.target.value)}
              disabled={submitting}
              placeholder="Name, Klasse oder Gruppe"
            />
          </div>

          <div className="rounded-lg border border-gray-200 bg-white p-3">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm font-semibold text-gray-900">
                Empfänger auswählen
              </h3>
              <span className="text-xs text-gray-500">
                {targets.length} Ziel{targets.length === 1 ? "" : "e"}{" "}
                ausgewählt
              </span>
            </div>
            <div className="grid gap-3 lg:grid-cols-2">
              {targetGroups.map((group) => (
                <section
                  key={group.type}
                  className="rounded-lg border border-gray-200 bg-gray-50/70 p-3"
                >
                  <h4 className="text-xs font-semibold tracking-wide text-gray-700 uppercase">
                    {group.label}
                  </h4>
                  <div className="mt-2 max-h-48 space-y-1 overflow-y-auto pr-1">
                    {group.choices.length === 0 ? (
                      <p className="py-2 text-xs text-gray-500">
                        Keine Treffer
                      </p>
                    ) : (
                      group.choices.map((choice) => {
                        const selected = selectedKeys.has(choice.key);
                        const disabled =
                          submitting || (!selected && choice.covered);
                        const checkboxId = `calendar-target-${choice.key.replace(/[^a-z0-9_-]/gi, "-")}`;
                        return (
                          <label
                            key={choice.key}
                            htmlFor={checkboxId}
                            className={`flex items-center gap-2 rounded-md border px-2 py-1.5 text-sm transition-colors ${
                              selected
                                ? "border-gray-900 bg-white text-gray-950"
                                : choice.covered
                                  ? "border-gray-200 bg-[#ECF7DA] text-gray-500"
                                  : "border-transparent bg-white text-gray-700 hover:border-gray-200"
                            } ${disabled ? "cursor-not-allowed opacity-75" : "cursor-pointer"}`}
                          >
                            <Checkbox
                              id={checkboxId}
                              checked={selected}
                              disabled={disabled}
                              onChange={() => toggleTarget(choice)}
                            />
                            <span className="min-w-0 flex-1 truncate">
                              {choice.label}
                            </span>
                            {choice.covered && !selected ? (
                              <span className="text-[11px] font-medium text-gray-500">
                                bereits enthalten
                              </span>
                            ) : null}
                          </label>
                        );
                      })
                    )}
                  </div>
                </section>
              ))}
            </div>
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
                  htmlFor={`calendar-weekday-${day.value}`}
                  className="inline-flex items-center gap-1 rounded-md border border-gray-200 px-2 py-1 text-sm text-gray-700"
                >
                  <Checkbox
                    id={`calendar-weekday-${day.value}`}
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
      </Modal>

      <Modal
        isOpen={overview !== null || overviewLoading}
        onClose={() => {
          if (!overviewLoading) setOverview(null);
        }}
        title="Teilnehmer"
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-xl"
      >
        {overviewLoading ? (
          <div className="py-8 text-center text-sm text-gray-500">
            Teilnehmer werden geladen...
          </div>
        ) : overview ? (
          <CalendarOverviewList overview={overview} />
        ) : null}
      </Modal>
    </main>
  );
}
