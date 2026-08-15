"use client";

/**
 * InstanceDetailModal — centered modal showing the full state of a
 * clicked instance plus lifecycle action buttons (#1956).
 *
 * Shows the operational state of one timetable instance: lifecycle,
 * assigned staff, children, attendance state, and admin corrections.
 */

import { useEffect, useMemo, useState } from "react";
import type React from "react";
import Link from "next/link";
import {
  Check,
  CheckCircle2,
  CircleX,
  Palette,
  Pencil,
  Repeat,
  RotateCcw,
  StickyNote,
  Trash2,
  TriangleAlert,
  UserPlus,
  X,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { OriginChip } from "~/components/ui/origin-chip";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import { berlinTodayISO, formatDate, parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { canCompleteInstance } from "~/lib/timetable-lifecycle";
import { useSWRAuth } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import type { InstanceParticipantNames } from "~/lib/timetable-api";
import {
  getActivityTypeBadge,
  getGermanWeekdayAdverb,
  getGermanWeekdayLong,
  getStatusLabel,
} from "~/lib/timetable-helpers";
import {
  timetableDangerPanel,
  timetableMutedSurface,
  timetableNestedSurface,
} from "./timetable-style";
import {
  attendanceStaffTone,
  attendanceStudentTone,
  capacityTone,
  TimetableRatioPill,
} from "./timetable-ratio-pill";
import type {
  AttendancePatchBody,
  EnrichedInstance,
  InstanceStudentSummary,
  InstanceStatus,
} from "~/lib/timetable-types";
import { isNotScheduledRow } from "~/lib/timetable-types";

export type LifecycleAction = "start" | "complete" | "cancel" | "reopen";

type PendingConfirmAction = "complete" | "cancel" | "delete" | "reopen";

const CONFIRM_DIALOGS: Record<
  PendingConfirmAction,
  {
    title: string;
    body: string;
    confirmText: string;
    confirmButtonClass?: string;
  }
> = {
  complete: {
    title: "Termin beenden?",
    body: "Der laufende Termin wird beendet. Innerhalb von fünf Minuten kann er kontrolliert wieder geöffnet werden.",
    confirmText: "Beenden",
  },
  cancel: {
    title: "Termin absagen?",
    body: "Der Termin wird im Plan als abgesagt markiert. Das kann nicht rückgängig gemacht werden.",
    confirmText: "Absagen",
  },
  delete: {
    title: "Abgesagten Termin löschen?",
    body: "Der abgesagte Termin wird dauerhaft entfernt.",
    confirmText: "Löschen",
    confirmButtonClass: "bg-moto-red hover:bg-moto-red-strong",
  },
  reopen: {
    title: "Termin wieder öffnen?",
    body: "Die Aufsicht sowie die Anwesenheiten zum Zeitpunkt des Abschlusses werden wiederhergestellt.",
    confirmText: "Wieder öffnen",
  },
};

interface InstanceDetailModalProps {
  instance: EnrichedInstance | null;
  onClose: () => void;
  onLifecycleAction: (action: LifecycleAction) => Promise<void>;
  onDeleteCancelled?: (instance: EnrichedInstance) => Promise<void>;
  onDeleteFollowing?: (instance: EnrichedInstance) => Promise<void>;
  onEdit?: (instance: EnrichedInstance) => void;
  onRepeat?: (instance: EnrichedInstance) => void;
  staffNames?: Map<string, string>;
  studentNames?: Map<string, string>;
  onAttendancePatch?: (
    instanceId: string,
    studentId: string,
    body: AttendancePatchBody,
  ) => Promise<void>;
  /**
   * When true, edit + spontaneous-create UI surfaces are visible but
   * disabled with a tooltip. Default true until backend PUT/POST land.
   */
  editDeferred?: boolean;
  /**
   * Blendet das Detail-Modal aus, solange ein anderes Overlay (z.B. der
   * Termin-Editor) offen ist: beide portalen auf denselben festen z-index,
   * und das Kit-Modal lauscht dokumentweit auf Escape — gestapelt würde es
   * den Editor verdecken und bei Escape mitschließen. Die Auswahl
   * (?block=…) bleibt bestehen, das Modal erscheint danach wieder.
   */
  suspended?: boolean;
  /**
   * Öffnet den Personalpool (#1884) für diesen Block. Nur sichtbar für
   * geplante/laufende Blöcke ab heute — vergangene Blöcke sind Historie.
   */
  onOpenPool?: (instance: EnrichedInstance) => void;
  /**
   * Controls whether the pool opener describes a write action. View-only users
   * may still inspect the schedules:read pool, but cannot move or assign staff.
   * Defaults false so a missing permission prop never exposes mutation chrome.
   */
  canManageStaffPool?: boolean;
  /**
   * Leseansicht (#2283): false blendet sämtliche Termin-Aktionen im Footer
   * aus (Lebenszyklus, Vertretungs-Link, Löschen). Default true, damit
   * bestehende Admin-Aufrufer unverändert bleiben; die Callback-gebundenen
   * Aktionen (Bearbeiten, Wiederholen, Anwesenheit) steuert weiterhin der
   * Aufrufer über die Props.
   */
  canManage?: boolean;
  /**
   * Leseansicht (#2283): true lädt die Kindernamen über den schmalen
   * Teilnehmer-Endpunkt (schedules:read) statt über die users:read-gegatete
   * studentNames-Map des Aufrufers.
   */
  fetchParticipantNames?: boolean;
}

const EMPTY_STAFF_NAMES = new Map<string, string>();
const EMPTY_STUDENT_NAMES = new Map<string, string>();

function germanFullDate(iso: string): string {
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  const day = String(d.getDate()).padStart(2, "0");
  const month = String(d.getMonth() + 1).padStart(2, "0");
  return `${getGermanWeekdayLong(d)}, ${day}.${month}.${d.getFullYear()}`;
}

/**
 * Regeltermin-Herkunftstext für den OriginChip im Detail-Modal
 * (docs/planung-redesign/docs/06-betreuungsplan.md Abschnitt 3.2: "aus
 * Regeltermin {Titel}, montags 12:00"). Die Instanz trägt keinen separaten
 * Template-Titel — materialisierte Instanzen erben den Titel des
 * Regeltermins 1:1 (timetable-helpers.ts Mapper), daher genügt
 * `instance.title`. Der Wochentag wird aus dem Instanzdatum abgeleitet
 * (materialisierte Instanzen liegen exakt auf dem Regeltermin-Wochentag)
 * und als Adverb kleingeschrieben ("Montag" -> "montags").
 */
function regelterminOriginLabel(instance: EnrichedInstance): string {
  const weekdayLong = getGermanWeekdayLong(parseISODate(instance.date));
  return [
    `aus Regeltermin ${instance.title},`,
    getGermanWeekdayAdverb(weekdayLong),
    instance.startTime,
  ]
    .filter(Boolean)
    .join(" ");
}

interface StatusBadgeProps {
  status: InstanceStatus;
}

function StatusBadge({ status }: StatusBadgeProps) {
  const palette: Record<InstanceStatus, { bg: string; text: string }> = {
    planned: { bg: "#F3F4F6", text: "#374151" },
    active: { bg: LOCATION_COLORS.GROUP_ROOM, text: "#FFFFFF" },
    completed: { bg: "#E5E7EB", text: "#6B7280" },
    cancelled: { bg: LOCATION_COLORS.DANGER, text: "#FFFFFF" },
  };
  const { bg, text } = palette[status];
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase"
      style={{ backgroundColor: bg, color: text }}
    >
      {status === "active" && (
        <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-white" />
      )}
      {getStatusLabel(status)}
    </span>
  );
}

function attendanceLabel(
  status: InstanceStudentSummary["status"],
  plannedContext = false,
): string {
  switch (status) {
    case "expected":
      return "Erwartet";
    case "present":
      return "Anwesend";
    case "absent":
      return plannedContext ? "Abgemeldet" : "Fehlt";
  }
}

function attendanceSubstatusLabel(
  substatus: NonNullable<InstanceStudentSummary["substatus"]>,
): string {
  switch (substatus) {
    case "late":
      return "Verspätet";
    case "excused":
      return "Entschuldigt";
    case "sick":
      return "Krank";
    case "field_trip":
      return "Ausflug";
    case "other":
      return "Sonstiges";
  }
}

/**
 * Row label for a child the care plan does not place here today (#1747). The
 * assignment row still says "expected", so labelling it "Erwartet" would
 * contradict the header count, which leaves the child out.
 */
function careDayLabel(status: InstanceStudentSummary["careDayStatus"]): string {
  return status === "cancelled" ? "Heute abgemeldet" : "Heute nicht eingeplant";
}

/** Neutral surface for rows grouped as "heute nicht eingeplant". */
const NOT_SCHEDULED_TONE = "border-gray-200 bg-gray-50 text-gray-600";

function attendanceTone(status: InstanceStudentSummary["status"]): string {
  switch (status) {
    case "present":
      return "border-moto-green/20 bg-moto-green/10 text-moto-green-strong";
    case "absent":
      return "border-moto-red/20 bg-moto-red/10 text-moto-red-strong";
    case "expected":
      return "border-gray-200 bg-gray-50 text-gray-600";
  }
}

function fallbackStudentRows(studentIds: string[]): InstanceStudentSummary[] {
  return studentIds.map((studentId) => ({
    studentId,
    status: "expected",
    careDayStatus: "unknown",
  }));
}

function studentsForInstance(
  instance: EnrichedInstance | null,
): InstanceStudentSummary[] {
  if (!instance) return [];
  if (instance.students.length > 0) return instance.students;
  return fallbackStudentRows(instance.studentIds);
}

function attendancePatchForInstance(
  attendanceWebEnabled: boolean,
  instance: EnrichedInstance | null,
  onAttendancePatch: InstanceDetailModalProps["onAttendancePatch"],
): InstanceDetailModalProps["onAttendancePatch"] {
  if (!attendanceWebEnabled || !instance) return undefined;
  if (instance.status === "cancelled" || instance.status === "completed") {
    return undefined;
  }
  return onAttendancePatch;
}

/**
 * Parent callbacks own user-facing error reporting and rethrow so their callers
 * can react. This drawer only owns pending UI state, so it consumes that already
 * reported rejection and tells the local flow whether the mutation succeeded.
 */
async function awaitReportedAction(
  action: () => Promise<void>,
): Promise<boolean> {
  try {
    await action();
    return true;
  } catch {
    return false;
  }
}

function ActivityTypeBadge({
  activityType,
}: Readonly<{ activityType: EnrichedInstance["activityType"] }>) {
  const badge = getActivityTypeBadge(activityType);
  if (!badge) return null;
  return (
    <span
      className="rounded-full px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase"
      style={{ backgroundColor: badge.bg }}
    >
      {badge.label}
    </span>
  );
}

function AssignedStaffSection({
  instance,
  staffNames,
  onOpenPool,
  canManageStaffPool,
}: Readonly<{
  instance: EnrichedInstance;
  staffNames: Map<string, string>;
  onOpenPool?: (instance: EnrichedInstance) => void;
  canManageStaffPool: boolean;
}>) {
  return (
    <Section title="Personal">
      {instance.staff.length === 0 ? (
        <EmptyLine>Kein Personal zugeordnet.</EmptyLine>
      ) : (
        <div className="space-y-1.5">
          {instance.staff.map((item) => (
            <PersonLine
              key={item.staffId}
              name={staffNames.get(item.staffId) ?? `Personal #${item.staffId}`}
              meta={[
                item.isPrimary ? "Zuständig" : null,
                item.isAbsent ? "Abwesend" : null,
                item.isSubstitute ? "Ersatz" : null,
              ]}
              danger={item.isAbsent}
            />
          ))}
        </div>
      )}
      {onOpenPool && (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() => onOpenPool(instance)}
          className="border border-gray-200 text-gray-600 hover:border-gray-300 hover:bg-gray-50"
        >
          <span className="inline-flex items-center gap-1.5">
            {canManageStaffPool ? (
              <UserPlus className="h-3.5 w-3.5" />
            ) : (
              <MotoConceptIcon concept="staff" size={16} />
            )}
            {canManageStaffPool ? "Person hinzuziehen" : "Personalpool ansehen"}
          </span>
        </Button>
      )}
    </Section>
  );
}

