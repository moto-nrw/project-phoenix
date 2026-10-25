"use client";

import { teacherHintKey, teacherHintText } from "~/lib/student-helpers";
import {
  Fragment,
  useEffect,
  useId,
  useState,
  type ComponentType,
  type ReactNode,
} from "react";
import { UserPlus, Users } from "lucide-react";
import { ArrowUpRightIcon } from "@phosphor-icons/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { EditActions } from "~/components/ui/edit-actions";
import type { FormErrorInput } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  HOMEWORK_FEEDBACK_COLORS,
  HOMEWORK_FEEDBACK_LABELS,
  HOMEWORK_FEEDBACK_STATUSES,
  HOMEWORK_NOTE_MAX_LENGTH,
  homeworkLine,
} from "~/lib/homework-feedback";
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import { OccupancyBadges } from "~/components/ui/occupancy-badges";
import type { AddUnplannedStudentModalProps } from "~/components/active-supervisions/add-unplanned-student-modal";
import type { PresentChildrenPickerProps } from "~/components/active-supervisions/present-children-picker";
import {
  SectionCard,
  SectionCollapseToggle,
} from "~/components/ui/section-card";
import { Tooltip } from "~/components/ui/tooltip";
import { GroupIcon, SchoolClassIcon } from "~/components/students/student-card";
import { overbookedHintFor, type Occupancy } from "~/lib/activity-occupancy";
import { useNFCEnabled } from "~/lib/tenant-context";
import {
  clearOwnAttendanceMutation,
  markOwnAttendanceMutation,
} from "~/lib/sse-optimistic-mutations";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  rosterPickupTimeLabel,
  upcomingArrivalTime,
} from "~/lib/timetable-roster-helpers";
import { saveStudentPartialAbsence } from "~/lib/student-partial-absences-api";
import {
  canCompleteInstance,
  completeAvailableClock,
} from "~/lib/timetable-lifecycle";
import { hasOpenEnd } from "~/lib/block-children";
import { TIMETABLE_VIEW_ONLY_NOTICE } from "~/lib/timetable-operation-access";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type {
  HomeworkFeedbackBody,
  TimetableHomeworkFeedbackStatus,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { isCareDayExpected, isNotScheduledRow } from "~/lib/timetable-types";
import { rosterRunState } from "~/components/active-supervisions/view-model";

/**
 * Marks the attendance mutation as our own before firing the request so the
 * global SSE handler can skip the echo event, and clears the mark again when
 * the request fails (the echo will never arrive).
 */
export async function runOwnAttendanceMutation<T>(
  eventType: "student_checkin" | "student_checkout",
  studentId: string,
  request: () => Promise<T>,
): Promise<T> {
  markOwnAttendanceMutation(eventType, studentId);
  try {
    return await request();
  } catch (error) {
    clearOwnAttendanceMutation(eventType, studentId);
    throw error;
  }
}

const ATTENDANCE_SUBSTATUS_LABELS: Record<
  NonNullable<TimetableRosterRow["substatus"]>,
  string
> = {
  late: "Verspätet",
  excused: "Entschuldigt",
  sick: "Krank",
  field_trip: "Ausflug",
  other: "Sonstiges",
};

// Builds the info notice after a check-in auto-moved the child out of another
// running session (#2386). Null when the response reports no move.
export function moveNoticeFromRoster(
  roster: TimetableRoster,
  studentId: string,
): string | null {
  if (roster.movedFrom == null) return null;
  const name =
    roster.rows.find((row) => row.studentId === studentId)?.studentName ??
    "Das Kind";
  return roster.movedFrom
    ? `${name} wurde aus „${roster.movedFrom}“ hierher geholt.`
    : `${name} wurde aus einer anderen Aktivität hierher geholt.`;
}

// Class and group follow the name on its line, with the icons of the room
// cards (#4356); the hidden words keep them apart for screen readers. A
// group named like the class is not repeated.
function RosterStudentMeta({
  row,
  instanceIsSpontaneous,
}: Readonly<{ row: TimetableRosterRow; instanceIsSpontaneous: boolean }>) {
  const showGroup = row.groupName && row.groupName !== row.schoolClass;
  return (
    <>
      {row.schoolClass ? (
        <span className={ROSTER_META_CLASS}>
          <SchoolClassIcon />
          <span className="sr-only">Klasse </span>
          {row.schoolClass}
        </span>
      ) : null}
      {showGroup ? (
        <span className={ROSTER_META_CLASS}>
          <GroupIcon />
          <span className="sr-only">Gruppe </span>
          {row.groupName}
        </span>
      ) : null}
      {row.isUnplanned && !instanceIsSpontaneous ? (
        <span className="text-xs font-medium text-gray-500">ungeplant</span>
      ) : null}
    </>
  );
}

// Die Antworten der Hausaufgaben-Rückmeldung (#4796) im Auswahlfeld.
const HOMEWORK_FEEDBACK_TONES = {
  done: "green",
  not_done: "red",
  difficult: "neutral",
} as const satisfies Record<TimetableHomeworkFeedbackStatus, string>;

const ROSTER_META_CLASS =
  "inline-flex items-center gap-1 text-xs font-medium text-gray-500";

function RosterSummaryStat({
  label,
  value,
}: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-xl bg-white/80 px-3 py-2 shadow-[0_1px_0_rgba(17,24,39,0.04)]">
      <span className="block text-sm font-semibold text-gray-900">{value}</span>
      <span className="block text-xs font-medium text-gray-500">{label}</span>
    </div>
  );
}

export type RosterAction =
  "check-in" | "check-out" | "excused" | "absent" | "expected";

export async function runRosterActionRequest(
  action: RosterAction,
  instanceId: string,
  studentId: string,
): Promise<TimetableRoster | null> {
  if (action === "check-in") {
    return runOwnAttendanceMutation("student_checkin", studentId, () =>
      timetableOperationsApi.checkIn(instanceId, studentId),
    );
  }
  if (action === "check-out") {
    return runOwnAttendanceMutation("student_checkout", studentId, () =>
      timetableOperationsApi.checkOut(instanceId, studentId),
    );
  }
  if (action === "expected") {
    await timetableOperationsApi.patchAttendance(instanceId, studentId, {
      status: "expected",
      substatus: null,
      note: null,
    });
    return null;
  }
  await timetableOperationsApi.patchAttendance(
    instanceId,
    studentId,
    action === "excused"
      ? { status: "absent", substatus: "excused" }
      : { status: "absent" },
  );
  return null;
}

