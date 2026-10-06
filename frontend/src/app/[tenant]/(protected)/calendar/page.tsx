"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "next/navigation";
import type { FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import dynamic from "next/dynamic";
import { useSession } from "next-auth/react";

import {
  calendarEmptyLabel,
  CalendarOverviewList,
  PersonalCalendar,
  PersonalCalendarChrome,
  visibleCalendarEvents,
  type CalendarViewMode,
} from "~/components/calendar/personal-calendar";
import { CalendarSubscribePanel } from "~/components/calendar/calendar-subscribe-panel";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  ListSkeleton,
  SkeletonRegion,
  TableSkeleton,
} from "~/components/ui/page-skeletons";
import { TenantPage } from "~/components/ui/tenant-page";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { berlinTodayISO, toISODate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  cancelStaffAppointment,
  cancelStaffAppointmentOccurrence,
  createStaffAppointment,
  deleteStaffAppointment,
  getCalendarRecipientOptions,
  getStaffAppointmentDetail,
  getStaffAppointmentOverview,
  getStaffCalendar,
  respondStaffCalendar,
  updateStaffAppointment,
  type CalendarAppointmentOverview,
  type CalendarDeliveryMode,
  type CalendarEvent,
  type CalendarOverviewVisibility,
  type CalendarRecipientOptions,
  type CalendarResponse,
  type CalendarTarget,
  type CalendarTargetType,
} from "~/lib/personal-calendar-api";
import { useSWRAuth } from "~/lib/swr";
import { useCalDAVEnabled } from "~/lib/tenant-context";
import { getWeekRange } from "~/lib/timetable-helpers";

type RecurrenceFrequency = "none" | "daily" | "weekly" | "monthly" | "yearly";

interface DraftTarget extends CalendarTarget {
  readonly key: string;
  readonly label: string;
}