/**
 * Leseansicht (#2283): lädt die Kindernamen pro Termin über den schmalen
 * Teilnehmer-Endpunkt (schedules:read). Eigene Komponente statt Hook im
 * Modal, damit der Session-abhängige SWR-Aufruf nur gemountet wird, wenn
 * die Leseansicht ihn wirklich braucht; gefilterte Kinder fallen in der
 * Anzeige auf "Kind #ID" zurück.
 */
function ParticipantNamesLoader({
  instanceId,
  children,
}: Readonly<{
  instanceId: string;
  children: (names: InstanceParticipantNames) => React.ReactNode;
}>) {
  const { data } = useSWRAuth(`timetable-participants-${instanceId}`, () =>
    timetableService.getInstanceParticipants(instanceId),
  );
  return (
    <>
      {children(
        data ?? {
          studentNames: EMPTY_STUDENT_NAMES,
          staffNames: EMPTY_STAFF_NAMES,
        },
      )}
    </>
  );
}

function InstanceStudentsSection({
  groupedStudents,
  handleAttendancePatch,
  instance,
  onAttendancePatch,
  pendingStudentId,
  studentNames,
  students,
}: Readonly<{
  groupedStudents: Record<
    InstanceStudentSummary["status"] | "notScheduled",
    InstanceStudentSummary[]
  >;
  handleAttendancePatch: (
    studentId: string,
    body: AttendancePatchBody,
  ) => Promise<void>;
  instance: EnrichedInstance;
  onAttendancePatch?: InstanceDetailModalProps["onAttendancePatch"];
  pendingStudentId: string | null;
  studentNames: Map<string, string>;
  students: InstanceStudentSummary[];
}>) {
  if (students.length === 0) {
    const reason = instance.emptyRosterReason;
    let message = "Keine Kinder geplant.";
    if (reason?.kind === "before_offering_start" && reason.serviceStartDate) {
      message = `Dieser Termin liegt vor dem Betreuungsbeginn am ${formatDate(reason.serviceStartDate)}. Die Kinder aus den ausgewählten Angeboten werden erst ab diesem Tag übernommen.`;
    } else if (reason?.kind === "offering_source_empty") {
      message =
        "Aus den ausgewählten Angeboten wurden für diesen Termin keine Kinder übernommen. Das kann an den gebuchten Wochentagen, den gewählten Filtern oder geänderten Anmeldungen liegen.";
    }
    return (
      <Section title="Kinder">
        {reason ? (
          <Alert type="info" message={message} announce="polite" />
        ) : (
          <EmptyLine>{message}</EmptyLine>
        )}
      </Section>
    );
  }

  // The care-day group carries exactly one attendance action: "anwesend", for
  // the child who is not in care today and turns up anyway (#1747 review).
  // "abmelden" and "zurücksetzen" stay out — they would write attendance for a
  // day the counts already treat as not in care.
  const groups = [
    { key: "expected", status: "expected", rows: groupedStudents.expected },
    {
      key: "not-scheduled",
      status: "expected",
      rows: groupedStudents.notScheduled,
      careDayGroup: true,
    },
    { key: "present", status: "present", rows: groupedStudents.present },
    { key: "absent", status: "absent", rows: groupedStudents.absent },
  ] as const;

  return (
    <Section title="Kinder">
      <div className="space-y-3">
        {groups.map((group) => (
          <StudentGroup
            key={group.key}
            status={group.status}
            careDayGroup={"careDayGroup" in group}
            students={group.rows}
            studentNames={studentNames}
            pendingStudentId={pendingStudentId}
            onAttendancePatch={onAttendancePatch}
            instanceStatus={instance.status}
            handleAttendancePatch={handleAttendancePatch}
          />
        ))}
      </div>
    </Section>
  );
}