/** The block was excused, but the partial absence for the rest of the day
 *  could not be written (conflict with another excusal, network error). */
export class RestOfDayNotSavedError extends Error {
  constructor(cause: unknown) {
    // The cause keeps code and request ID for the shared error display.
    super(cause instanceof Error ? cause.message : String(cause), { cause });
    this.name = "RestOfDayNotSavedError";
  }
}

/**
 * „Rest des Tages“ (#3166): excuses this block by hand, then records a partial
 * absence from the block's start through the existing partial-absence path.
 * That path excuses every later block of the day that nobody decided by hand
 * yet; earlier blocks start before the cutoff and stay untouched.
 */
export async function runRestOfDayExcusalRequest(
  instance: Pick<TimetableRoster["instance"], "id" | "date" | "startTime">,
  studentId: string,
): Promise<void> {
  await runRosterActionRequest("excused", instance.id, studentId);
  try {
    await saveStudentPartialAbsence(
      studentId,
      null,
      instance.date,
      instance.startTime,
    );
  } catch (error) {
    throw new RestOfDayNotSavedError(error);
  }
}

const EXCUSE_SCOPE_BLOCK = "block";
const EXCUSE_SCOPE_REST_OF_DAY = "rest-of-day";

interface ExcuseScopeModalProps {
  readonly instance: TimetableRoster["instance"];
  readonly row: TimetableRosterRow | null;
  readonly isBusy: boolean;
  readonly onClose: () => void;
  readonly onSelect: (scope: string) => void;
}

function ExcuseScopeModal({
  instance,
  row,
  isBusy,
  onClose,
  onSelect,
}: ExcuseScopeModalProps) {
  const name = row?.studentName || `Kind ${row?.studentId ?? ""}`;
  return (
    <ChoiceModal
      isOpen={row !== null}
      onClose={onClose}
      title={`${name} entschuldigen`}
      isBusy={isBusy}
      onSelect={onSelect}
      options={[
        {
          value: EXCUSE_SCOPE_BLOCK,
          label: "Nur dieser Block",
          description: `${instance.title}, ${instance.startTime}–${instance.endTime} Uhr`,
        },
        {
          value: EXCUSE_SCOPE_REST_OF_DAY,
          label: "Rest des Tages",
          description: `Ab ${instance.startTime} Uhr alle Blöcke heute.`,
        },
      ]}
    />
  );
}

interface RosterActionAccess {
  readonly attendance: boolean;
  readonly statuses: boolean;
  readonly absence: boolean;
}

interface RosterRowActionsProps {
  readonly access: RosterActionAccess;
  readonly row: TimetableRosterRow;
  readonly onAction: (
    action: RosterAction,
    row: TimetableRosterRow,
  ) => Promise<void>;
  /** Adresse der Kinderseite; ohne sie gibt es den Knopf „Profil ansehen“ nicht. */
  readonly profileHref?: string;
  /**
   * Öffnet die Rückmeldung zu den Hausaufgaben (#4796). Nur in einer
   * Lernzeit mit dem Recht dazu, und nur bei Kindern mit Hausaufgaben.
   */
  readonly onHomework?: (row: TimetableRosterRow) => void;
}

function RosterRowActions({
  row,
  onAction,
  access,
  profileHref,
  onHomework,
}: RosterRowActionsProps) {
  const homeworkAction = onHomework && (row.homework?.length ?? 0) > 0;
  if (
    !access.attendance &&
    !access.statuses &&
    !access.absence &&
    !profileHref &&
    !homeworkAction
  ) {
    return null;
  }
  const runAction = async (action: RosterAction) => {
    await onAction(action, row);
  };

  return (
    <div className="flex flex-wrap gap-2">
      {access.attendance &&
      !row.currentlyPresent &&
      row.status === "expected" ? (
        <Button
          type="button"
          onClick={() => runAction("check-in")}
          variant="success"
          size="md"
        >
          Einchecken
        </Button>
      ) : null}
      {access.attendance &&
      !row.currentlyPresent &&
      row.status !== "expected" ? (
        <Button
          type="button"
          onClick={() => runAction("check-in")}
          variant="success"
          size="md"
        >
          Wieder einchecken
        </Button>
      ) : null}
      {access.attendance && row.currentlyPresent ? (
        <Button
          type="button"
          onClick={() => runAction("check-out")}
          variant="outline"
          size="md"
        >
          Raum verlassen
        </Button>
      ) : null}
      {row.planned &&
      row.status === "expected" &&
      isCareDayExpected(row.careDayStatus) ? (
        <>
          {access.absence ? (
            <Button
              type="button"
              onClick={() => runAction("excused")}
              variant="outline"
              size="md"
              className="text-moto-purple-strong !ring-moto-purple"
            >
              Entschuldigt
            </Button>
          ) : null}
          {access.statuses ? (
            <Button
              type="button"
              onClick={() => runAction("absent")}
              variant="outline"
              size="md"
            >
              Abwesend
            </Button>
          ) : null}
        </>
      ) : null}
      {(row.substatus === "sick" || row.substatus === "excused"
        ? access.absence
        : access.statuses) &&
      row.planned &&
      !row.currentlyPresent &&
      row.status === "absent" ? (
        <Button
          type="button"
          onClick={() => runAction("expected")}
          variant="outline"
          size="md"
        >
          Zurück auf erwartet
        </Button>
      ) : null}
      {homeworkAction ? (
        <Button
          type="button"
          onClick={() => onHomework?.(row)}
          variant="outline"
          size="md"
        >
          Hausaufgaben
        </Button>
      ) : null}
      {/* Ein Link, kein Anwesenheitsknopf: er öffnet nur die Kinderseite
          (#4356). Nur ein Icon am Ende der Zeile, damit die Zeile nicht
          wächst; auf dem Handy rechts außen. Der Name im Label unterscheidet
          die Links der Liste. */}
      {profileHref ? (
        <Tooltip
          content="Profil ansehen"
          asChild
          iconOnly
          className="ml-auto sm:ml-0"
          bubbleClassName="right-0 left-auto"
        >
          <ButtonLink
            href={profileHref}
            variant="outline"
            size="icon"
            aria-label={`Profil ansehen: ${row.studentName || `Kind ${row.studentId}`}`}
            className="size-9 shrink-0"
          >
            <ArrowUpRightIcon
              className="size-4"
              weight="bold"
              aria-hidden="true"
            />
          </ButtonLink>
        </Tooltip>
      ) : null}
    </div>
  );
}

