"use client";

import { useState } from "react";
import { UserPlus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  clearOwnAttendanceMutation,
  markOwnAttendanceMutation,
} from "~/lib/sse-optimistic-mutations";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  rosterPickupTimeLabel,
  upcomingArrivalTime,
} from "~/lib/timetable-roster-helpers";
import { canCompleteInstance } from "~/lib/timetable-lifecycle";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { isCareDayExpected, isNotScheduledRow } from "~/lib/timetable-types";
import type { Student } from "~/lib/student-helpers";

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

function rosterStudentMeta(
  row: TimetableRosterRow,
  instanceIsSpontaneous: boolean,
): string {
  return [
    row.schoolClass,
    row.groupName,
    row.isUnplanned && !instanceIsSpontaneous ? "ungeplant" : "",
  ]
    .filter((value, index, values) => value && values.indexOf(value) === index)
    .join(" · ");
}

function RosterSummaryStat({
  label,
  value,
}: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-xl bg-white/80 px-3 py-2 shadow-[0_1px_0_rgba(17,24,39,0.04)]">
      <span className="block text-sm font-semibold text-gray-900">{value}</span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
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

interface RosterRowActionsProps {
  readonly row: TimetableRosterRow;
  readonly onAction: (
    action: RosterAction,
    row: TimetableRosterRow,
  ) => Promise<void>;
}

function RosterRowActions({ row, onAction }: RosterRowActionsProps) {
  const runAction = async (action: RosterAction) => {
    await onAction(action, row);
  };

  return (
    <div className="flex flex-wrap gap-2">
      {!row.currentlyPresent && row.status === "expected" ? (
        <Button
          type="button"
          onClick={() => runAction("check-in")}
          variant="success"
          size="md"
        >
          Einchecken
        </Button>
      ) : null}
      {!row.currentlyPresent && row.status !== "expected" ? (
        <Button
          type="button"
          onClick={() => runAction("check-in")}
          variant="success"
          size="md"
        >
          Wieder einchecken
        </Button>
      ) : null}
      {row.currentlyPresent ? (
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
          <Button
            type="button"
            onClick={() => runAction("excused")}
            variant="outline"
            size="md"
            className="text-moto-purple-strong !ring-moto-purple"
          >
            Entschuldigt
          </Button>
          <Button
            type="button"
            onClick={() => runAction("absent")}
            variant="outline"
            size="md"
          >
            Abwesend
          </Button>
        </>
      ) : null}
      {row.planned && !row.currentlyPresent && row.status === "absent" ? (
        <Button
          type="button"
          onClick={() => runAction("expected")}
          variant="outline"
          size="md"
        >
          Zurück auf erwartet
        </Button>
      ) : null}
    </div>
  );
}

interface TimetableRosterRowProps {
  readonly attendanceWebEnabled: boolean;
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
}

function TimetableRosterStudentRow({
  attendanceWebEnabled,
  instanceIsSpontaneous,
  now,
  rosterDate,
  pickupTimesLoaded,
  pickupTimesRedacted,
  row,
  onAction,
  onOpenStudent,
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
    <div className="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 flex-1">
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
        <div className="mt-1 text-sm text-gray-500">
          {rosterStudentMeta(row, instanceIsSpontaneous)}
        </div>
        {pickupTimeLabel === null ? null : (
          <div className="mt-1 text-sm font-medium text-gray-700">
            Gehzeit: {pickupTimeLabel}
          </div>
        )}
        {arrivalTime ? (
          <div className="mt-1 text-sm font-medium text-gray-700">
            Kommt um {arrivalTime} Uhr
          </div>
        ) : null}
        {classException && expectsArrival ? (
          <div className="mt-1 text-sm text-gray-700">
            {classException.message}
          </div>
        ) : null}
        {planningNotes.map((warning) => (
          <div
            key={`${warning.kind}:${warning.message}`}
            className="text-moto-amber-strong mt-1 text-sm"
          >
            {warning.message}
          </div>
        ))}
        {attendanceDetail ? (
          <div className="text-moto-amber-strong mt-1 text-sm">
            {attendanceDetail}
          </div>
        ) : null}
        {row.parallelPresentIn ? (
          <div className="text-moto-amber-strong mt-1 text-sm">
            {`Auch in „${row.parallelPresentIn.title}“ (${row.parallelPresentIn.startTime}–${row.parallelPresentIn.endTime}) als anwesend eingetragen`}
          </div>
        ) : null}
      </div>
      {attendanceWebEnabled ? (
        <RosterRowActions row={row} onAction={onAction} />
      ) : null}
    </div>
  );
}

interface TimetableRosterSectionProps {
  readonly attendanceWebEnabled: boolean;
  /** One line under the section title — for a precondition the rows share. */
  readonly description?: string;
  readonly instanceIsSpontaneous: boolean;
  readonly now: Date;
  readonly rosterDate: string;
  readonly pickupTimesLoaded?: boolean;
  readonly pickupTimesRedacted?: boolean;
  readonly onAction: RosterRowActionsProps["onAction"];
  readonly onOpenStudent?: (row: TimetableRosterRow) => void;
  readonly rows: TimetableRosterRow[];
  readonly showTimetableCounts: boolean;
  readonly title: string;
}

function TimetableRosterSection({
  attendanceWebEnabled,
  description,
  instanceIsSpontaneous,
  now,
  rosterDate,
  pickupTimesLoaded,
  pickupTimesRedacted,
  onAction,
  onOpenStudent,
  rows,
  showTimetableCounts,
  title,
}: TimetableRosterSectionProps) {
  if (rows.length === 0) return null;
  const countLabel = showTimetableCounts ? ` (${rows.length})` : "";

  return (
    <section className="moto-content-surface overflow-hidden rounded-lg border">
      <div className="border-b border-gray-100 bg-gray-50 px-4 py-2">
        <span className="text-sm font-semibold text-gray-700">
          {title}
          {countLabel}
        </span>
        {description ? (
          <p className="mt-0.5 text-xs font-normal text-gray-500">
            {description}
          </p>
        ) : null}
      </div>
      {rows.map((row) => (
        <TimetableRosterStudentRow
          key={`${row.studentId}-${row.status}-${row.visitId ?? "planned"}`}
          attendanceWebEnabled={attendanceWebEnabled}
          instanceIsSpontaneous={instanceIsSpontaneous}
          now={now}
          rosterDate={rosterDate}
          pickupTimesLoaded={pickupTimesLoaded}
          pickupTimesRedacted={pickupTimesRedacted}
          row={row}
          onAction={onAction}
          onOpenStudent={onOpenStudent}
        />
      ))}
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
  readonly onComplete: () => Promise<void>;
  readonly onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void>;
}

function TimetableRosterHeader({
  attendanceWebEnabled,
  confirmableExpectedRows,
  isCompletingInstance,
  isConfirmingExpected,
  now,
  roster,
  showTimetableCounts,
  summary,
  note,
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
      <div className="flex flex-col gap-3 border-b border-gray-100 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100">
            <MotoConceptIcon concept="present" size={18} />
          </span>
          <div className="min-w-0">
            {/* Der Kicker folgt dem Zustand des Blocks. Fest "Aktiv" war
                gelogen, sobald der Roster eines beendeten Termins offen ist —
                im Reopen-Fenster der OGS ebenso wie in der Aufsicht einer
                Lehrkraft (#2527). */}
            <p
              className={`text-xs font-semibold tracking-wide uppercase ${
                roster.instance.status === "completed"
                  ? "text-gray-500"
                  : "text-moto-green-strong"
              }`}
            >
              {roster.instance.status === "completed" ? "Beendet" : "Aktiv"}
            </p>
            <h2 className="truncate text-base font-semibold text-gray-900">
              {roster.instance.title}
            </h2>
            <p className="truncate text-sm text-gray-600">
              {roster.instance.roomName ?? `Raum ${roster.instance.roomId}`} ·{" "}
              {roster.instance.startTime}-{roster.instance.endTime}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2 sm:justify-end">
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
          {attendanceWebEnabled ? (
            <Button
              type="button"
              disabled={isCompletingInstance || !completeEnabled}
              onClick={handleCompleteClick}
              variant="primary"
              size="md"
            >
              {completeEnabled
                ? "Beenden"
                : `Beenden ab ${roster.instance.endTime}`}
            </Button>
          ) : null}
        </div>
      </div>
      {note ? (
        <p className="border-b border-gray-100 px-4 py-3 text-sm text-gray-600">
          {note}
        </p>
      ) : null}
      {showTimetableCounts ? (
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

interface AddUnplannedStudentFormProps {
  readonly isAddingStudent: boolean;
  readonly results: Student[];
  readonly search: string;
  readonly onAdd: (studentId: string) => Promise<boolean>;
  readonly onSearchChange: (value: string) => void;
}

function AddUnplannedStudentForm({
  isAddingStudent,
  results,
  search,
  onAdd,
  onSearchChange,
}: AddUnplannedStudentFormProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // Derived against the current results so a stale selection from a previous
  // search can never add the wrong child.
  const selectedStudent =
    results.find((student) => student.id.toString() === selectedId) ?? null;
  const targetStudent =
    selectedStudent ?? (results.length === 1 ? (results[0] ?? null) : null);
  const addStudent = async (studentId: string) => {
    if (await onAdd(studentId)) {
      setSelectedId(null);
    }
  };
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (targetStudent && !isAddingStudent) {
      await addStudent(targetStudent.id.toString());
    }
  };

  return (
    <form
      className="moto-content-surface rounded-2xl border p-4 shadow-sm"
      onSubmit={handleSubmit}
    >
      <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <UserPlus
          className="text-moto-green-vivid h-4 w-4"
          aria-hidden="true"
        />
        Kind ungeplant hinzufügen
      </div>
      <div className="flex flex-col gap-2 sm:flex-row">
        <div className="flex-1">
          <Input
            type="search"
            name="unplanned-student-search"
            aria-label="Kind ungeplant suchen"
            controlSize="compact"
            value={search}
            onChange={(event) => {
              setSelectedId(null);
              onSearchChange(event.target.value);
            }}
            placeholder="Weiteres Kind suchen..."
          />
        </div>
        <Button
          type="submit"
          disabled={isAddingStudent || !targetStudent}
          variant="success"
          size="md"
        >
          Hinzufügen
        </Button>
      </div>
      {results.length > 0 ? (
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {results.map((student) => {
            const studentId = student.id.toString();
            const isSelected = selectedStudent?.id.toString() === studentId;
            return (
              <Button
                key={student.id}
                type="button"
                variant="ghost"
                size="md"
                disabled={isAddingStudent}
                aria-pressed={isSelected}
                onClick={() =>
                  setSelectedId((prev) =>
                    prev === studentId ? null : studentId,
                  )
                }
                className={`min-h-11 w-full !justify-start border px-3 text-left !shadow-none ${
                  isSelected
                    ? "!border-moto-green !bg-moto-green/10 hover:!bg-moto-green/15"
                    : "hover:!border-moto-green !border-gray-200 !bg-transparent hover:!bg-gray-100"
                }`}
              >
                <span className="font-medium text-gray-900">
                  {student.name ||
                    [student.first_name, student.second_name]
                      .filter(Boolean)
                      .join(" ")}
                </span>
                <span className="ml-2 text-gray-500">
                  {[student.school_class, student.group_name]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </Button>
            );
          })}
        </div>
      ) : null}
      {results.length > 1 && !selectedStudent ? (
        <p className="mt-2 text-sm text-gray-500">
          Bitte ein Kind aus der Liste antippen.
        </p>
      ) : null}
    </form>
  );
}

interface TimetableRosterContentProps {
  readonly addStudentResults: Student[];
  readonly addStudentSearch: string;
  readonly attendanceWebEnabled: boolean;
  readonly isAddingStudent: boolean;
  readonly isCompletingInstance: boolean;
  readonly isConfirmingExpected: boolean;
  readonly roster: TimetableRoster;
  readonly showTimetableCounts: boolean;
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
  readonly onAddStudent: (studentId: string) => Promise<boolean>;
  readonly onComplete: () => Promise<void>;
  readonly onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void>;
  readonly onRosterAction: RosterRowActionsProps["onAction"];
  readonly onOpenStudent?: (row: TimetableRosterRow) => void;
  readonly onSearchChange: (value: string) => void;
}

export function TimetableRosterContent({
  addStudentResults,
  addStudentSearch,
  attendanceWebEnabled,
  isAddingStudent,
  isCompletingInstance,
  isConfirmingExpected,
  roster,
  showTimetableCounts,
  canAddUnplanned = true,
  headerNote,
  onAddStudent,
  onComplete,
  onConfirmExpected,
  onRosterAction,
  onOpenStudent,
  onSearchChange,
}: TimetableRosterContentProps) {
  const now = useMinuteClock();
  const present = roster.rows.filter(
    (row) => row.currentlyPresent && row.planned,
  );
  // The care plan decides who counts as expected (#1747): rows the plan does
  // not place here today — not booked, or the day was cancelled — go into
  // their own section below, never into "Erwartet" and never into the bulk
  // confirm, which would persist attendance for a child who is not coming.
  const stillExpected = roster.rows.filter(
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
    (row) =>
      upcomingArrivalTime(row.warnings, now, roster.instance.date) !== null,
  );
  const expected = stillExpected.filter(
    (row) =>
      upcomingArrivalTime(row.warnings, now, roster.instance.date) === null,
  );
  // An absence a sick / excused / class-trip day status wrote onto a day the
  // child was never booked into care belongs here too, not under "Abwesend":
  // the block has not ended yet, so nothing has undone that false absence, and
  // the header count already groups it this way (#1747).
  const notScheduled = roster.rows.filter(
    (row) =>
      row.planned &&
      !row.currentlyPresent &&
      isNotScheduledRow(row.status, row.careDayStatus),
  );
  const absent = roster.rows.filter(
    (row) =>
      row.planned &&
      !row.currentlyPresent &&
      row.status === "absent" &&
      !isNotScheduledRow(row.status, row.careDayStatus),
  );
  const departed = roster.rows.filter(
    (row) =>
      !row.currentlyPresent &&
      (row.status === "present" || (row.isUnplanned && row.visitId)),
  );
  const unplanned = roster.rows.filter(
    (row) => row.isUnplanned && row.currentlyPresent,
  );
  const confirmableExpectedRows = expected.filter(
    (row) => row.planned && !row.currentlyPresent,
  );
  const instanceIsSpontaneous = roster.instance.isSpontaneous;
  const unplannedTitle = instanceIsSpontaneous ? "Teilnehmende" : "Ungeplant";
  const sectionProps = {
    attendanceWebEnabled,
    instanceIsSpontaneous,
    now,
    rosterDate: roster.instance.date,
    pickupTimesLoaded: roster.pickupTimesLoaded,
    pickupTimesRedacted: roster.pickupTimesRedacted,
    onAction: onRosterAction,
    onOpenStudent,
    showTimetableCounts,
  };

  return (
    <div className="space-y-4">
      <TimetableRosterHeader
        attendanceWebEnabled={attendanceWebEnabled}
        confirmableExpectedRows={confirmableExpectedRows}
        isCompletingInstance={isCompletingInstance}
        isConfirmingExpected={isConfirmingExpected}
        now={now}
        roster={roster}
        showTimetableCounts={showTimetableCounts}
        note={headerNote}
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
      {attendanceWebEnabled && canAddUnplanned ? (
        <AddUnplannedStudentForm
          key={roster.instance.id}
          isAddingStudent={isAddingStudent}
          results={addStudentResults}
          search={addStudentSearch}
          onAdd={onAddStudent}
          onSearchChange={onSearchChange}
        />
      ) : null}
      <TimetableRosterSection
        title="Anwesend"
        rows={present}
        {...sectionProps}
      />
      <TimetableRosterSection
        title="Erwartet"
        rows={expected}
        {...sectionProps}
      />
      <TimetableRosterSection
        title="Kommt später"
        description={
          attendanceWebEnabled
            ? "Diese Kinder kommen laut Plan später. Bei „Erwartete bestätigen“ sind sie nicht dabei. Kommt ein Kind früher, checken Sie es hier einzeln ein."
            : "Diese Kinder kommen laut Plan später."
        }
        rows={arrivingLater}
        {...sectionProps}
      />
      <TimetableRosterSection
        title="Heute nicht eingeplant"
        rows={notScheduled}
        {...sectionProps}
      />
      <TimetableRosterSection
        title="Entschuldigt / Abwesend"
        rows={absent}
        {...sectionProps}
      />
      <TimetableRosterSection
        title="Nicht mehr im Raum"
        rows={departed}
        {...sectionProps}
      />
      <TimetableRosterSection
        title={unplannedTitle}
        rows={unplanned}
        {...sectionProps}
      />
    </div>
  );
}