export function InstanceDetailModal({
  instance,
  onClose,
  onLifecycleAction,
  onDeleteCancelled,
  onDeleteFollowing,
  onEdit,
  onRepeat,
  staffNames = EMPTY_STAFF_NAMES,
  studentNames = EMPTY_STUDENT_NAMES,
  onAttendancePatch,
  editDeferred = true,
  suspended = false,
  onOpenPool,
  canManageStaffPool = false,
  canManage = true,
  fetchParticipantNames = false,
}: InstanceDetailModalProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const showTimetableCounts = useShowTimetableCounts();
  // Tenant-bewusster Pfad: im Path-Routing-Modus muss /vertretung den
  // /{slug}-Präfix tragen, sonst führt der Link ins Leere.
  const tenantPath = useTenantAwarePath();
  const today = useBerlinToday();
  const now = useMinuteClock();
  const completeEnabled = canCompleteInstance(
    instance?.canComplete === true,
    instance?.completeAvailableAt ?? "",
    now,
  );
  const [pendingAction, setPendingAction] = useState<LifecycleAction | null>(
    null,
  );
  const [pendingConfirm, setPendingConfirm] =
    useState<PendingConfirmAction | null>(null);
  const [pendingDelete, setPendingDelete] = useState(false);
  const [deleteScopeOpen, setDeleteScopeOpen] = useState(false);
  const [pendingDeleteScope, setPendingDeleteScope] = useState<string | null>(
    null,
  );
  const [pendingStudentId, setPendingStudentId] = useState<string | null>(null);
  // Personalpool (#1884) nur für bearbeitbare Blöcke ab heute — vergangene
  // Blöcke sind Historie (gleiche Regel wie das Backend erzwingt).
  const poolAvailable =
    instance !== null &&
    (instance.status === "planned" || instance.status === "active") &&
    instance.date >= today;
  const students = useMemo(() => studentsForInstance(instance), [instance]);
  // Same split the header counts use (#1747): an assignment row still reads
  // "expected" when the care plan does not place the child here today, so it
  // gets its own group instead of padding "Erwartet" with children the
  // expected count — and the staffing maths — deliberately leave out.
  const groupedStudents = useMemo(
    () => ({
      expected: students.filter(
        (student) =>
          student.status === "expected" &&
          !isNotScheduledRow(student.status, student.careDayStatus),
      ),
      notScheduled: students.filter((student) =>
        isNotScheduledRow(student.status, student.careDayStatus),
      ),
      present: students.filter((student) => student.status === "present"),
      absent: students.filter(
        (student) =>
          student.status === "absent" &&
          !isNotScheduledRow(student.status, student.careDayStatus),
      ),
    }),
    [students],
  );

  useEffect(() => {
    setPendingConfirm(null);
    setDeleteScopeOpen(false);
    setPendingDeleteScope(null);
  }, [instance?.id]);

  const handleLifecycle = async (action: LifecycleAction) => {
    setPendingAction(action);
    try {
      await awaitReportedAction(() => onLifecycleAction(action));
    } finally {
      setPendingAction(null);
    }
  };

  const handleDeleteCancelled = async (): Promise<boolean> => {
    if (!instance || !onDeleteCancelled) return false;
    setPendingDelete(true);
    try {
      return await awaitReportedAction(() => onDeleteCancelled(instance));
    } finally {
      setPendingDelete(false);
    }
  };

  const handleDeleteFollowing = async (): Promise<boolean> => {
    if (!instance || !onDeleteFollowing) return false;
    setPendingDelete(true);
    try {
      return await awaitReportedAction(() => onDeleteFollowing(instance));
    } finally {
      setPendingDelete(false);
    }
  };

  // "Ab jetzt dauerhaft" beendet den Regeltermin ab dem Datum des Termins —
  // das Backend lehnt ein Datum vor heute ab, weil das Vergangenheit löschen
  // würde (template_split_service: effective_date must not be in the past).
  // Für einen vergangenen Termin bleibt daher nur das Löschen dieser einen
  // Woche, also entfällt die Auswahl komplett statt eine Option anzubieten,
  // die zwangsläufig scheitert.
  const canEndSeries = (currentToday: string) =>
    instance !== null &&
    Boolean(instance.activityGroupId) &&
    !instance.isSpontaneous &&
    Boolean(onDeleteFollowing) &&
    instance.date >= currentToday;
  const seriesEndAvailable = canEndSeries(today);

  // If the scope dialog was opened before Berlin midnight, its recurring
  // option becomes invalid at the rollover. Continue with the only valid
  // deletion flow instead of leaving a stale option that the backend rejects.
  useEffect(() => {
    if (deleteScopeOpen && pendingDeleteScope === null && !seriesEndAvailable) {
      setDeleteScopeOpen(false);
      setPendingConfirm("delete");
    }
  }, [deleteScopeOpen, pendingDeleteScope, seriesEndAvailable]);

  const openDeleteFlow = () => {
    // The hook refreshes once a minute. Re-read Berlin's current date at the
    // interaction boundary so the short interval after midnight cannot open
    // an already invalid series-ending flow.
    if (canEndSeries(berlinTodayISO())) {
      setDeleteScopeOpen(true);
      return;
    }
    setPendingConfirm("delete");
  };

  const handleConfirm = async () => {
    const action = pendingConfirm;
    setPendingConfirm(null);
    if (action === "delete") {
      await handleDeleteCancelled();
    } else if (action) {
      await handleLifecycle(action);
    }
  };

  const handleDeleteScopeSelect = async (scope: string) => {
    // The scope modal may have been open across midnight. Do not send a
    // stale "following" request that the backend must reject.
    if (scope === "following" && !canEndSeries(berlinTodayISO())) {
      setDeleteScopeOpen(false);
      setPendingConfirm("delete");
      return;
    }
    setPendingDeleteScope(scope);
    try {
      const succeeded =
        scope === "following"
          ? await handleDeleteFollowing()
          : await handleDeleteCancelled();
      if (succeeded) {
        setDeleteScopeOpen(false);
      }
    } finally {
      setPendingDeleteScope(null);
    }
  };

  const handleAttendancePatch = async (
    studentId: string,
    body: AttendancePatchBody,
  ) => {
    if (!instance || !onAttendancePatch) return;
    setPendingStudentId(studentId);
    try {
      await awaitReportedAction(() =>
        onAttendancePatch(instance.id, studentId, body),
      );
    } finally {
      setPendingStudentId(null);
    }
  };

  const attendancePatch = attendancePatchForInstance(
    attendanceWebEnabled,
    instance,
    onAttendancePatch,
  );

  if (!instance) return null;

  const footer = (
    <div className="flex w-full flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        {/* Sprung in den Vertretungs-Bereich bei einer Störung des Blocks
                  (offene Lücke oder eingetragene Abwesenheit) —
                  docs/planung-redesign/docs/07-vertretung.md Abschnitt 6. Nutzt
                  nur bereits geladene Instanzdaten, kein zusätzlicher Abruf. */}
        {canManage &&
          (instance.status === "planned" || instance.status === "active") &&
          (instance.staff.some((row) => row.isAbsent) ||
            (instance.requiredStaffCount > 0 &&
              instance.assignedStaffCount < instance.requiredStaffCount)) && (
            <Link
              href={tenantPath(
                `/vertretung?d=${instance.date}&block=${instance.id}`,
              )}
              className="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <MotoConceptIcon concept="substitution" size={18} />
              Vertretung bearbeiten
            </Link>
          )}
        {instance.status === "planned" && !editDeferred && onEdit && (
          <Button
            variant="outline"
            size="md"
            type="button"
            onClick={() => onEdit(instance)}
            disabled={pendingAction !== null}
          >
            <span className="inline-flex items-center gap-2">
              <Pencil className="h-4 w-4" />
              Bearbeiten
            </span>
          </Button>
        )}
        {instance.status === "planned" &&
          !instance.activityGroupId &&
          onRepeat && (
            <Button
              variant="outline"
              size="md"
              type="button"
              onClick={() => onRepeat(instance)}
              disabled={pendingAction !== null}
            >
              <span className="inline-flex items-center gap-2">
                <Repeat className="h-4 w-4" />
                Wiederholen
              </span>
            </Button>
          )}
        {canManage && instance.status === "active" && attendanceWebEnabled && (
          <Button
            variant="primary"
            size="md"
            type="button"
            onClick={() => setPendingConfirm("complete")}
            isLoading={pendingAction === "complete"}
            loadingText="Beende …"
            disabled={pendingAction !== null || !completeEnabled}
          >
            <span className="inline-flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4" />
              {completeEnabled ? "Beenden" : `Beenden ab ${instance.endTime}`}
            </span>
          </Button>
        )}
        {canManage &&
          (instance.status === "planned" ||
            (instance.status === "active" && attendanceWebEnabled)) && (
            <Button
              variant="outline_danger"
              size="md"
              type="button"
              onClick={() => setPendingConfirm("cancel")}
              isLoading={pendingAction === "cancel"}
              loadingText="Sage ab …"
              disabled={pendingAction !== null}
            >
              <span className="inline-flex items-center gap-2">
                <CircleX className="h-4 w-4" />
                Absagen
              </span>
            </Button>
          )}
        {instance.status === "planned" && onDeleteCancelled && (
          <Button
            variant="outline_danger"
            size="md"
            type="button"
            onClick={openDeleteFlow}
            isLoading={pendingDelete}
            loadingText="Lösche …"
            disabled={pendingAction !== null || pendingDelete}
          >
            <span className="inline-flex items-center gap-2">
              <Trash2 className="h-4 w-4" />
              Löschen
            </span>
          </Button>
        )}
        {instance.status === "completed" && (
          <>
            <span className="inline-flex items-center gap-2 text-xs text-gray-500">
              <CheckCircle2 className="h-4 w-4" />
              Diese Aktivität ist bereits abgeschlossen.
            </span>
            {canManage && instance.canReopen && (
              <Button
                variant="outline"
                size="md"
                type="button"
                onClick={() => setPendingConfirm("reopen")}
                isLoading={pendingAction === "reopen"}
                loadingText="Öffne wieder …"
                disabled={pendingAction !== null}
              >
                Wieder öffnen
              </Button>
            )}
          </>
        )}
        {instance.status === "cancelled" && (
          <>
            <span className="inline-flex items-center gap-2 text-xs text-gray-500">
              <CircleX className="h-4 w-4" />
              Diese Aktivität wurde abgesagt.
            </span>
            {onDeleteCancelled && (
              <Button
                variant="outline_danger"
                size="md"
                type="button"
                onClick={openDeleteFlow}
                isLoading={pendingDelete}
                loadingText="Lösche …"
                disabled={pendingAction !== null || pendingDelete}
              >
                <span className="inline-flex items-center gap-2">
                  <Trash2 className="h-4 w-4" />
                  Löschen
                </span>
              </Button>
            )}
          </>
        )}
      </div>
      {editDeferred && (
        <div className="flex items-center justify-end gap-2 text-xs text-gray-400">
          <Pencil className="h-3.5 w-3.5" />
          <span>Bearbeiten kommt im nächsten Update</span>
        </div>
      )}
    </div>
  );

  return (
    <>
      {/* Confirmation-/ChoiceModal teilen sich mit dem Detail-Modal denselben
          fixen z-index. Solange eines offen ist, wird das Detail-Modal
          ausgeblendet statt gestapelt (gleiches Muster wie
          staff/shift-move-dialog.tsx). */}
      <Modal
        isOpen={pendingConfirm === null && !deleteScopeOpen && !suspended}
        onClose={onClose}
        title={instance.title}
        closeLabel="Schließen"
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-3xl"
        footer={footer}
      >
        <div className="space-y-5">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge status={instance.status} />
              {instance.isSpontaneous && (
                <span
                  className="rounded-full bg-gray-100 px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-gray-600 uppercase"
                  title="Dieser Termin wurde spontan gestartet und war nicht geplant."
                >
                  Spontan gestartet
                </span>
              )}
              <ActivityTypeBadge activityType={instance.activityType} />
            </div>
            <p className="text-sm text-gray-500">
              {germanFullDate(instance.date)} • {instance.startTime} –{" "}
              {instance.endTime}
            </p>
            {instance.activityGroupId && (
              <OriginChip
                label={regelterminOriginLabel(instance)}
                className="mt-1.5"
              />
            )}
          </div>
          {instance.conflictWarnings.length > 0 && (
            <div className={timetableDangerPanel}>
              <div className="text-moto-red-strong flex items-center gap-2 text-xs font-bold">
                <TriangleAlert className="h-4 w-4" />
                {instance.conflictWarnings.length} Konflikt(e)
              </div>
              <ul className="text-moto-red-strong mt-1 space-y-0.5 text-xs">
                {instance.conflictWarnings.map((warning) => (
                  <li key={warning.message}>• {warning.message}</li>
                ))}
              </ul>
            </div>
          )}

          <StatsRow instance={instance} />

          <Section title="Details">
            <Row
              icon={<MotoConceptIcon concept="careTimes" size={18} />}
              label="Zeit"
            >
              {instance.startTime} – {instance.endTime}
            </Row>
            <Row
              icon={<MotoConceptIcon concept="rooms" size={18} />}
              label="Raum"
            >
              {instance.roomName || `Raum #${instance.roomId}`}
            </Row>
            <Row icon={<Palette className="h-4 w-4" />} label="Planungsspur">
              {instance.planningTrackName ?? "Keine Planungsspur"}
            </Row>
            <Row
              icon={<MotoConceptIcon concept="staff" size={18} />}
              label={`Personal (${instance.staffCount})`}
            >
              {instance.staffCount === 0
                ? "Niemand zugeordnet"
                : `${instance.staffCount - instance.absentStaffCount} aktiv${
                    instance.absentStaffCount > 0
                      ? `, ${instance.absentStaffCount} abwesend`
                      : ""
                  }`}
            </Row>
            {showTimetableCounts ? (
              <Row
                icon={<MotoConceptIcon concept="children" size={18} />}
                label="Kinder"
              >
                {instance.expectedStudentsCount + instance.presentStudentsCount}{" "}
                eingetragen
                {instance.presentStudentsCount > 0
                  ? ` • ${instance.presentStudentsCount} anwesend`
                  : ""}
                {/* Names the gap between the assignment list and the care
                      plan (#1747) instead of leaving a smaller number
                      unexplained. */}
                {instance.notScheduledStudentsCount > 0
                  ? ` • ${instance.notScheduledStudentsCount} heute nicht eingeplant`
                  : ""}
              </Row>
            ) : null}
            {instance.seriesNotes && (
              <Row icon={<Repeat className="h-4 w-4" />} label="Wochennotiz">
                <span className="whitespace-pre-line">
                  {instance.seriesNotes}
                </span>
              </Row>
            )}
            {instance.notes && (
              <Row
                icon={<StickyNote className="h-4 w-4" />}
                label={instance.seriesNotes ? "Tagesnotiz" : "Notiz"}
              >
                <span className="whitespace-pre-line">{instance.notes}</span>
              </Row>
            )}
          </Section>

          {fetchParticipantNames ? (
            <ParticipantNamesLoader instanceId={instance.id}>
              {(names) => (
                <>
                  <AssignedStaffSection
                    instance={instance}
                    staffNames={names.staffNames}
                    onOpenPool={poolAvailable ? onOpenPool : undefined}
                    canManageStaffPool={canManageStaffPool}
                  />
                  <InstanceStudentsSection
                    groupedStudents={groupedStudents}
                    handleAttendancePatch={handleAttendancePatch}
                    instance={instance}
                    onAttendancePatch={attendancePatch}
                    pendingStudentId={pendingStudentId}
                    studentNames={names.studentNames}
                    students={students}
                  />
                </>
              )}
            </ParticipantNamesLoader>
          ) : (
            <>
              <AssignedStaffSection
                instance={instance}
                staffNames={staffNames}
                onOpenPool={poolAvailable ? onOpenPool : undefined}
                canManageStaffPool={canManageStaffPool}
              />
              <InstanceStudentsSection
                groupedStudents={groupedStudents}
                handleAttendancePatch={handleAttendancePatch}
                instance={instance}
                onAttendancePatch={attendancePatch}
                pendingStudentId={pendingStudentId}
                studentNames={studentNames}
                students={students}
              />
            </>
          )}
        </div>
      </Modal>
      {pendingConfirm && (
        <ConfirmationModal
          isOpen
          onClose={() => setPendingConfirm(null)}
          onConfirm={handleConfirm}
          title={
            pendingConfirm === "delete" && instance?.status === "planned"
              ? "Termin löschen?"
              : CONFIRM_DIALOGS[pendingConfirm].title
          }
          confirmText={CONFIRM_DIALOGS[pendingConfirm].confirmText}
          cancelText="Abbrechen"
          confirmButtonClass={
            CONFIRM_DIALOGS[pendingConfirm].confirmButtonClass
          }
        >
          <p className="text-sm leading-relaxed text-gray-600">
            {pendingConfirm === "cancel" && instance?.status === "active"
              ? "Die laufende Betreuung wird gestoppt und der Termin als abgesagt markiert. Das kann nicht rückgängig gemacht werden."
              : pendingConfirm === "delete" && instance?.status === "planned"
                ? "Der geplante Termin wird dauerhaft entfernt."
                : CONFIRM_DIALOGS[pendingConfirm].body}
          </p>
        </ConfirmationModal>
      )}
      <ChoiceModal
        isOpen={deleteScopeOpen}
        onClose={() => setDeleteScopeOpen(false)}
        title="Wiederholenden Termin löschen"
        description={`Der Termin am ${germanFullDate(instance.date)} gehört zu einem Regeltermin.`}
        options={[
          {
            value: "single",
            label: "Nur diese Woche",
            description:
              "Löscht nur diesen einen Termin und verhindert, dass er erneut eingetragen wird; der Regeltermin bleibt bestehen.",
          },
          {
            value: "following",
            label: "Ab jetzt dauerhaft",
            description:
              "Beendet den Regeltermin ab diesem Datum; frühere Termine bleiben erhalten.",
          },
        ]}
        onSelect={handleDeleteScopeSelect}
        isBusy={pendingDeleteScope !== null}
      />
    </>
  );
}