interface TimetableRosterRowProps {
  readonly actionAccess: RosterActionAccess;
  readonly instanceIsSpontaneous: boolean;
  /** Minute clock of the page — decides whether an expected arrival is still ahead. */
  readonly now: Date;
  readonly rosterDate: string;
  readonly pickupTimesLoaded?: boolean;
  readonly pickupTimesRedacted?: boolean;
  readonly row: TimetableRosterRow;
  readonly onAction: RosterRowActionsProps["onAction"];
  /**
   * Öffnet die Abhol- und Notfallinformationen des Kindes (#2527). Nur das
   * Schul-Portal reicht das durch; ohne Callback bleibt der Name Text und
   * sieht auch nicht antippbar aus.
   */
  readonly onOpenStudent?: (row: TimetableRosterRow) => void;
  readonly studentProfileHref?: (row: TimetableRosterRow) => string;
  readonly onHomework?: (row: TimetableRosterRow) => void;
}

function TimetableRosterStudentRow({
  actionAccess,
  instanceIsSpontaneous,
  now,
  rosterDate,
  pickupTimesLoaded,
  pickupTimesRedacted,
  row,
  onAction,
  onOpenStudent,
  studentProfileHref,
  onHomework,
}: TimetableRosterRowProps) {
  const pickupTimeLabel = rosterPickupTimeLabel(
    row.pickupTime,
    pickupTimesLoaded,
    pickupTimesRedacted,
  );
  const attendanceDetail = [
    row.substatus ? ATTENDANCE_SUBSTATUS_LABELS[row.substatus] : null,
    row.note,
  ]
    .filter(Boolean)
    .join(" · ");
  // A still-upcoming arrival gets the concrete time instead of the backend's
  // warning sentence; once the time has passed the child is simply expected
  // and the stale sentence would only add noise (#2878). A child who already
  // arrived early, is absent, or has already departed carries no arrival line
  // — "Kommt um 13:45 Uhr" would contradict the list it stands in. Every
  // other planning warning keeps its message — the preview showed it, so the
  // started view must not lose it. An arrival warning without a time cannot be
  // replaced by a time, so its message stays too.
  const expectsArrival =
    row.planned &&
    !row.currentlyPresent &&
    row.status === "expected" &&
    isCareDayExpected(row.careDayStatus);
  const arrivalTime = expectsArrival
    ? upcomingArrivalTime(row.warnings, now, rosterDate)
    : null;
  const planningNotes = (row.warnings ?? []).filter(
    (warning) =>
      (warning.kind !== "arrival_after_slot_start" ||
        !warning.expectedArrival) &&
      warning.kind !== "class_arrival_exception",
  );
  // A class-wide day exception (#2962) is plain information, not a warning:
  // the whole class arrives at another time today and the line says why.
  const classException = (row.warnings ?? []).find(
    (warning) => warning.kind === "class_arrival_exception",
  );

  return (
    <div className="flex flex-col gap-2 border-b border-gray-100 px-4 py-2.5 last:border-b-0 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5">
          {onOpenStudent ? (
            <button
              type="button"
              onClick={() => onOpenStudent(row)}
              className="text-left font-medium text-gray-900 underline decoration-gray-300 underline-offset-4 hover:decoration-gray-600"
            >
              {row.studentName || `Kind ${row.studentId}`}
            </button>
          ) : (
            <div className="font-medium text-gray-900">
              {row.studentName || `Kind ${row.studentId}`}
            </div>
          )}
          <RosterStudentMeta
            row={row}
            instanceIsSpontaneous={instanceIsSpontaneous}
          />
        </div>
        {pickupTimeLabel === null ? null : (
          <div className="mt-0.5 text-sm font-medium text-gray-700">
            Gehzeit: {pickupTimeLabel}
          </div>
        )}
        {arrivalTime ? (
          <div className="mt-0.5 text-sm font-medium text-gray-700">
            Kommt um {arrivalTime} Uhr
          </div>
        ) : null}
        {classException && expectsArrival ? (
          <div className="mt-0.5 text-sm text-gray-700">
            {classException.message}
          </div>
        ) : null}
        {planningNotes.map((warning) => (
          <div
            key={`${warning.kind}:${warning.message}`}
            className="text-moto-amber-strong mt-0.5 text-sm"
          >
            {warning.message}
          </div>
        ))}
        {attendanceDetail ? (
          <div className="text-moto-amber-strong mt-0.5 text-sm">
            {attendanceDetail}
          </div>
        ) : null}
        {/* Hinweise der Lehrkraft (#4795): dieselbe Zeile wie auf den Karten. */}
        {(row.teacherHints ?? []).map((hint) => (
          <div
            key={teacherHintKey(hint)}
            className="mt-0.5 text-sm break-words text-gray-700"
          >
            {teacherHintText(hint)}
          </div>
        ))}
        {row.parallelPresentIn ? (
          <div className="text-moto-amber-strong mt-0.5 text-sm">
            {`Auch in „${row.parallelPresentIn.title}“ (${row.parallelPresentIn.startTime}–${row.parallelPresentIn.endTime}) als anwesend eingetragen`}
          </div>
        ) : null}
        {/* Hausaufgaben der Klasse in der Lernzeit (#4796): was zu tun ist
            und, falls schon gegeben, die Rückmeldung. */}
        {row.homework?.map((entry, index) => (
          <div
            key={`${index}:${homeworkLine(entry)}`}
            className="mt-0.5 text-sm break-words text-gray-700"
          >
            Hausaufgaben: {homeworkLine(entry)}
          </div>
        ))}
        {row.homeworkFeedback ? (
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <StatusColorBadge
              label={`Rückmeldung: ${HOMEWORK_FEEDBACK_LABELS[row.homeworkFeedback.status]}`}
              color={HOMEWORK_FEEDBACK_COLORS[row.homeworkFeedback.status]}
            />
            {row.homeworkFeedback.note ? (
              <span className="text-sm break-words text-gray-600">
                {row.homeworkFeedback.note}
              </span>
            ) : null}
          </div>
        ) : null}
      </div>
      <RosterRowActions
        row={row}
        onAction={onAction}
        access={actionAccess}
        profileHref={studentProfileHref?.(row)}
        onHomework={onHomework}
      />
    </div>
  );
}