interface Choice {
  readonly key: string;
  readonly type: CalendarTargetType;
  readonly id?: string;
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
  all_school_parents: "Alle Eltern der Schule",
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

const logger = createLogger({ component: "StaffCalendarPage" });

function logFailure(event: string, error: unknown) {
  logger.error(event, {
    error: error instanceof Error ? error.message : String(error),
  });
}

function weekdayName(dateISO: string): string {
  const date = new Date(`${dateISO}T00:00:00`);
  return date.toLocaleDateString("en-US", { weekday: "long" }).toLowerCase();
}

function calendarRange(referenceDate: Date, viewMode: CalendarViewMode) {
  if (viewMode === "day") return { from: referenceDate, to: referenceDate };
  if (viewMode === "month") {
    // Month view renders a 42-day grid starting at the week containing the 1st
    // (see monthGridDays in personal-calendar.tsx). Fetch that full visible
    // range so appointments on the leading/trailing adjacent-month days show,
    // instead of only the calendar month itself.
    const firstOfMonth = new Date(
      referenceDate.getFullYear(),
      referenceDate.getMonth(),
      1,
    );
    const from = getWeekRange(firstOfMonth).from;
    const to = new Date(from);
    to.setDate(from.getDate() + 41);
    return { from, to };
  }
  return getWeekRange(referenceDate);
}

function targetKey(
  type: CalendarTargetType,
  id?: string,
  value?: string,
): string {
  if (type === "all_staff") return "all_staff";
  if (type === "all_school_parents") return "all_school_parents";
  return `${type}:${id ?? value ?? ""}`;
}

function serializeTarget(target: DraftTarget): CalendarTarget {
  if (target.type === "parents_by_class") {
    return { type: target.type, value: target.value };
  }
  if (target.type === "all_staff" || target.type === "all_school_parents") {
    return { type: target.type };
  }
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
  // Any parent-scoped choice is already covered once the whole school is targeted.
  if (
    choice.type === "parents_by_class" ||
    choice.type === "parents_by_group" ||
    choice.type === "parents_by_student" ||
    choice.type === "guardian_profile"
  ) {
    if (targets.some((target) => target.type === "all_school_parents")) {
      return true;
    }
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
      type: "all_school_parents",
      label: targetTypeLabels.all_school_parents,
      choices: [
        {
          key: "all_school_parents",
          type: "all_school_parents",
          label: "Alle Eltern der Schule",
        },
      ],
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

// Betreuungsplan-Leseansicht (#2283) als zweiter Tab der einen
// Kalenderfläche für Nicht-Admins. Dynamisch geladen, damit der große
// Planer-Code das Bundle der reinen Kalender-Nutzung nicht belastet.
const SchoolPlanReadView = dynamic(
  () =>
    import("~/components/timetable/betreuungsplan-view").then(
      (mod) => mod.BetreuungsplanView,
    ),
  {
    ssr: false,
    loading: () => (
      <SkeletonRegion label="Betreuungsplan wird geladen…">
        <TableSkeleton rows={7} columns={5} />
      </SkeletonRegion>
    ),
  },
);

/**
 * Ein Deeplink in den Betreuungsplan (`?view=tag&d=…&block=…`) muss auch den
 * richtigen Tab öffnen — sonst landet ein geteilter Link (#2621) auf "Meine
 * Termine" und der Zustand ist verloren. Der Betreuungsplan verwaltet seine
 * Parameter über eine Allowlist (d/view/block) und würde einen eigenen
 * Tab-Parameter beim nächsten Wechsel wieder abräumen; deshalb entscheidet
 * die Anwesenheit genau dieser Parameter über den Starttab.
 */
const SCHOOL_PLAN_PARAMS = ["d", "view", "block"] as const;

function hasSchoolPlanParams(params: URLSearchParams | null): boolean {
  return SCHOOL_PLAN_PARAMS.some((key) => params?.has(key) === true);
}

type CalendarTab = "meine" | "schule";

function StaffCalendarPageInner() {
  const searchParams = useSearchParams();
  const toast = useToast();
  const { data: session } = useSession();
  const calDAVEnabled = useCalDAVEnabled();
  const canManageCalendar = hasPermission(session, "calendar:manage");
  // Eine Kalenderfläche (#2283): Nicht-Admins mit schedules:read sehen den
  // Betreuungsplan (Leseansicht) als zweiten Tab statt als eigene Seite.
  // Admins behalten den vollwertigen Planungsbereich in der Sidebar.
  const showSchoolPlanTab =
    !isAdmin(session) && hasPermission(session, "schedules:read");
  const schoolPlanSelected = hasSchoolPlanParams(searchParams);
  const [activeTab, setActiveTab] = useState<CalendarTab>(() =>
    schoolPlanSelected ? "schule" : "meine",
  );

  // Deeplinks and browser navigation change the query without remounting this
  // page. Keep the visible tab aligned with the URL in both directions.
  useEffect(() => {
    setActiveTab(schoolPlanSelected ? "schule" : "meine");
  }, [schoolPlanSelected]);

  const handleCalendarTabChange = useCallback(
    (value: string) => {
      const nextTab = value as CalendarTab;
      const nextParams = new URLSearchParams(searchParams?.toString());
      if (nextTab === "meine") {
        for (const key of SCHOOL_PLAN_PARAMS) nextParams.delete(key);
      } else if (!hasSchoolPlanParams(nextParams)) {
        // Der Betreuungsplan hält seinen Zustand ausschließlich in
        // d/view/block. Wer den Tab von Hand wählt, hat noch keinen dieser
        // Parameter — dann trägt das Öffnen eines Termins `block` ein und das
        // Schließen räumt es wieder ab, die Fläche stünde ohne Plan-Parameter
        // da und spränge zurück auf "Meine Termine" (#2957). Der sichtbare Tag
        // ist der dauerhafte Zustand: heute, also genau das, was der Plan ohne
        // `d` ohnehin anzeigt.
        nextParams.set("d", berlinTodayISO());
      }
      const query = nextParams.toString();
      window.history.replaceState(
        null,
        "",
        query ? `?${query}` : window.location.pathname,
      );
      setActiveTab(nextTab);
    },
    [searchParams],
  );
  // Focal date defaults to today; the calendar component derives the week
  // range for week view, so today shows the current week / month / day
  // correctly (not the start of the week or the wrong month at boundaries).
  const [referenceDate, setReferenceDate] = useState(() => new Date());
  const [viewMode, setViewMode] = useState<CalendarViewMode>("week");
  // Der Wochenend-Schalter sitzt im Bedienband der Kopfkarte, das Raster
  // darunter richtet sich danach — deshalb liegt der Zustand hier.
  const [showWeekend, setShowWeekend] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [busyAppointmentId, setBusyAppointmentId] = useState<string | null>(
    null,
  );
  // Löschen bestätigt die ConfirmDeleteModal, bei einer Reihe mit Scope-Slot
  // (#3110); Absagen bleibt ein Zustandswechsel und geht über ChoiceModal
  // (Reihe) bzw. ConfirmationModal (Einzeltermin).
  const [deleteTarget, setDeleteTarget] = useState<CalendarEvent | null>(null);
  const [deleteScope, setDeleteScope] = useState<
    "occurrence" | "series" | null
  >(null);
  const [cancelTarget, setCancelTarget] = useState<CalendarEvent | null>(null);
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
  const [sendEmail, setSendEmail] = useState(false);
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
  // Recurrence fields the form has no dedicated control for yet (monthly
  // by-day-of-month, count-based end). They are preserved verbatim across an
  // edit so re-saving a series never silently drops them (which would change
  // the schedule or make a bounded series unbounded).
  const [monthDays, setMonthDays] = useState<number[]>([]);
  const [occurrenceCount, setOccurrenceCount] = useState<number | null>(null);
  const [submitting, setSubmitting] = useState(false);
  // Fehler des Terminformulars: Alert oben im Panel, Feldfehler am Feld
  // (Bauart 2 Regel 5). Kein Toast: der verblasst, bevor jemand bei
  // fünfzehn Feldern das fehlende gefunden hat.
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const clearFormErrors = formErrors.clear;
  // Aktionen ohne eigene Fläche (Termin öffnen, Antworten, Teilnehmer,
  // Absagen): Toast mit Katalogtext und Wiederholen (#2517).
  const { show: showActionError } = useApiErrorDisplay();
  // Fehler beim Löschen bleibt im geöffneten Dialog stehen.
  const deleteErrors = useApiFormError();
  const clearDeleteErrors = deleteErrors.clear;
  // „Wiederholen“ läuft über die jeweils aktuelle Fassung der Aktion.
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const latestEditRef = useRef<(event: CalendarEvent) => void>(() => undefined);
  const latestRunScopeRef = useRef<
    (
      event: CalendarEvent,
      mode: "cancel" | "delete",
      scope: "occurrence" | "series",
    ) => void
  >(() => undefined);
  const latestOverviewRef = useRef<(appointmentId: string) => void>(
    () => undefined,
  );
  const latestRespondRef = useRef<
    (recipientId: string, status: "accepted" | "declined") => void
  >(() => undefined);
  const [respondingRecipientId, setRespondingRecipientId] = useState<
    string | null
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
    setMonthDays([]);
    setOccurrenceCount(null);
    setOverviewVisibility("organizer");
    setDeliveryMode("rsvp_required");
    setSendEmail(false);
    setEditingId(null);
    clearFormErrors();
    setFormOpen(false);
  };

  const handleCreate = () => {
    resetForm();
    setStartDate(toISODate(new Date()));
    setEndDate(toISODate(new Date()));
    setStartTime("09:00");
    setEndTime("10:00");
    setAllDay(false);
    setFormOpen(true);
  };

  const handleEdit = async (event: CalendarEvent) => {
    if (!event.appointment_id) return;
    setBusyAppointmentId(event.appointment_id);
    try {
      // Editing is series-scoped (UpdateStaffAppointment rewrites the whole
      // appointment), so prefill from the persisted appointment DETAIL — its
      // base title/dates/etc. — NOT from the clicked occurrence. Otherwise
      // opening a later occurrence and saving would re-anchor the series to that
      // occurrence's date and drop earlier occurrences. Times don't shift per
      // occurrence (only dates do), so the occurrence event's clean HH:MM values
      // match the base and are safe to reuse.
      const detail = await getStaffAppointmentDetail(event.appointment_id);
      const base = detail.appointment;
      setTitle(base.title);
      setDescription(base.description ?? "");
      setLocation(base.location ?? "");
      setStartDate(base.start_date);
      setEndDate(base.end_date);
      setAllDay(base.all_day);
      setStartTime(base.all_day ? "09:00" : event.start_time);
      setEndTime(base.all_day ? "10:00" : event.end_time);
      setOverviewVisibility(base.overview_visibility);
      setSendEmail(base.notify_guardians);
      if (detail.recurrence) {
        setFrequency(detail.recurrence.frequency);
        setIntervalCount(detail.recurrence.interval_count);
        setWeeklyDays(detail.recurrence.weekdays ?? []);
        setEndsOn(detail.recurrence.ends_on ?? "");
        // Preserve fields the form can't yet edit so re-saving doesn't drop them.
        setMonthDays(detail.recurrence.month_days ?? []);
        setOccurrenceCount(detail.recurrence.occurrence_count ?? null);
      } else {
        setFrequency("none");
        setIntervalCount(1);
        setWeeklyDays([]);
        setEndsOn("");
        setMonthDays([]);
        setOccurrenceCount(null);
      }
      setEditingId(event.appointment_id);
      setFormOpen(true);
    } catch (err) {
      logFailure("calendar_appointment_detail_failed", err);
      void showActionError(err, {
        object: "das Öffnen des Termins",
        retry: () => latestEditRef.current(event),
      });
    } finally {
      setBusyAppointmentId(null);
    }
  };

  const runScope = async (
    event: CalendarEvent,
    mode: "cancel" | "delete",
    scope: "occurrence" | "series",
  ) => {
    if (!event.appointment_id) return;
    const appointmentId = event.appointment_id;
    setBusyAppointmentId(appointmentId);
    clearDeleteErrors();
    try {
      if (scope === "occurrence") {
        await cancelStaffAppointmentOccurrence(
          appointmentId,
          event.occurrence_date ?? event.start_date,
        );
      } else if (mode === "cancel") {
        await cancelStaffAppointment(appointmentId);
      } else {
        await deleteStaffAppointment(appointmentId);
      }
      toast.success(scopeSuccessMessage(event, mode, scope));
      setDeleteTarget(null);
      setCancelTarget(null);
    } catch (err) {
      logFailure(`calendar_appointment_${mode}_failed`, err);
      const object = scopeErrorObject(mode, scope);
      if (mode === "delete") {
        // Der Löschdialog bleibt offen und zeigt den Grund.
        await deleteErrors.show(err, {
          object,
          retry: () => latestRunScopeRef.current(event, mode, scope),
        });
      } else {
        // Die Absage-Dialoge haben keinen Fehler-Slot: schließen, Grund im
        // Toast, Wiederholen mit derselben Wahl.
        setCancelTarget(null);
        void showActionError(err, {
          object,
          retry: () => latestRunScopeRef.current(event, mode, scope),
        });
      }
      return;
    } finally {
      setBusyAppointmentId(null);
    }
    await refreshCalendar();
  };

  const handleCancel = (event: CalendarEvent) => setCancelTarget(event);
  const handleDelete = (event: CalendarEvent) => {
    setDeleteScope(event.recurring ? null : "series");
    clearDeleteErrors();
    setDeleteTarget(event);
  };

  const handleShowOverview = async (appointmentId: string) => {
    // Clear any previous appointment's attendees so a failed/slow request
    // can't leave the old overview visible under a different appointment.
    setOverview(null);
    setOverviewLoading(true);
    try {
      setOverview(await getStaffAppointmentOverview(appointmentId));
    } catch (err) {
      setOverview(null);
      logFailure("calendar_appointment_overview_failed", err);
      void showActionError(err, {
        object: "die Teilnehmerübersicht",
        retry: () => latestOverviewRef.current(appointmentId),
      });
    } finally {
      setOverviewLoading(false);
    }
  };

  const handleRespond = async (
    recipientId: string,
    status: "accepted" | "declined",
  ) => {
    setRespondingRecipientId(recipientId);
    try {
      await respondStaffCalendar(recipientId, status);
      toast.success(
        status === "accepted"
          ? "Ihre Zusage ist gespeichert."
          : "Ihre Absage ist gespeichert.",
      );
    } catch (err) {
      logFailure("calendar_response_failed", err);
      void showActionError(err, {
        object: "das Speichern Ihrer Antwort",
        retry: () => latestRespondRef.current(recipientId, status),
      });
      return;
    } finally {
      setRespondingRecipientId(null);
    }
    await refreshCalendar();
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submit();
  };

  const submit = async () => {
    // Prüfung vor dem Senden: Hinweis am Feld, Sammelsatz oben im Panel.
    const fields = appointmentFieldErrors(title, startDate, endDate);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }
    if (!editingId && targets.length === 0) {
      formErrors.invalid("Bitte wählen Sie mindestens einen Empfänger aus.");
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
            // Preserve monthly day-of-month selection through an edit.
            month_days:
              frequency === "monthly" && monthDays.length > 0
                ? monthDays
                : undefined,
            ends_on: endsOn || undefined,
            // ends_on and occurrence_count are mutually exclusive end modes
            // (the backend rejects both), so only send the count when no
            // end date is set — this keeps a count-bounded series bounded.
            occurrence_count:
              !endsOn && occurrenceCount ? occurrenceCount : undefined,
          };

    formErrors.clear();
    setSubmitting(true);
    try {
      if (editingId) {
        // Editing keeps the original audience + delivery mode (see the API
        // note): those cannot change without discarding collected RSVPs.
        await updateStaffAppointment(editingId, {
          title: title.trim(),
          description: description.trim() || undefined,
          location: location.trim() || undefined,
          start_date: startDate,
          end_date: endDate || startDate,
          start_time: allDay ? "00:00" : startTime,
          end_time: allDay ? "23:59" : endTime,
          all_day: allDay,
          overview_visibility: overviewVisibility,
          recurrence,
          send_email: sendEmail,
        });
      } else {
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
          send_email: sendEmail,
        });
      }
      toast.success(`Der Termin „${title.trim()}“ ist gespeichert.`);
      resetForm();
    } catch (err) {
      logFailure("calendar_appointment_save_failed", err);
      await formErrors.show(err, {
        object: "das Speichern des Termins",
        retry: () => latestSubmitRef.current(),
      });
      return;
    } finally {
      setSubmitting(false);
    }
    await refreshCalendar();
  };

  // Gespeichert ist schon. Scheitert nur das Neuladen des Rasters, zeigt die
  // Seite den Ladefehler mit Wiederholen über `calendarError`; nichts geht
  // verloren, deshalb kein zweiter Fehler hier.
  const refreshCalendar = async () => {
    try {
      await mutate();
    } catch (err) {
      logFailure("calendar_refresh_failed", err);
    }
  };

  useLayoutEffect(() => {
    latestSubmitRef.current = () => void submit();
    latestEditRef.current = (event) => void handleEdit(event);
    latestRunScopeRef.current = (event, mode, scope) =>
      void runScope(event, mode, scope);
    latestOverviewRef.current = (appointmentId) =>
      void handleShowOverview(appointmentId);
    latestRespondRef.current = (recipientId, status) =>
      void handleRespond(recipientId, status);
  });

  const calendarEvents = calendarError ? [] : (data?.events ?? []);

  const personalCalendar = (
    <PersonalCalendar
      // Titel, Zeitnavigation und Umschalter trägt die Kopfkarte der Seite
      // (`PersonalCalendarChrome`); hier bleibt nur das Raster.
      // On a load error SWR may still hold the previous range's data; don't
      // render stale appointments under the new date label.
      events={calendarEvents}
      referenceDate={referenceDate}
      viewMode={viewMode}
      showWeekend={showWeekend}
      onShowOverview={handleShowOverview}
      onRespond={handleRespond}
      respondingRecipientId={respondingRecipientId}
      onEdit={canManageCalendar ? handleEdit : undefined}
      onCancel={canManageCalendar ? handleCancel : undefined}
      onDelete={canManageCalendar ? handleDelete : undefined}
      busyAppointmentId={busyAppointmentId}
      icsHrefBase="/api/calendar/appointments"
    />
  );

  // Statuszeile: die Zahl der Termine im sichtbaren Zeitraum. Den Zeitraum
  // selbst führt die Bedienzeile der Zeitnavigation als `dateLabel`, er steht
  // deshalb nicht ein zweites Mal in der Kopfkarte.
  const eventCount = visibleCalendarEvents(
    calendarEvents,
    showWeekend,
    viewMode,
  ).length;
  const statusLine =
    isLoading || calendarError
      ? undefined
      : `${eventCount} ${eventCount === 1 ? "Termin" : "Termine"}`;

  // Laden, Fehler und Leerzustand gehören dem Gerüst (Bauart 3 Regel 5). Sie
  // gelten nur für den Kalender-Reiter — der Betreuungsplan-Reiter bringt
  // seine eigenen Zustände mit.
  const onCalendarTab = activeTab === "meine";
  // Ladefehler mit Katalogtext, Wiederholen und Vorgangskennung (#2517).
  const calendarLoadError = useSwrLoadError(
    calendarError,
    "die Liste der Termine",
    () => mutate(),
  );
  // Bei einem Ladefehler steht nie ein leeres Raster unter dem Datum (siehe
  // `calendarError` unten); der Abo-Bereich bleibt erreichbar.
  const calendarLoading = onCalendarTab && isLoading;
  const calendarErrorState =
    onCalendarTab && calendarError && calendarLoadError
      ? calendarLoadError
      : null;
  const calendarEmpty =
    onCalendarTab && !isLoading && !calendarError && eventCount === 0
      ? {
          title: calendarEmptyLabel(viewMode),
          description: canManageCalendar
            ? "Legen Sie einen Termin an oder wechseln Sie den Zeitraum."
            : "Wechseln Sie den Zeitraum, um andere Termine zu sehen.",
          action: canManageCalendar ? (
            <Button
              type="button"
              variant="primary"
              size="md"
              className="gap-1.5"
              onClick={handleCreate}
            >
              <Plus className="h-4 w-4" aria-hidden />
              Neuer Termin
            </Button>
          ) : undefined,
        }
      : null;

  const personalCalendarWithSubscribe = (
    // Der Abo-Bereich (#2621) bleibt auch in einer leeren Woche erreichbar,
    // deshalb ersetzt der Leerzustand hier nur das Raster, nicht den Inhalt.
    <>
      {calendarError ? null : calendarEmpty ? (
        <SectionCard>
          <EmptyState
            title={calendarEmpty.title}
            description={calendarEmpty.description}
            action={calendarEmpty.action}
          />
        </SectionCard>
      ) : (
        personalCalendar
      )}
      <CalendarSubscribePanel audience="staff" calDAVEnabled={calDAVEnabled} />
    </>
  );

  return (
    <TenantPage
      title="Mein Kalender"
      stats={onCalendarTab ? statusLine : undefined}
      statsLoading={calendarLoading}
      loading={calendarLoading}
      error={
        calendarErrorState
          ? {
              message: calendarErrorState.message,
              action: errorAlertActions(calendarErrorState),
              keepContent: true,
            }
          : null
      }
      actions={
        canManageCalendar && onCalendarTab ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            className="gap-1.5"
            onClick={handleCreate}
          >
            <Plus className="h-4 w-4" aria-hidden />
            Neuer Termin
          </Button>
        ) : undefined
      }
      searchSlot={
        onCalendarTab ? (
          <PersonalCalendarChrome
            events={calendarEvents}
            referenceDate={referenceDate}
            viewMode={viewMode}
            showWeekend={showWeekend}
            onShowWeekendChange={setShowWeekend}
            onDateChange={setReferenceDate}
            onViewModeChange={setViewMode}
          />
        ) : undefined
      }
      tabs={
        showSchoolPlanTab
          ? {
              value: activeTab,
              onChange: handleCalendarTabChange,
              items: [
                { value: "meine", label: "Meine Termine" },
                { value: "schule", label: "Betreuungsplan" },
              ],
              label: "Kalenderbereiche",
            }
          : undefined
      }
      overlays={
        <>
          {/* Das Terminformular ist laenger als sechs Felder und steht deshalb
              im Panel neben dem Kalender, nicht als Fenster darueber. */}
          <SlideOver
            open={formOpen && canManageCalendar}
            onOpenChange={(open) => {
              if (!open && !submitting) resetForm();
            }}
          >
            <SlideOverContent widthClass="sm:w-[860px]">
              <SlideOverHeader className="flex-row items-start justify-between gap-3">
                <div className="min-w-0">
                  <SlideOverTitle>
                    {editingId ? "Termin bearbeiten" : "Termin erstellen"}
                  </SlideOverTitle>
                </div>
                <SlideOverCloseButton disabled={submitting} />
              </SlideOverHeader>
              <SlideOverBody error={formErrors.error}>
                <div ref={formRef}>
                  <form className="space-y-5" onSubmit={handleSubmit}>
                    <div className="flex items-center gap-2">
                      <MotoConceptIcon concept="calendarPeriods" size={20} />
                      <p className="text-sm text-gray-600">
                        {editingId
                          ? "Passen Sie Zeitpunkt und Details an. Empfänger und Antwortregel bleiben unverändert."
                          : "Legen Sie Zeitpunkt, Antwortregel und Empfängergruppen fest."}
                      </p>
                    </div>

                    <div className="grid gap-4 md:grid-cols-2">
                      <Input
                        label="Titel"
                        name="title"
                        value={title}
                        onChange={(event) => setTitle(event.target.value)}
                        error={formErrors.fieldError("title")}
                        disabled={submitting}
                        required
                      />
                      <Input
                        label="Ort"
                        name="location"
                        value={location}
                        onChange={(event) => setLocation(event.target.value)}
                        error={formErrors.fieldError("location")}
                        disabled={submitting}
                      />
                      <ISODatePicker
                        label="Startdatum"
                        id="calendar-start-date"
                        name="start_date"
                        controlSize="lg"
                        value={startDate}
                        error={formErrors.fieldError("start_date")}
                        onChange={(next) => {
                          setStartDate(next);
                          if (endDate < next) setEndDate(next);
                        }}
                        disabled={submitting}
                        calendarLayout="popover"
                        hideClearButton
                        required
                      />
                      <ISODatePicker
                        label="Enddatum"
                        id="calendar-end-date"
                        name="end_date"
                        controlSize="lg"
                        value={endDate}
                        error={formErrors.fieldError("end_date")}
                        min={startDate}
                        onChange={setEndDate}
                        disabled={submitting}
                        calendarLayout="popover"
                        hideClearButton
                        required
                      />
                      <Input
                        label="Startzeit"
                        name="start_time"
                        type="time"
                        value={startTime}
                        error={formErrors.fieldError("start_time")}
                        onChange={(event) => setStartTime(event.target.value)}
                        disabled={submitting || allDay}
                        required
                      />
                      <Input
                        label="Endzeit"
                        name="end_time"
                        type="time"
                        value={endTime}
                        error={formErrors.fieldError("end_time")}
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

                    <label
                      htmlFor="calendar-send-email"
                      className="flex items-start gap-2 text-sm font-medium text-gray-700"
                    >
                      <Checkbox
                        id="calendar-send-email"
                        checked={sendEmail}
                        onChange={(event) => setSendEmail(event.target.checked)}
                        disabled={submitting}
                      />
                      <span>
                        Eltern per E-Mail benachrichtigen
                        <span className="mt-0.5 block text-xs font-normal text-gray-500">
                          Sendet eine E-Mail mit Titel und Termin an die
                          eingeladenen Eltern. Ohne Haken erscheint der Termin
                          nur im Eltern-Portal.
                        </span>
                      </span>
                    </label>

                    <label className="block">
                      <span className="mb-2 block text-sm font-medium text-gray-700">
                        Beschreibung
                      </span>
                      <textarea
                        className="block min-h-24 w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
                        value={description}
                        onChange={(event) => setDescription(event.target.value)}
                        disabled={submitting}
                      />
                    </label>

                    <div
                      className={`grid gap-4 ${editingId ? "" : "md:grid-cols-2"}`}
                    >
                      {!editingId ? (
                        <label
                          htmlFor="calendar-delivery-mode"
                          className="block"
                        >
                          <span className="mb-2 block text-sm font-medium text-gray-700">
                            Antwortregel
                          </span>
                          <CustomSelect
                            id="calendar-delivery-mode"
                            ariaLabel="Antwortregel"
                            value={deliveryMode}
                            onChange={(next) =>
                              setDeliveryMode(next as CalendarDeliveryMode)
                            }
                            disabled={submitting}
                            options={[
                              {
                                value: "rsvp_required",
                                label:
                                  "Antwort erforderlich: Zusage oder Absage",
                              },
                              {
                                value: "informational",
                                label:
                                  "Nur informieren: ohne Rückmeldung eintragen",
                              },
                            ]}
                          />
                        </label>
                      ) : null}
                      <label
                        htmlFor="calendar-overview-visibility"
                        className="block"
                      >
                        <span className="mb-2 block text-sm font-medium text-gray-700">
                          Teilnehmerübersicht
                        </span>
                        <CustomSelect
                          id="calendar-overview-visibility"
                          ariaLabel="Teilnehmerübersicht"
                          value={overviewVisibility}
                          onChange={(next) =>
                            setOverviewVisibility(
                              next as CalendarOverviewVisibility,
                            )
                          }
                          disabled={submitting}
                          options={[
                            { value: "organizer", label: "Nur ich" },
                            {
                              value: "staff",
                              label: "Mitarbeitende mit Termin",
                            },
                            { value: "all", label: "Alle Eingeladenen" },
                          ]}
                        />
                      </label>
                    </div>

                    {!editingId ? (
                      <div className="rounded-lg border border-gray-200 bg-white p-3">
                        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                          <h3 className="text-sm font-semibold text-gray-900">
                            Empfänger auswählen
                          </h3>
                          <span className="text-xs text-gray-500">
                            {targets.length} Ziel
                            {targets.length === 1 ? "" : "e"} ausgewählt
                          </span>
                        </div>
                        <div className="mb-3">
                          <Input
                            label="Ziele suchen"
                            name="calendar-target-search"
                            value={targetSearch}
                            onChange={(event) =>
                              setTargetSearch(event.target.value)
                            }
                            disabled={submitting}
                            placeholder="Name, Klasse oder Gruppe"
                          />
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
                                    const selected = selectedKeys.has(
                                      choice.key,
                                    );
                                    const disabled =
                                      submitting ||
                                      (!selected && choice.covered);
                                    const checkboxId = `calendar-target-${choice.key.replace(/[^a-z0-9_-]/gi, "-")}`;
                                    return (
                                      <label
                                        key={choice.key}
                                        htmlFor={checkboxId}
                                        className={`flex items-center gap-2 rounded-md border px-2 py-1.5 text-sm transition-colors ${
                                          selected
                                            ? "border-gray-900 bg-white text-gray-950"
                                            : choice.covered
                                              ? "bg-moto-green-soft border-gray-200 text-gray-500"
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
                                          <span className="text-xs font-medium text-gray-500">
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
                    ) : null}

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
                      <label htmlFor="calendar-frequency" className="block">
                        <span className="mb-2 block text-sm font-medium text-gray-700">
                          Wiederholung
                        </span>
                        <CustomSelect
                          id="calendar-frequency"
                          ariaLabel="Wiederholung"
                          value={frequency}
                          onChange={(next) =>
                            setFrequency(next as RecurrenceFrequency)
                          }
                          disabled={submitting}
                          options={[
                            { value: "none", label: "Keine" },
                            { value: "daily", label: "Täglich" },
                            { value: "weekly", label: "Wöchentlich" },
                            { value: "monthly", label: "Monatlich" },
                            { value: "yearly", label: "Jährlich" },
                          ]}
                        />
                      </label>
                      <Input
                        label="Intervall"
                        name="calendar-recurrence-interval"
                        type="number"
                        min={1}
                        value={intervalCount}
                        onChange={(event) =>
                          setIntervalCount(
                            Number.parseInt(event.target.value, 10) || 1,
                          )
                        }
                        disabled={submitting || frequency === "none"}
                      />
                      <ISODatePicker
                        label="Endet am"
                        id="calendar-recurrence-end"
                        controlSize="lg"
                        value={endsOn}
                        min={startDate}
                        onChange={setEndsOn}
                        disabled={submitting || frequency === "none"}
                        calendarLayout="popover"
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
                                    : current.filter(
                                        (value) => value !== day.value,
                                      ),
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
                        onClick={resetForm}
                        disabled={submitting}
                      >
                        Abbrechen
                      </Button>
                      <Button
                        type="submit"
                        size="md"
                        isLoading={submitting}
                        loadingText="Speichert…"
                      >
                        {editingId
                          ? "Änderungen speichern"
                          : "Termin speichern"}
                      </Button>
                    </div>
                  </form>
                </div>
              </SlideOverBody>
            </SlideOverContent>
          </SlideOver>

          <Modal
            isOpen={overview !== null || overviewLoading}
            onClose={() => {
              if (!overviewLoading) setOverview(null);
            }}
            title="Teilnehmer"
            widthClass="mx-4 w-[calc(100%-2rem)] max-w-xl"
          >
            {overviewLoading ? (
              <SkeletonRegion label="Teilnehmer werden geladen…">
                <ListSkeleton rows={4} avatar={false} />
              </SkeletonRegion>
            ) : overview ? (
              <CalendarOverviewList overview={overview} />
            ) : null}
          </Modal>

          <ConfirmDeleteModal
            isOpen={deleteTarget !== null}
            title="Termin löschen"
            description={
              <p className="text-gray-700">
                {deleteTarget?.recurring
                  ? "Dieser Termin gehört zu einer Reihe. Die Empfänger sehen gelöschte Termine nicht mehr."
                  : "Der Termin wird gelöscht. Die Empfänger sehen ihn dann nicht mehr."}
              </p>
            }
            scope={
              deleteTarget?.recurring
                ? {
                    label: "Was soll gelöscht werden?",
                    name: "calendar-delete-scope",
                    value: deleteScope,
                    onChange: (value) =>
                      setDeleteScope(
                        value === "occurrence" ? "occurrence" : "series",
                      ),
                    options: [
                      {
                        value: "occurrence",
                        label: "Nur diesen Termin",
                        description: "Die Reihe bleibt bestehen.",
                      },
                      {
                        value: "series",
                        label: "Ganze Reihe",
                        description:
                          "Alle Termine dieser Reihe werden gelöscht.",
                      },
                    ],
                  }
                : undefined
            }
            gate={{ mode: "twoStep", firstStepLabel: "Löschen" }}
            confirmLabel={
              deleteTarget?.recurring ? "Löschen" : "Endgültig löschen"
            }
            onConfirm={() => {
              if (deleteTarget && deleteScope) {
                void runScope(deleteTarget, "delete", deleteScope);
              }
            }}
            onClose={() => {
              if (!busyAppointmentId) {
                setDeleteTarget(null);
                clearDeleteErrors();
              }
            }}
            loading={Boolean(busyAppointmentId)}
            error={deleteErrors.error}
          />

          <ChoiceModal
            isOpen={cancelTarget?.recurring === true}
            onClose={() => {
              if (!busyAppointmentId) setCancelTarget(null);
            }}
            title="Termin absagen"
            description="Dieser Termin gehört zu einer Reihe. Die Empfänger sehen abgesagte Termine als „Abgesagt“."
            options={[
              {
                value: "occurrence",
                label: "Nur diesen Termin absagen",
                description: "Die Reihe bleibt bestehen.",
              },
              {
                value: "series",
                label: "Ganze Reihe absagen",
                description: "Alle Termine dieser Reihe werden abgesagt.",
              },
            ]}
            onSelect={(value) => {
              if (cancelTarget) {
                void runScope(
                  cancelTarget,
                  "cancel",
                  value === "occurrence" ? "occurrence" : "series",
                );
              }
            }}
            isBusy={Boolean(busyAppointmentId)}
          />

          <ConfirmationModal
            isOpen={cancelTarget !== null && cancelTarget.recurring !== true}
            onClose={() => {
              if (!busyAppointmentId) setCancelTarget(null);
            }}
            onConfirm={() => {
              if (cancelTarget) void runScope(cancelTarget, "cancel", "series");
            }}
            title="Termin absagen"
            confirmText="Absagen"
            isConfirmLoading={Boolean(busyAppointmentId)}
            isDismissDisabled={Boolean(busyAppointmentId)}
          >
            <p className="text-sm text-gray-700">
              Möchten Sie diesen Termin absagen? Die Empfänger sehen ihn als
              „Abgesagt“.
            </p>
          </ConfirmationModal>
        </>
      }
    >
      {showSchoolPlanTab && activeTab === "schule" ? (
        // BetreuungsplanView liest Search-Params (d/view/block) und
        // braucht deshalb eine Suspense-Grenze.
        <Suspense fallback={null}>
          <SchoolPlanReadView />
        </Suspense>
      ) : (
        personalCalendarWithSubscribe
      )}
    </TenantPage>
  );
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
function appointmentFieldErrors(
  title: string,
  startDate: string,
  endDate: string,
): Record<string, string> {
  const fields: Record<string, string> = {};
  if (!title.trim()) fields.title = "Bitte geben Sie einen Titel ein.";
  if (!startDate) fields.start_date = "Bitte wählen Sie den ersten Tag.";
  if (!endDate) fields.end_date = "Bitte wählen Sie den letzten Tag.";
  return fields;
}

/** Wer was bei einer Absage oder Löschung nicht bekommen hat. */
function scopeErrorObject(
  mode: "cancel" | "delete",
  scope: "occurrence" | "series",
): string {
  if (mode === "cancel") return "die Absage des Termins";
  return scope === "occurrence"
    ? "das Entfernen des Termins"
    : "das Löschen des Termins";
}

function scopeSuccessMessage(
  event: CalendarEvent,
  mode: "cancel" | "delete",
  scope: "occurrence" | "series",
): string {
  const name = `„${event.title}“`;
  if (scope === "occurrence") {
    return mode === "cancel"
      ? `Der Termin ${name} ist an diesem Tag abgesagt.`
      : `Der Termin ${name} ist an diesem Tag entfernt.`;
  }
  const subject = event.recurring
    ? `Die Terminreihe ${name}`
    : `Der Termin ${name}`;
  return mode === "cancel"
    ? `${subject} ist abgesagt.`
    : `${subject} ist gelöscht.`;
}

export default function StaffCalendarPage() {
  // useSearchParams braucht eine Suspense-Grenze (Next.js 16).
  return (
    <Suspense fallback={null}>
      <StaffCalendarPageInner />
    </Suspense>
  );
}