function StudentGroup({
  status,
  careDayGroup = false,
  students,
  studentNames,
  pendingStudentId,
  onAttendancePatch,
  instanceStatus,
  handleAttendancePatch,
}: {
  status: InstanceStudentSummary["status"];
  /**
   * True for the "not in care today" group: it is labelled by the care-day
   * verdict rather than the attendance status, and keeps only the "anwesend"
   * action for a child who turns up anyway (#1747).
   */
  careDayGroup?: boolean;
  students: InstanceStudentSummary[];
  studentNames: Map<string, string>;
  pendingStudentId: string | null;
  onAttendancePatch?: InstanceDetailModalProps["onAttendancePatch"];
  instanceStatus: InstanceStatus;
  handleAttendancePatch: (
    studentId: string,
    body: AttendancePatchBody,
  ) => Promise<void>;
}) {
  const showTimetableCounts = useShowTimetableCounts();
  if (students.length === 0) return null;
  const isPlanned = instanceStatus === "planned";
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-[11px] font-bold tracking-wide text-gray-400 uppercase">
        <span>
          {careDayGroup
            ? "Heute nicht eingeplant"
            : attendanceLabel(status, isPlanned)}
        </span>
        {showTimetableCounts ? <span>{students.length}</span> : null}
      </div>
      {students.map((student) => {
        const studentName =
          studentNames.get(student.studentId) ?? `Kind #${student.studentId}`;

        return (
          <div
            key={student.studentId}
            className={`flex flex-wrap items-center justify-between gap-2 ${NESTED_SURFACE_BASE} px-3 py-2 ${
              // A non-booking is not an attendance outcome: a row that landed
              // here carrying a status-day 'absent' must not be tinted like a
              // real absence (#1747).
              careDayGroup ? NOT_SCHEDULED_TONE : attendanceTone(student.status)
            }`}
          >
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold text-gray-900">
                {studentName}
              </div>
              <div className="text-[11px] text-gray-500">
                {careDayGroup
                  ? careDayLabel(student.careDayStatus)
                  : attendanceLabel(student.status, isPlanned)}
                {student.substatus
                  ? ` • ${attendanceSubstatusLabel(student.substatus)}`
                  : ""}
                {student.note ? ` • ${student.note}` : ""}
              </div>
            </div>
            {onAttendancePatch && (
              <div className="flex shrink-0 items-center gap-1">
                {/*
                  A child who is not in care today but turns up anyway is still
                  checked in from here (#1747 review). The care-day group keeps
                  only this one action: "anwesend" records what actually
                  happened and takes the row out of the group, while "abmelden"
                  and "zurücksetzen" would write attendance for a day the child
                  was never expected on.
                */}
                {!isPlanned && student.status !== "present" && (
                  <IconActionButton
                    icon={<Check className="h-3.5 w-3.5" />}
                    label={`${studentName} als anwesend markieren`}
                    tone="green"
                    isLoading={pendingStudentId === student.studentId}
                    disabled={pendingStudentId !== null}
                    onClick={() =>
                      void handleAttendancePatch(student.studentId, {
                        status: "present",
                        substatus: null,
                      })
                    }
                  />
                )}
                {!careDayGroup && student.status !== "absent" && (
                  <IconActionButton
                    icon={<X className="h-3.5 w-3.5" />}
                    label={
                      isPlanned
                        ? `${studentName} abmelden`
                        : `${studentName} als fehlend markieren`
                    }
                    tone="red"
                    isLoading={pendingStudentId === student.studentId}
                    disabled={pendingStudentId !== null}
                    onClick={() =>
                      void handleAttendancePatch(student.studentId, {
                        status: "absent",
                        substatus: isPlanned ? "excused" : null,
                      })
                    }
                  />
                )}
                {!careDayGroup && student.status !== "expected" && (
                  <IconActionButton
                    icon={<RotateCcw className="h-3.5 w-3.5" />}
                    label={`Status von ${studentName} zurücksetzen`}
                    tone="slate"
                    isLoading={pendingStudentId === student.studentId}
                    disabled={pendingStudentId !== null}
                    onClick={() =>
                      void handleAttendancePatch(student.studentId, {
                        status: "expected",
                        substatus: null,
                        note: null,
                      })
                    }
                  />
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function EmptyLine({ children }: { children: React.ReactNode }) {
  return (
    <div
      className={`${timetableMutedSurface} border-dashed px-3 py-2 text-xs text-gray-500`}
    >
      {children}
    </div>
  );
}

/**
 * Structural radius/shadow shared by PersonLine and the StudentGroup row —
 * same shape as `timetableNestedSurface`, but callers layer their own
 * tone-conditional border/bg color on top (nested surface tokens are
 * gray/white only).
 */
const NESTED_SURFACE_BASE = "rounded-xl border shadow-sm";

type IconActionTone = "green" | "red" | "slate";

interface IconActionButtonProps {
  icon: React.ReactNode;
  label: string;
  tone: IconActionTone;
  onClick: () => void;
  isLoading?: boolean;
  disabled?: boolean;
}

const ICON_ACTION_PALETTE: Record<IconActionTone, string> = {
  green:
    "border-moto-green/20 bg-white text-moto-green-strong hover:border-moto-green/40 hover:bg-moto-green/10",
  red: "border-moto-red/20 bg-white text-moto-red-strong hover:border-moto-red/40 hover:bg-moto-red/10",
  slate:
    "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50",
};

function IconActionButton({
  icon,
  label,
  tone,
  onClick,
  isLoading,
  disabled,
}: IconActionButtonProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      onClick={onClick}
      disabled={disabled || isLoading}
      title={label}
      aria-label={label}
      className={`rounded-full border ${ICON_ACTION_PALETTE[tone]}`}
    >
      {isLoading ? (
        <span
          className="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent"
          aria-hidden
        />
      ) : (
        <span aria-hidden>{icon}</span>
      )}
    </Button>
  );
}

function PersonLine({
  name,
  meta,
  danger,
}: {
  name: string;
  meta: Array<string | null>;
  danger?: boolean;
}) {
  const labels = meta.filter(Boolean);
  return (
    <div
      className={`${NESTED_SURFACE_BASE} px-3 py-2 ${
        danger
          ? "border-moto-red/20 bg-moto-red/10"
          : "border-gray-200 bg-white"
      }`}
    >
      <div className="text-sm font-semibold text-gray-900">{name}</div>
      {labels.length > 0 && (
        <div
          className={`mt-0.5 text-[11px] ${
            danger ? "text-moto-red-strong" : "text-gray-500"
          }`}
        >
          {labels.join(" • ")}
        </div>
      )}
    </div>
  );
}

interface StatsRowProps {
  instance: EnrichedInstance;
}

function StatsRow({ instance }: StatsRowProps) {
  const showTimetableCounts = useShowTimetableCounts();
  const expected = instance.expectedStudentsCount;
  const present = instance.presentStudentsCount;
  const totalStudents = expected + present;
  const activeStaff = instance.staffCount - instance.absentStaffCount;

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {showTimetableCounts && (
        <TimetableRatioPill
          icon={<MotoConceptIcon concept="present" size={16} />}
          label="Anwesend"
          value={totalStudents === 0 ? "—" : `${present} / ${totalStudents}`}
          tone={attendanceStudentTone(present, totalStudents)}
        />
      )}
      <TimetableRatioPill
        icon={<MotoConceptIcon concept="staff" size={16} />}
        label="Personal"
        value={
          instance.staffCount === 0
            ? "—"
            : `${activeStaff} / ${instance.staffCount}`
        }
        tone={attendanceStaffTone(
          instance.staffCount,
          instance.absentStaffCount,
        )}
      />
      <TimetableRatioPill
        icon={<MotoConceptIcon concept="supervision" size={16} />}
        label="Besetzung"
        value={
          instance.requiredStaffCount === 0
            ? "—"
            : `${instance.assignedStaffCount} / ${instance.requiredStaffCount}`
        }
        tone={capacityTone(
          instance.assignedStaffCount,
          instance.requiredStaffCount,
        )}
      />
    </div>
  );
}

interface SectionProps {
  title: string;
  children: React.ReactNode;
}

function Section({ title, children }: SectionProps) {
  return (
    <div className="space-y-2">
      <h4 className="text-[10px] font-bold tracking-wider text-gray-400 uppercase">
        {title}
      </h4>
      <div className="space-y-1.5">{children}</div>
    </div>
  );
}

interface RowProps {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}

function Row({ icon, label, children }: RowProps) {
  return (
    <div
      className={`${timetableNestedSurface} flex items-start gap-3 p-3 text-sm`}
    >
      <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center text-gray-400">
        {icon}
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-[11px] font-medium text-gray-500">{label}</span>
        <span className="text-sm text-gray-900">{children}</span>
      </div>
    </div>
  );
}