interface TimetableRosterSectionProps {
  readonly actionAccess: RosterActionAccess;
  /** Folded when the roster opens; nothing remembers the reader's choice. */
  readonly defaultCollapsed?: boolean;
  /** One line under the section title — for a precondition the rows share. */
  readonly description?: string;
  readonly instanceIsSpontaneous: boolean;
  readonly now: Date;
  readonly rosterDate: string;
  readonly pickupTimesLoaded?: boolean;
  readonly pickupTimesRedacted?: boolean;
  readonly onAction: RosterRowActionsProps["onAction"];
  readonly onOpenStudent?: (row: TimetableRosterRow) => void;
  readonly studentProfileHref?: (row: TimetableRosterRow) => string;
  readonly onHomework?: (row: TimetableRosterRow) => void;
  readonly rows: TimetableRosterRow[];
  readonly showTimetableCounts: boolean;
  readonly title: string;
}

function TimetableRosterSection({
  actionAccess,
  defaultCollapsed = false,
  description,
  instanceIsSpontaneous,
  now,
  rosterDate,
  pickupTimesLoaded,
  pickupTimesRedacted,
  onAction,
  onOpenStudent,
  studentProfileHref,
  onHomework,
  rows,
  showTimetableCounts,
  title,
}: TimetableRosterSectionProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed);
  const bodyId = useId();
  if (rows.length === 0) return null;
  const countLabel = showTimetableCounts ? ` (${rows.length})` : "";

  return (
    <section className="moto-content-surface overflow-hidden rounded-lg border">
      <div
        className={`grid grid-cols-[1fr_auto] items-center gap-x-2 bg-gray-50 py-1 pr-2 pl-4 ${collapsed ? "" : "border-b border-gray-100"}`}
      >
        <span className="text-sm font-semibold text-gray-700">
          {title}
          {countLabel}
        </span>
        <SectionCollapseToggle
          title={title}
          collapsed={collapsed}
          controls={collapsed ? undefined : bodyId}
          onToggle={() => setCollapsed((value) => !value)}
        />
        {description ? (
          <p className="col-span-2 -mt-1 pb-1 text-xs font-normal text-gray-500">
            {description}
          </p>
        ) : null}
      </div>
      {collapsed ? null : (
        <div id={bodyId}>
          {rows.map((row) => (
            <TimetableRosterStudentRow
              key={`${row.studentId}-${row.status}-${row.visitId ?? "planned"}`}
              actionAccess={actionAccess}
              instanceIsSpontaneous={instanceIsSpontaneous}
              now={now}
              rosterDate={rosterDate}
              pickupTimesLoaded={pickupTimesLoaded}
              pickupTimesRedacted={pickupTimesRedacted}
              row={row}
              onAction={onAction}
              onOpenStudent={onOpenStudent}
              studentProfileHref={studentProfileHref}
              onHomework={onHomework}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function confirmExpectedLabel(
  isConfirmingExpected: boolean,
  showTimetableCounts: boolean,
  count: number,
): string {
  if (isConfirmingExpected) return "Bestätigt...";
  if (showTimetableCounts) return `${count} erwartete bestätigen`;
  return "Erwartete bestätigen";
}

interface TimetableRosterHeaderProps {
  readonly attendanceWebEnabled: boolean;
  readonly endEnabled: boolean;
  readonly confirmableExpectedRows: TimetableRosterRow[];
  readonly isCompletingInstance: boolean;
  readonly isConfirmingExpected: boolean;
  readonly roster: TimetableRoster;
  readonly showTimetableCounts: boolean;
  readonly now: Date;
  readonly summary: {
    readonly absent: number;
    readonly arrivingLater: number;
    readonly departed: number;
    readonly expected: number;
    readonly present: number;
    readonly unplanned: number;
  };
  readonly note?: string;
  readonly occupancy?: TimetableRosterContentProps["occupancy"];
  readonly nfcEnabled: boolean;
  readonly extraActions?: ReactNode;
  readonly toggle?: ReactNode;
  readonly showStatus: boolean;
  readonly showRoom: boolean;
  /** Öffnet den Dialog „Kind ungeplant hinzufügen“; fehlt ohne das Recht. */
  readonly onAddStudent?: () => void;
  readonly onAddPresentStudents?: () => void;
  readonly onComplete: () => Promise<void>;
  readonly onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void>;
}

function TimetableRosterHeader({
  attendanceWebEnabled,
  endEnabled,
  confirmableExpectedRows,
  isCompletingInstance,
  isConfirmingExpected,
  now,
  roster,
  showTimetableCounts,
  summary,
  note,
  occupancy,
  nfcEnabled,
  extraActions,
  toggle,
  showStatus,
  showRoom,
  onAddStudent,
  onAddPresentStudents,
  onComplete,
  onConfirmExpected,
}: TimetableRosterHeaderProps) {
  const completeEnabled = canCompleteInstance(
    roster.instance.canComplete,
    roster.instance.completeAvailableAt,
    now,
  );
  const handleConfirmExpectedClick = async () => {
    await onConfirmExpected(confirmableExpectedRows);
  };
  const handleCompleteClick = async () => {
    await onComplete();
  };
  const confirmLabel = confirmExpectedLabel(
    isConfirmingExpected,
    showTimetableCounts,
    confirmableExpectedRows.length,
  );

  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border border-gray-200 shadow-sm backdrop-blur-md">
      {/* Auf dem Telefon steht der Einklapp-Pfeil neben dem Titel und die
          Knöpfe darunter; ab sm steht der Pfeil hinter den Knöpfen, wie in
          der eingeklappten Karte. */}
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b border-gray-100 p-4 sm:flex">
        {/* Ab sm hält der Titel seine Mindestbreite: viele Knöpfe brechen
            um, statt Raum und Uhrzeit abzuschneiden. */}
        <div className="flex min-w-0 items-center gap-3 sm:min-w-64">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100">
            <MotoConceptIcon concept="present" size={18} />
          </span>
          <div className="min-w-0">
            {/* Der Kicker folgt dem Zustand des Blocks. Fest "Aktiv" war
                gelogen, sobald der Roster eines beendeten Termins offen ist —
                im Reopen-Fenster der OGS ebenso wie in der Aufsicht einer
                Lehrkraft (#2527). Er heißt wie das Abzeichen im Seitenkopf
                („Aktiv“ / „Beendet“, #4354). */}
            {showStatus ? (
              <p
                className={`text-xs font-semibold tracking-wide uppercase ${
                  roster.instance.status === "completed"
                    ? "text-gray-500"
                    : "text-moto-green-strong"
                }`}
              >
                {rosterRunState(roster.instance.status).label}
              </p>
            ) : null}
            <h2 className="truncate text-base font-semibold text-gray-900">
              {roster.instance.title}
            </h2>
            <p className="truncate text-sm text-gray-600">
              {showRoom
                ? `${roster.instance.roomName ?? `Raum ${roster.instance.roomId}`} · `
                : null}
              {hasOpenEnd(roster.instance)
                ? `seit ${roster.instance.startTime}`
                : `${roster.instance.startTime}-${roster.instance.endTime}`}
            </p>
            {occupancy?.limit != null ? (
              <div className="mt-1 flex flex-wrap gap-1.5">
                <OccupancyBadges
                  occupancy={occupancy}
                  nfcEnabled={nfcEnabled}
                  compact
                />
              </div>
            ) : null}
          </div>
        </div>
        {toggle ? <div className="sm:order-last">{toggle}</div> : null}
        <div className="col-span-2 flex flex-wrap gap-2 sm:ml-auto sm:justify-end">
          {extraActions}
          {attendanceWebEnabled && onAddPresentStudents ? (
            <Button
              type="button"
              onClick={onAddPresentStudents}
              variant="outline"
              size="md"
              className="bg-white"
            >
              <Users className="h-4 w-4" aria-hidden="true" />
              Anwesende Kinder
            </Button>
          ) : null}
          {attendanceWebEnabled && onAddStudent ? (
            <Button
              type="button"
              onClick={onAddStudent}
              variant="outline"
              size="md"
              className="bg-white"
            >
              <UserPlus className="h-4 w-4" aria-hidden="true" />
              Kind hinzufügen
            </Button>
          ) : null}
          {attendanceWebEnabled ? (
            <Button
              type="button"
              disabled={
                isConfirmingExpected || confirmableExpectedRows.length === 0
              }
              onClick={handleConfirmExpectedClick}
              variant="success"
              size="md"
            >
              <MotoConceptIcon concept="present" size={16} />
              {confirmLabel}
            </Button>
          ) : null}
          {endEnabled ? (
            <Button
              type="button"
              disabled={isCompletingInstance || !completeEnabled}
              onClick={handleCompleteClick}
              variant="primary"
              size="md"
            >
              {completeEnabled
                ? "Beenden"
                : `Beenden ab ${completeAvailableClock(roster.instance.completeAvailableAt, roster.instance.endTime)}`}
            </Button>
          ) : null}
        </div>
      </div>
      {note ? (
        <p className="border-b border-gray-100 px-4 py-3 text-sm text-gray-600">
          {note}
        </p>
      ) : null}
      {showTimetableCounts && roster.instance.isSpontaneous ? (
        // Ein spontaner Block hat keine eigenen Kinder: alle sind ungeplant
        // dazugekommen. Anwesend, Erwartet und Abwesend wären immer 0, die
        // Kinder im Raum heißen hier wie die Liste darunter (#3921).
        <div className="grid grid-cols-2 gap-2 p-4">
          <RosterSummaryStat label="Teilnehmende" value={summary.unplanned} />
          <RosterSummaryStat label="Gegangen" value={summary.departed} />
        </div>
      ) : showTimetableCounts ? (
        <div
          className={`grid grid-cols-2 gap-2 p-4 ${
            summary.arrivingLater > 0 ? "sm:grid-cols-6" : "sm:grid-cols-5"
          }`}
        >
          <RosterSummaryStat label="Anwesend" value={summary.present} />
          <RosterSummaryStat label="Erwartet" value={summary.expected} />
          {summary.arrivingLater > 0 ? (
            <RosterSummaryStat
              label="Kommt später"
              value={summary.arrivingLater}
            />
          ) : null}
          <RosterSummaryStat label="Abwesend" value={summary.absent} />
          <RosterSummaryStat label="Gegangen" value={summary.departed} />
          <RosterSummaryStat label="Ungeplant" value={summary.unplanned} />
        </div>
      ) : null}
    </div>
  );
}

interface TimetableRosterContentProps {
  readonly addStudentSearch: string;
  readonly attendanceWebEnabled: boolean;
  readonly isAddingStudent: boolean;
  readonly isCompletingInstance: boolean;
  readonly isConfirmingExpected: boolean;
  readonly roster: TimetableRoster;
  readonly showTimetableCounts: boolean;
  /**
   * Kinder der laufenden Sitzung und die Teilnehmergrenze der Aktivität
   * (#3634). Mit Grenze steht „Anzahl / Grenze“ im Kopf, über der Grenze
   * dazu „Überbucht“ und der Hinweis. Ohne Grenze bleibt der Kopf wie er ist.
   */
  readonly occupancy?: Occupancy;
  /**
   * Erlaubt das Nachtragen eines Kindes, das nicht auf der Liste steht. Das
   * Schul-Portal setzt `false` (#2527): eine Lehrkraft darf nur die Kinder
   * ihrer Aufsicht anfassen und hat keine Kindersuche, hinter der das
   * Suchfeld etwas finden könnte.
   */
  readonly canAddUnplanned?: boolean;
  /**
   * Eine Zeile in der Kopfkarte, direkt über den Zahlen. Für Hinweise, die zur
   * Liste gehören und deshalb auf ihrer Fläche stehen müssen — freier Text
   * zwischen den Karten liegt sonst auf dem nackten Seitenhintergrund.
   */
  readonly headerNote?: string;
  /** Weitere Knöpfe vor den Aktionen der Kopfkarte, z. B. „Betreuer hinzufügen“. */
  readonly headerActions?: ReactNode;
  /**
   * Einklapp-Pfeil der Kopfkarte, wenn die Liste ein Abschnitt einer Seite
   * mit mehreren Blöcken ist (#3281).
   */
  readonly headerToggle?: ReactNode;
  /**
   * Der Seitenkopf sagt schon, ob der Block läuft (#4354): die Kopfkarte
   * wiederholt „Aktiv“ / „Beendet“ dann nicht. Ein Raum mit mehreren Blöcken
   * lässt das weg, dort trägt jede Karte ihren eigenen Zustand.
   */
  readonly statusInPageHead?: boolean;
  /**
   * Der Seitenkopf nennt den Raum schon (#4354): in der eigenen Aufsicht in
   * der Statuszeile, im offenen Raum als Titel. Die Kopfkarte zeigt dann nur
   * die Uhrzeit.
   */
  readonly roomInPageHead?: boolean;
  readonly onAddStudent: (studentId: string) => Promise<boolean>;
  /**
   * Trägt eine Auswahl der gerade anwesenden Kinder gesammelt ein (#3824).
   * Ohne ihn fehlt die Kopf-Aktion „Anwesende Kinder“.
   */
  readonly onAddPresentStudents?: (studentIds: string[]) => Promise<boolean>;
  /**
   * Der Dialog „Anwesende Kinder hinzufügen“. Die Tenant-Seiten reichen ihn
   * herein, damit das Schulportal, das ihn nie zeigt, ihn auch nicht mitlädt.
   */
  readonly presentChildrenPicker?: ComponentType<PresentChildrenPickerProps>;
  /**
   * Der Dialog „Kind ungeplant hinzufügen“. Er lädt alle Kinder der Schule;
   * wie die Auswahl oben reichen ihn nur die Tenant-Seiten herein (#4355).
   */
  readonly addUnplannedStudentModal?: ComponentType<AddUnplannedStudentModalProps>;
  /**
   * Öffnet die Auswahl „Anwesende Kinder hinzufügen“ einmal von selbst, z. B.
   * direkt nach dem Start einer spontanen Aktivität.
   */
  readonly presentPickerAutoOpen?: boolean;
  readonly onPresentPickerAutoOpened?: () => void;
  readonly onComplete: () => Promise<void>;
  readonly onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void>;
  readonly onRosterAction: RosterRowActionsProps["onAction"];
  /**
   * Entschuldigt das Kind für diesen Block und den Rest des Tages (#3166).
   * Nur wer Teilabwesenheiten anlegen darf (`users:update`), bekommt ihn;
   * ohne ihn entschuldigt „Entschuldigt“ sofort nur diesen Block. Das
   * Schul-Portal reicht ihn bewusst nicht durch: eine Lehrkraft entscheidet
   * nur über die eigene Aufsicht.
   */
  readonly onExcuseRestOfDay?: (row: TimetableRosterRow) => Promise<void>;
  readonly onOpenStudent?: (row: TimetableRosterRow) => void;
  /**
   * Speichert die Rückmeldung zu den Hausaufgaben eines Kindes (#4796).
   * Ohne sie fehlt der Knopf „Hausaufgaben“; die Hausaufgaben stehen dann nur
   * zum Lesen am Kind.
   */
  readonly onSaveHomeworkFeedback?: (
    row: TimetableRosterRow,
    body: HomeworkFeedbackBody,
  ) => Promise<void>;
  /**
   * Adresse der Kinderseite für den Knopf „Profil ansehen“ (#4356). Nur wer
   * die Kinderseite öffnen darf (`users:read`), bekommt sie; ohne sie fehlt
   * der Knopf.
   */
  readonly studentProfileHref?: (row: TimetableRosterRow) => string;
  readonly onSearchChange: (value: string) => void;
  /**
   * Suche und Filter der Seite (#3889). Sie engen die Abschnitte ein, die
   * Zahlen im Kopf bleiben die des ganzen Blocks. Ohne Filter alle Zeilen.
   */
  readonly rowFilter?: ((row: TimetableRosterRow) => boolean) | null;
  /** Fehler des Nachtragens; steht im Dialog „Kind ungeplant hinzufügen“. */
  readonly addStudentError?: FormErrorInput;
}

/**
 * Sorts a roster's rows into the sections of the list. Runs once over every
 * row for the head figures and the bulk confirm, and once over the rows the
 * page's search and filters leave for the sections on screen (#3889).
 */
function rosterSections(
  rows: readonly TimetableRosterRow[],
  now: Date,
  date: string,
) {
  const present = rows.filter((row) => row.currentlyPresent && row.planned);
  // The care plan decides who counts as expected (#1747): rows the plan does
  // not place here today — not booked, or the day was cancelled — go into
  // their own section below, never into "Erwartet" and never into the bulk
  // confirm, which would persist attendance for a child who is not coming.
  const stillExpected = rows.filter(
    (row) =>
      row.planned &&
      !row.currentlyPresent &&
      row.status === "expected" &&
      isCareDayExpected(row.careDayStatus),
  );
  // A child whose expected arrival is still ahead (six lessons instead of
  // five, #2878) is not expected yet: it gets its own "Kommt später" section
  // and stays out of the bulk confirm, which would check it in prematurely.
  // Once the minute clock passes the arrival time the row moves to "Erwartet"
  // by itself.
  const arrivingLater = stillExpected.filter(
    (row) => upcomingArrivalTime(row.warnings, now, date) !== null,
  );
  const expected = stillExpected.filter(
    (row) => upcomingArrivalTime(row.warnings, now, date) === null,
  );
  // An absence a sick / excused / class-trip day status wrote onto a day the
  // child was never booked into care belongs here too, not under "Abwesend":
  // the block has not ended yet, so nothing has undone that false absence, and
  // the header count already groups it this way (#1747).
  const notScheduled = rows.filter(
    (row) =>
      row.planned &&
      !row.currentlyPresent &&
      isNotScheduledRow(row.status, row.careDayStatus),
  );
  const absent = rows.filter(
    (row) =>
      row.planned &&
      !row.currentlyPresent &&
      row.status === "absent" &&
      !isNotScheduledRow(row.status, row.careDayStatus),
  );
  const departed = rows.filter(
    (row) =>
      !row.currentlyPresent &&
      (row.status === "present" || (row.isUnplanned && row.visitId)),
  );
  const unplanned = rows.filter(
    (row) => row.isUnplanned && row.currentlyPresent,
  );
  return {
    present,
    expected,
    arrivingLater,
    notScheduled,
    absent,
    departed,
    unplanned,
  };
}

export function TimetableRosterContent({
  addStudentSearch,
  attendanceWebEnabled,
  isAddingStudent,
  isCompletingInstance,
  isConfirmingExpected,
  roster,
  showTimetableCounts,
  occupancy,
  canAddUnplanned = true,
  headerNote,
  headerActions,
  headerToggle,
  statusInPageHead = false,
  roomInPageHead = false,
  onAddStudent,
  onAddPresentStudents,
  presentChildrenPicker: PresentChildrenPicker,
  addUnplannedStudentModal: AddUnplannedStudentModal,
  presentPickerAutoOpen = false,
  onPresentPickerAutoOpened,
  onComplete,
  onConfirmExpected,
  onRosterAction,
  onExcuseRestOfDay,
  onOpenStudent,
  onSaveHomeworkFeedback,
  studentProfileHref,
  onSearchChange,
  rowFilter = null,
  addStudentError,
}: TimetableRosterContentProps) {
  const now = useMinuteClock();
  // Seeing a running block does not mean acting on it (#3167): staff who only
  // see it through the all_staff overview get the list without actions and a
  // line that says why. Older backends omit the flag and keep the actions.
  const lifecycleEnabled = attendanceWebEnabled && roster.canOperate !== false;
  // Ending has its own school setting (#3622); older backends omit the flag.
  const endEnabled =
    attendanceWebEnabled && (roster.canEnd ?? roster.canOperate) !== false;
  const actionsEnabled =
    attendanceWebEnabled &&
    (roster.canEditAttendance ?? roster.canOperate ?? true);
  const actionAccess: RosterActionAccess = {
    attendance: actionsEnabled,
    statuses: lifecycleEnabled,
    absence:
      attendanceWebEnabled && (roster.canReportAbsence ?? lifecycleEnabled),
  };
  const viewOnly =
    attendanceWebEnabled &&
    !actionsEnabled &&
    !lifecycleEnabled &&
    !endEnabled &&
    !actionAccess.absence;
  const nfcEnabled = useNFCEnabled();
  const overbookedNote = occupancy
    ? overbookedHintFor(occupancy, nfcEnabled)
    : null;
  const note = [
    viewOnly ? TIMETABLE_VIEW_ONLY_NOTICE : undefined,
    headerNote,
    overbookedNote,
  ]
    .filter(Boolean)
    .join(" ");
  const [addStudentOpen, setAddStudentOpen] = useState(false);
  const [presentPickerOpen, setPresentPickerOpen] = useState(false);
  const addStudentEnabled =
    canAddUnplanned && AddUnplannedStudentModal !== undefined;
  const presentPickerEnabled =
    actionsEnabled &&
    canAddUnplanned &&
    onAddPresentStudents !== undefined &&
    PresentChildrenPicker !== undefined;
  useEffect(() => {
    if (!presentPickerAutoOpen) return;
    if (presentPickerEnabled) setPresentPickerOpen(true);
    onPresentPickerAutoOpened?.();
  }, [presentPickerAutoOpen, presentPickerEnabled, onPresentPickerAutoOpened]);
  const [excuseRow, setExcuseRow] = useState<TimetableRosterRow | null>(null);
  const [isExcusing, setIsExcusing] = useState(false);
  // Mit dem Recht für Teilabwesenheiten fragt „Entschuldigt“ zuerst, ob nur
  // dieser Block oder der Rest des Tages gemeint ist (#3166).
  const handleRosterAction: RosterRowActionsProps["onAction"] = async (
    action,
    row,
  ) => {
    if (action === "excused" && onExcuseRestOfDay) {
      setExcuseRow(row);
      return;
    }
    await onRosterAction(action, row);
  };
  const handleExcuseScope = async (scope: string) => {
    if (!excuseRow) return;
    setIsExcusing(true);
    try {
      if (scope === EXCUSE_SCOPE_REST_OF_DAY && onExcuseRestOfDay) {
        await onExcuseRestOfDay(excuseRow);
      } else {
        await onRosterAction("excused", excuseRow);
      }
    } finally {
      setIsExcusing(false);
      setExcuseRow(null);
    }
  };
  // Rückmeldung zu den Hausaufgaben (#4796): eine Lernzeit mit dem Recht
  // dazu. Der Dialog sammelt Antwort und Notiz und speichert erst dann.
  const homeworkEnabled =
    roster.canRecordHomeworkFeedback === true &&
    onSaveHomeworkFeedback !== undefined;
  const [homeworkRow, setHomeworkRow] = useState<TimetableRosterRow | null>(
    null,
  );
  const [homeworkStatus, setHomeworkStatus] =
    useState<TimetableHomeworkFeedbackStatus | null>(null);
  const [homeworkNote, setHomeworkNote] = useState("");
  const [savingHomework, setSavingHomework] = useState(false);
  const homeworkErrors = useApiFormError();
  const openHomework = (row: TimetableRosterRow) => {
    homeworkErrors.clear();
    setHomeworkRow(row);
    setHomeworkStatus(row.homeworkFeedback?.status ?? null);
    setHomeworkNote(row.homeworkFeedback?.note ?? "");
  };
  const closeHomework = () => {
    homeworkErrors.clear();
    setHomeworkRow(null);
  };
  const saveHomework = async () => {
    if (!homeworkRow || !onSaveHomeworkFeedback) return;
    if (!homeworkStatus) {
      homeworkErrors.invalid("Bitte wählen Sie eine Antwort.");
      return;
    }
    homeworkErrors.clear();
    setSavingHomework(true);
    try {
      await onSaveHomeworkFeedback(homeworkRow, {
        status: homeworkStatus,
        note: homeworkNote.trim() === "" ? null : homeworkNote.trim(),
      });
      setHomeworkRow(null);
    } catch (cause) {
      void homeworkErrors.show(cause, {
        object: "die Rückmeldung",
        retry: () => void saveHomework(),
      });
    } finally {
      setSavingHomework(false);
    }
  };
  const closeAddStudent = () => {
    setAddStudentOpen(false);
    // Der nächste Dialog startet ohne Suche und ohne Fehlermeldung; beides
    // wird vom gemeinsamen Such-Handler zurückgesetzt.
    onSearchChange("");
  };
  const closePresentPicker = () => {
    setPresentPickerOpen(false);
    // Fehler gehören zu diesem Dialog; der gemeinsame Such-Handler räumt sie.
    onSearchChange("");
  };
  const inBlockStudentIds = new Set(
    roster.rows
      .filter((row) => row.currentlyPresent)
      .map((row) => row.studentId),
  );
  const all = rosterSections(roster.rows, now, roster.instance.date);
  const { present, expected, arrivingLater, absent, departed, unplanned } = all;
  // The page's search and filters narrow the sections, never the figures in
  // the head: those describe the block, not the search (#3889).
  const visible = rowFilter
    ? rosterSections(roster.rows.filter(rowFilter), now, roster.instance.date)
    : all;
  const isNarrowed = rowFilter !== null;
  const hasNoMatch = rowFilter ? !roster.rows.some(rowFilter) : false;
  const confirmableExpectedRows = expected.filter(
    (row) => row.planned && !row.currentlyPresent,
  );
  const instanceIsSpontaneous = roster.instance.isSpontaneous;
  const unplannedTitle = instanceIsSpontaneous ? "Teilnehmende" : "Ungeplant";
  const sectionProps = {
    actionAccess,
    instanceIsSpontaneous,
    now,
    rosterDate: roster.instance.date,
    pickupTimesLoaded: roster.pickupTimesLoaded,
    pickupTimesRedacted: roster.pickupTimesRedacted,
    onAction: handleRosterAction,
    onOpenStudent,
    studentProfileHref,
    onHomework: homeworkEnabled ? openHomework : undefined,
    showTimetableCounts,
  };

  return (
    <div className="space-y-4">
      <TimetableRosterHeader
        attendanceWebEnabled={actionsEnabled}
        endEnabled={endEnabled}
        confirmableExpectedRows={confirmableExpectedRows}
        isCompletingInstance={isCompletingInstance}
        isConfirmingExpected={isConfirmingExpected}
        now={now}
        roster={roster}
        showTimetableCounts={showTimetableCounts}
        note={note}
        occupancy={occupancy}
        nfcEnabled={nfcEnabled}
        extraActions={headerActions}
        toggle={headerToggle}
        showStatus={!statusInPageHead}
        showRoom={!roomInPageHead}
        onAddStudent={
          addStudentEnabled ? () => setAddStudentOpen(true) : undefined
        }
        onAddPresentStudents={
          presentPickerEnabled ? () => setPresentPickerOpen(true) : undefined
        }
        summary={{
          absent: absent.length,
          arrivingLater: arrivingLater.length,
          departed: departed.length,
          expected: expected.length,
          present: present.length,
          unplanned: unplanned.length,
        }}
        onComplete={onComplete}
        onConfirmExpected={onConfirmExpected}
      />
      {roster.pickupTimesLoaded === false && !roster.pickupTimesRedacted ? (
        <Alert
          type="warning"
          announce="polite"
          message="Die Gehzeiten konnten nicht geladen werden. Die Anwesenheitsliste bleibt verfügbar."
        />
      ) : null}
      {actionsEnabled && addStudentEnabled ? (
        <AddUnplannedStudentModal
          isOpen={addStudentOpen}
          instanceId={roster.instance.id}
          inBlockStudentIds={inBlockStudentIds}
          isAddingStudent={isAddingStudent}
          search={addStudentSearch}
          error={addStudentError}
          onAdd={onAddStudent}
          onClose={closeAddStudent}
          onSearchChange={onSearchChange}
        />
      ) : null}
      {presentPickerEnabled && onAddPresentStudents ? (
        <PresentChildrenPicker
          isOpen={presentPickerOpen}
          instanceId={roster.instance.id}
          inBlockStudentIds={inBlockStudentIds}
          isAdding={isAddingStudent}
          error={addStudentError}
          onAdd={onAddPresentStudents}
          onClose={closePresentPicker}
        />
      ) : null}
      {actionAccess.absence && onExcuseRestOfDay ? (
        <ExcuseScopeModal
          instance={roster.instance}
          row={excuseRow}
          isBusy={isExcusing}
          onClose={() => setExcuseRow(null)}
          onSelect={(scope) => void handleExcuseScope(scope)}
        />
      ) : null}
      {homeworkEnabled ? (
        <FormModal
          isOpen={homeworkRow !== null}
          onClose={closeHomework}
          title={`Hausaufgaben von ${homeworkRow?.studentName ?? ""}`}
          size="md"
          mobilePosition="bottom"
          closeDisabled={savingHomework}
          error={homeworkErrors.error}
          footer={
            <EditActions
              className="p-4"
              onCancel={closeHomework}
              onSave={() => void saveHomework()}
              saving={savingHomework}
              saveLabel="Rückmeldung speichern"
            />
          }
        >
          <div className="space-y-4 p-4">
            <ul className="space-y-1 text-sm text-gray-700">
              {homeworkRow?.homework?.map((entry, index) => (
                <li
                  key={`${index}:${homeworkLine(entry)}`}
                  className="break-words whitespace-pre-line"
                >
                  {homeworkLine(entry)}
                </li>
              ))}
            </ul>
            <div className="space-y-2">
              <p className="text-sm font-medium text-gray-700">
                Wie lief es heute?
              </p>
              <SegmentedControl
                items={HOMEWORK_FEEDBACK_STATUSES.map((status) => ({
                  value: status,
                  label: HOMEWORK_FEEDBACK_LABELS[status],
                  tone: HOMEWORK_FEEDBACK_TONES[status],
                  disabled: savingHomework,
                }))}
                value={homeworkStatus}
                onChange={setHomeworkStatus}
                variant="pills"
                ariaLabel="Rückmeldung zu den Hausaufgaben"
                fullWidth
              />
            </div>
            <Input
              id="homework-feedback-note"
              name="note"
              label="Kurze Notiz (freiwillig)"
              controlSize="compact"
              maxLength={HOMEWORK_NOTE_MAX_LENGTH}
              value={homeworkNote}
              onChange={(event) => setHomeworkNote(event.target.value)}
            />
          </div>
        </FormModal>
      ) : null}
      {hasNoMatch ? (
        <SectionCard>
          <EmptyStudentResults
            totalCount={roster.rows.length}
            filteredCount={0}
          />
        </SectionCard>
      ) : null}
      {/* Children in the room come first, planned or not (#4353). Each list
          folds from its head. A search or filter remounts the lists open so
          every hit shows; clearing it restores the opening state. */}
      <Fragment key={isNarrowed ? "narrowed" : "all"}>
        <TimetableRosterSection
          title="Anwesend"
          rows={visible.present}
          {...sectionProps}
        />
        <TimetableRosterSection
          title={unplannedTitle}
          rows={visible.unplanned}
          {...sectionProps}
        />
        <TimetableRosterSection
          title="Erwartet"
          rows={visible.expected}
          {...sectionProps}
        />
        <TimetableRosterSection
          title="Kommt später"
          rows={visible.arrivingLater}
          {...sectionProps}
        />
        <TimetableRosterSection
          title="Heute nicht eingeplant"
          rows={visible.notScheduled}
          {...sectionProps}
        />
        <TimetableRosterSection
          title="Entschuldigt / Abwesend"
          rows={visible.absent}
          {...sectionProps}
        />
        <TimetableRosterSection
          title="Nicht mehr im Raum"
          rows={visible.departed}
          defaultCollapsed={!isNarrowed}
          {...sectionProps}
        />
      </Fragment>
    </div>
  );
}
