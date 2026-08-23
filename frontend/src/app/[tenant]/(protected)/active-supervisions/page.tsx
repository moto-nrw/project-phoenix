"use client";

import {
  useState,
  useEffect,
  Suspense,
  useMemo,
  useCallback,
  useRef,
} from "react";
import { LogOut, UserPlus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { redirect } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import { useOptionalSupervision } from "~/lib/supervision-context";
import {
  clearOwnAttendanceMutation,
  markOwnAttendanceMutation,
} from "~/lib/sse-optimistic-mutations";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import {
  CardGridSkeleton,
  PageHeaderSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import {
  StudentCard,
  StudentInfoRow,
  SchoolClassIcon,
  GroupIcon,
  PickupTimeRow,
  ArrivalTimeRow,
  StudentAbsenceRow,
  StudentPendingExcusedRow,
} from "~/components/students/student-card";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";
import type { BulkArrivalTime } from "~/lib/student-arrival-api";
import { berlinTodayISO } from "~/lib/date-helpers";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { canCompleteInstance } from "~/lib/timetable-lifecycle";
import { createLogger } from "~/lib/logger";
import { activeService } from "~/lib/active-api";
import { fetchStudents } from "~/lib/student-api";
import {
  isReopenUnavailableError,
  timetableOperationsApi,
} from "~/lib/timetable-operations-api";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { isCareDayExpected, isNotScheduledRow } from "~/lib/timetable-types";
import { isAdmin, isCaregiver } from "~/lib/auth-utils";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import type { Student } from "~/lib/student-helpers";
import {
  SCHOOL_YEAR_FILTER_OPTIONS,
  getSchoolYear,
} from "~/lib/student-helpers";
import { UnclaimedRooms } from "~/components/active/unclaimed-rooms";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import { useSWRAuth } from "~/lib/swr";
import { combineTimeNotes, getStudentAbsence } from "~/lib/student-time-status";
import { getDayPlanningNotComingLabel } from "~/lib/day-planning-helper";
import {
  ActiveSupervisionLoadingView,
  EmptyRoomsView,
  NoActiveSupervisionAccessView,
  ReleaseSupervisionModal,
  SchulhofNotSupervisingView,
} from "~/components/active-supervisions/states";
import { PastBlocksSection } from "~/components/active-supervisions/past-blocks-section";
import { PlannedNowSection } from "~/components/active-supervisions/planned-now-section";
import {
  SpontaneousActivityStart,
  type SpontaneousActivityStartPayload,
} from "~/components/active-supervisions/spontaneous-activity-start";
import { TransitStudentsSection } from "~/components/rooms/transit-students-section";
import {
  SCHULHOF_ROOM_NAME,
  SCHULHOF_TAB_ID,
  activeSupervisionRosterKey,
  buildGroupNameToIdMap,
  mapSupervisedGroupsToRooms,
  mapVisitsToSupervisionStudents,
  resolveSupervisionSelection,
  roomsOutsideSchulhofStatus,
  supervisionTabLabel,
  withActiveSupervisionPresence,
} from "~/components/active-supervisions/view-model";
import type {
  ActiveSupervisionRoom,
  SupervisionSessionInfo,
  ActiveSupervisionStudent,
  MinimalActiveGroup,
  SchulhofStatusResponse,
} from "~/components/active-supervisions/view-model";

const logger = createLogger({ component: "ActiveSupervisionsPage" });

async function runOwnAttendanceMutation<T>(
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

type ActiveRoom = ActiveSupervisionRoom;
type StudentWithVisit = ActiveSupervisionStudent;

function padClockPart(value: number): string {
  return value.toString().padStart(2, "0");
}

function formatLocalDate(date: Date): string {
  return [
    date.getFullYear(),
    padClockPart(date.getMonth() + 1),
    padClockPart(date.getDate()),
  ].join("-");
}

function formatClock(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `${padClockPart(hours)}:${padClockPart(minutes)}`;
}

const REOPEN_STORAGE_KEY = "timetable-reopenable-instance";
const REOPEN_WINDOW_MS = 5 * 60 * 1000;

function readStoredReopenBanner(): {
  instanceId: string;
  expiresAt: number;
} | null {
  const raw = window.sessionStorage.getItem(REOPEN_STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as {
      instanceId?: string;
      expiresAt?: string | number;
    };
    if (!parsed.instanceId || parsed.expiresAt == null) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      return null;
    }
    const expiresAt =
      typeof parsed.expiresAt === "number"
        ? parsed.expiresAt
        : Date.parse(parsed.expiresAt);
    if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      return null;
    }
    return { instanceId: parsed.instanceId, expiresAt };
  } catch {
    window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
    return null;
  }
}

function writeStoredReopenBanner(instanceId: string, expiresAt: number): void {
  window.sessionStorage.setItem(
    REOPEN_STORAGE_KEY,
    JSON.stringify({ instanceId, expiresAt }),
  );
}

function clearStoredReopenBanner(): void {
  window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
}

export function spontaneousActivityWindow(now: Date): {
  date: string;
  startTime: string;
  endTime: string;
} {
  const currentMinutes = now.getHours() * 60 + now.getMinutes();
  const startMinutes = Math.min(currentMinutes, 23 * 60 + 30);
  const endMinutes = Math.min(startMinutes + 60, 23 * 60 + 59);
  return {
    date: formatLocalDate(now),
    startTime: formatClock(startMinutes),
    endTime: formatClock(endMinutes),
  };
}

// BFF response type for consolidated dashboard data
interface BFFDashboardResponse {
  supervisedGroups: Array<{
    id: string;
    name: string;
    room_id?: string;
    room?: { id: string; name: string; color?: string | null };
  }>;
  unclaimedGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  currentStaff: { id: string } | null;
  educationalGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  firstRoomVisits: Array<{
    studentId: string;
    studentName: string;
    schoolClass: string;
    groupName: string;
    activeGroupId: string;
    checkInTime: string;
    actualArrivalTime?: string;
    actualPickupTime?: string;
    isActive: boolean;
    sick?: boolean;
    sickSince?: string;
    excused?: boolean;
    excusedSince?: string;
    class_trip?: boolean;
    class_trip_since?: string;
    // Authenticated proxy URL — backend rewrites the raw /uploads path
    // to /api/students/{id}/photo/{filename} before sending it down.
    photoUrl?: string;
  }>;
  firstRoomId: string | null;
  schulhofStatus: SchulhofStatusResponse | null;
  capabilities?: {
    webSpontaneousActivitiesEnabled: boolean;
  };
  // Plan windows of today's running sessions for tab labels (#2265);
  // optional so a cached older BFF payload degrades to name-only labels
  activeSessions?: Array<{
    activeGroupId: string;
    instanceId: string;
    title: string;
    startTime: string;
    endTime: string;
  }>;
  plannedNow: PlannedTimetableInstance[];
  // Folded-in sections of the selected session (#2096): the aggregate
  // resolves group_id (or the first supervised session) and ships its
  // visits, tracking indicators, and pickup/arrival times in the same
  // response. Optional so a cached payload from an older backend degrades
  // gracefully instead of breaking the page.
  selectedGroupId?: string | null;
  trackingIndicators?: TrackingIndicatorsResponse;
  pickupTimes?: Array<{
    studentId: string;
    date: string;
    weekdayName: string;
    pickupTime: string | null;
    isException: boolean;
    notes: string;
    dayNotes: Array<{ id: string; content: string }>;
  }>;
  arrivalTimes?: Array<{
    studentId: string;
    date: string;
    weekdayName: string;
    expectedArrival: string | null;
    isException: boolean;
    notes: string;
    dayNotes: Array<{ id: string; content: string }>;
  }>;
}

/** Check if a student matches the current search, group, and year filters */
function matchesStudentFilters(
  student: StudentWithVisit,
  searchTerm: string,
  groupFilter: string,
  yearFilter: string,
): boolean {
  if (searchTerm) {
    const searchLower = searchTerm.toLowerCase();
    const matchesSearch =
      (student.name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
      (student.second_name?.toLowerCase().includes(searchLower) ?? false);
    if (!matchesSearch) return false;
  }
  if (groupFilter !== "all") {
    const studentGroupName = student.group_name ?? "Unbekannt";
    if (studentGroupName !== groupFilter) return false;
  }
  if (yearFilter !== "all") {
    const studentYear = getSchoolYear(student.school_class);
    if (studentYear !== yearFilter) return false;
  }
  return true;
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
function moveNoticeFromRoster(
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

type RosterAction =
  "check-in" | "check-out" | "excused" | "absent" | "expected";

async function runRosterActionRequest(
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
        <button
          type="button"
          onClick={() => runAction("check-in")}
          className="bg-moto-green rounded-md px-3 py-2 text-sm font-medium text-gray-950"
        >
          Einchecken
        </button>
      ) : null}
      {!row.currentlyPresent && row.status !== "expected" ? (
        <button
          type="button"
          onClick={() => runAction("check-in")}
          className="bg-moto-green rounded-md px-3 py-2 text-sm font-medium text-gray-950"
        >
          Wieder einchecken
        </button>
      ) : null}
      {row.currentlyPresent ? (
        <button
          type="button"
          onClick={() => runAction("check-out")}
          className="rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700"
        >
          Raum verlassen
        </button>
      ) : null}
      {row.planned &&
      row.status === "expected" &&
      isCareDayExpected(row.careDayStatus) ? (
        <>
          <button
            type="button"
            onClick={() => runAction("excused")}
            className="border-moto-purple text-moto-purple-strong rounded-md border px-3 py-2 text-sm font-medium"
          >
            Entschuldigt
          </button>
          <button
            type="button"
            onClick={() => runAction("absent")}
            className="rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700"
          >
            Abwesend
          </button>
        </>
      ) : null}
      {row.planned && !row.currentlyPresent && row.status === "absent" ? (
        <button
          type="button"
          onClick={() => runAction("expected")}
          className="rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700"
        >
          Zurück auf erwartet
        </button>
      ) : null}
    </div>
  );
}

interface TimetableRosterRowProps {
  readonly attendanceWebEnabled: boolean;
  readonly instanceIsSpontaneous: boolean;
  readonly row: TimetableRosterRow;
  readonly onAction: RosterRowActionsProps["onAction"];
}

function TimetableRosterStudentRow({
  attendanceWebEnabled,
  instanceIsSpontaneous,
  row,
  onAction,
}: TimetableRosterRowProps) {
  const attendanceDetail = [
    row.substatus ? ATTENDANCE_SUBSTATUS_LABELS[row.substatus] : null,
    row.note,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <div className="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <div className="font-medium text-gray-900">
          {row.studentName || `Kind ${row.studentId}`}
        </div>
        <div className="mt-1 text-sm text-gray-500">
          {rosterStudentMeta(row, instanceIsSpontaneous)}
        </div>
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
  readonly instanceIsSpontaneous: boolean;
  readonly onAction: RosterRowActionsProps["onAction"];
  readonly rows: TimetableRosterRow[];
  readonly showTimetableCounts: boolean;
  readonly title: string;
}

function TimetableRosterSection({
  attendanceWebEnabled,
  instanceIsSpontaneous,
  onAction,
  rows,
  showTimetableCounts,
  title,
}: TimetableRosterSectionProps) {
  if (rows.length === 0) return null;
  const countLabel = showTimetableCounts ? ` (${rows.length})` : "";

  return (
    <section className="moto-content-surface overflow-hidden rounded-lg border">
      <div className="border-b border-gray-100 bg-gray-50 px-4 py-2 text-sm font-semibold text-gray-700">
        {title}
        {countLabel}
      </div>
      {rows.map((row) => (
        <TimetableRosterStudentRow
          key={`${row.studentId}-${row.status}-${row.visitId ?? "planned"}`}
          attendanceWebEnabled={attendanceWebEnabled}
          instanceIsSpontaneous={instanceIsSpontaneous}
          row={row}
          onAction={onAction}
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
  readonly summary: {
    readonly absent: number;
    readonly departed: number;
    readonly expected: number;
    readonly present: number;
    readonly unplanned: number;
  };
  readonly onComplete: () => Promise<void>;
  readonly onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void>;
}

function TimetableRosterHeader({
  attendanceWebEnabled,
  confirmableExpectedRows,
  isCompletingInstance,
  isConfirmingExpected,
  roster,
  showTimetableCounts,
  summary,
  onComplete,
  onConfirmExpected,
}: TimetableRosterHeaderProps) {
  const now = useMinuteClock();
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
            <p className="text-moto-green-strong text-xs font-semibold tracking-wide uppercase">
              Aktiv
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
            <button
              type="button"
              disabled={
                isConfirmingExpected || confirmableExpectedRows.length === 0
              }
              onClick={handleConfirmExpectedClick}
              className="bg-moto-green hover:bg-moto-green-hover focus-visible:ring-moto-green/30 inline-flex h-9 items-center justify-center gap-2 rounded-lg px-3 text-sm font-medium text-gray-950 shadow-sm transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              <MotoConceptIcon concept="present" size={16} />
              {confirmLabel}
            </button>
          ) : null}
          {attendanceWebEnabled ? (
            <button
              type="button"
              disabled={isCompletingInstance || !completeEnabled}
              onClick={handleCompleteClick}
              className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {completeEnabled
                ? "Beenden"
                : `Beenden ab ${roster.instance.endTime}`}
            </button>
          ) : null}
        </div>
      </div>
      {showTimetableCounts ? (
        <div className="grid grid-cols-2 gap-2 p-4 sm:grid-cols-5">
          <RosterSummaryStat label="Anwesend" value={summary.present} />
          <RosterSummaryStat label="Erwartet" value={summary.expected} />
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
        <input
          type="search"
          name="unplanned-student-search"
          aria-label="Kind ungeplant suchen"
          value={search}
          onChange={(event) => {
            setSelectedId(null);
            onSearchChange(event.target.value);
          }}
          placeholder="Weiteres Kind suchen..."
          className="focus:border-moto-green focus:ring-moto-green/20 min-h-10 flex-1 rounded-lg border border-gray-300 px-3 text-sm focus:ring-2 focus:outline-none"
        />
        <button
          type="submit"
          disabled={isAddingStudent || !targetStudent}
          className="bg-moto-green hover:bg-moto-green-hover rounded-lg px-4 py-2 text-sm font-medium text-gray-950 shadow-sm transition-colors disabled:opacity-50"
        >
          Hinzufügen
        </button>
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
  readonly onAddStudent: (studentId: string) => Promise<boolean>;
  readonly onComplete: () => Promise<void>;
  readonly onConfirmExpected: (rows: TimetableRosterRow[]) => Promise<void>;
  readonly onRosterAction: RosterRowActionsProps["onAction"];
  readonly onSearchChange: (value: string) => void;
}

function TimetableRosterContent({
  addStudentResults,
  addStudentSearch,
  attendanceWebEnabled,
  isAddingStudent,
  isCompletingInstance,
  isConfirmingExpected,
  roster,
  showTimetableCounts,
  onAddStudent,
  onComplete,
  onConfirmExpected,
  onRosterAction,
  onSearchChange,
}: TimetableRosterContentProps) {
  const present = roster.rows.filter(
    (row) => row.currentlyPresent && row.planned,
  );
  // The care plan decides who counts as expected (#1747): rows the plan does
  // not place here today — not booked, or the day was cancelled — go into
  // their own section below, never into "Erwartet" and never into the bulk
  // confirm, which would persist attendance for a child who is not coming.
  const expected = roster.rows.filter(
    (row) =>
      row.planned &&
      !row.currentlyPresent &&
      row.status === "expected" &&
      isCareDayExpected(row.careDayStatus),
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
    onAction: onRosterAction,
    showTimetableCounts,
  };

  return (
    <div className="space-y-4">
      <TimetableRosterHeader
        attendanceWebEnabled={attendanceWebEnabled}
        confirmableExpectedRows={confirmableExpectedRows}
        isCompletingInstance={isCompletingInstance}
        isConfirmingExpected={isConfirmingExpected}
        roster={roster}
        showTimetableCounts={showTimetableCounts}
        summary={{
          absent: absent.length,
          departed: departed.length,
          expected: expected.length,
          present: present.length,
          unplanned: unplanned.length,
        }}
        onComplete={onComplete}
        onConfirmExpected={onConfirmExpected}
      />
      {attendanceWebEnabled ? (
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

function MeinRaumPageContent() {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const showTimetableCounts = useShowTimetableCounts();
  const router = useTenantRouter();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  // Check if user has access to active rooms
  const [hasAccess, setHasAccess] = useState<boolean | null>(null);

  // State variables for multiple rooms
  const [allRooms, setAllRooms] = useState<ActiveRoom[]>([]);
  const [selectedRoomId, setSelectedRoomId] = useState<string | null>(null);

  // Pre-select the session from the URL. `?session=<activeGroupId>` is the
  // precise key (parallel sessions can share one room, #2265); the legacy
  // `?room=<roomId>` entry point (sidebar, old links) still resolves but can
  // never switch between sessions inside the same room.
  const sessionParam = searchParams.get("session");
  const roomParam = searchParams.get("room");
  const [students, setStudents] = useState<StudentWithVisit[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [groupFilter, setGroupFilter] = useState("all");
  const [selectedYear, setSelectedYear] = useState("all");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [plannedNow, setPlannedNow] = useState<PlannedTimetableInstance[]>([]);
  const [selectedTimetableInstanceId, setSelectedTimetableInstanceId] =
    useState<string | null>(null);
  const [missingRosterActiveGroupIds, setMissingRosterActiveGroupIds] =
    useState<Set<string>>(() => new Set());
  const [isStartingInstance, setIsStartingInstance] = useState<string | null>(
    null,
  );
  const [isStartingSpontaneous, setIsStartingSpontaneous] = useState(false);
  const [isCompletingInstance, setIsCompletingInstance] = useState(false);
  const [showCompleteConfirmation, setShowCompleteConfirmation] =
    useState(false);
  const [storedReopen, setStoredReopen] = useState<{
    instanceId: string;
    expiresAt: number;
  } | null>(null);
  const reopenableInstanceId = storedReopen?.instanceId ?? null;

  useEffect(() => {
    setStoredReopen(readStoredReopenBanner());
  }, []);

  useEffect(() => {
    if (!storedReopen) return;
    const remainingMs = storedReopen.expiresAt - Date.now();
    if (remainingMs <= 0) {
      clearStoredReopenBanner();
      setStoredReopen(null);
      return;
    }
    const timeoutId = window.setTimeout(() => {
      clearStoredReopenBanner();
      setStoredReopen(null);
    }, remainingMs);
    return () => window.clearTimeout(timeoutId);
  }, [storedReopen]);
  const [isConfirmingExpected, setIsConfirmingExpected] = useState(false);
  const [addStudentSearch, setAddStudentSearch] = useState("");
  const [addStudentResult, setAddStudentResult] = useState<{
    readonly instanceId: string;
    readonly students: Student[];
  } | null>(null);
  const [isAddingStudent, setIsAddingStudent] = useState(false);
  // Info notice after a check-in auto-moved the child out of another running
  // session (#2386). Cleared by the next roster action.
  const [moveNotice, setMoveNotice] = useState<string | null>(null);

  // OGS group rooms for color detection
  const [myGroupRooms, setMyGroupRooms] = useState<string[]>([]);

  // OGS group IDs for permission checking
  const [myGroupIds, setMyGroupIds] = useState<string[]>([]);

  // Map from group name to group ID for enriching visit data
  const [groupNameToIdMap, setGroupNameToIdMap] = useState<Map<string, string>>(
    new Map(),
  );

  // Desktop detection — sidebar handles room switching at lg+
  const [isDesktop, setIsDesktop] = useState(false);
  useEffect(() => {
    const check = () => setIsDesktop(window.innerWidth >= 1024);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  // State for Schulhof release supervision modal
  const [showReleaseModal, setShowReleaseModal] = useState(false);
  const [isReleasingSupervision, setIsReleasingSupervision] = useState(false);

  // Schulhof permanent tab state
  const [schulhofStatus, setSchulhofStatus] =
    useState<SchulhofStatusResponse | null>(null);
  const [isTogglingSchulhof, setIsTogglingSchulhof] = useState(false);
  const [isSchulhofTabSelected, setIsSchulhofTabSelected] = useState(false);

  // Ref to always have latest schulhofStatus (prevents stale closure in callbacks)
  const schulhofStatusRef = useLatest(schulhofStatus);

  // Cached active groups for UnclaimedRooms (avoids duplicate API call)
  const [cachedActiveGroups, setCachedActiveGroups] = useState<
    MinimalActiveGroup[]
  >([]);
  const [currentStaffId, setCurrentStaffId] = useState<string | undefined>();

  // Get current selected room (null if Schulhof tab is selected but user isn't supervising)
  // Wrapped in useMemo to prevent dependency changes on every render
  const currentRoom = useMemo(
    () =>
      isSchulhofTabSelected
        ? schulhofStatus?.isUserSupervising && schulhofStatus?.activeGroupId
          ? {
              id: schulhofStatus.activeGroupId,
              name: SCHULHOF_ROOM_NAME,
              room_name: SCHULHOF_ROOM_NAME,
              room_id: schulhofStatus.roomId ?? undefined,
              student_count: schulhofStatus.studentCount,
            }
          : null
        : (allRooms.find((r) => r.id === selectedRoomId) ??
          allRooms[0] ??
          null),
    [
      isSchulhofTabSelected,
      schulhofStatus?.isUserSupervising,
      schulhofStatus?.activeGroupId,
      schulhofStatus?.roomId,
      schulhofStatus?.studentCount,
      allRooms,
      selectedRoomId,
    ],
  );

  const occupiedRoomIds = useMemo(() => {
    const ids = allRooms
      .map((room) => room.room_id)
      .filter((roomId): roomId is string => Boolean(roomId));
    // Schulhof is tracked separately from allRooms, so keep its live room id
    // in the occupancy set. The spontaneous modal deliberately treats that
    // one destination as navigation to the dedicated supervision instead of
    // disabling it; every normal occupied room remains unavailable to start.
    if (schulhofStatus?.activeGroupId && schulhofStatus.roomId)
      ids.push(schulhofStatus.roomId);
    return ids;
  }, [allRooms, schulhofStatus?.activeGroupId, schulhofStatus?.roomId]);

  // Set breadcrumb so the header names the session (not just the room —
  // parallel sessions can share one room, #2265)
  useSetBreadcrumb({
    activeSupervisionName: isSchulhofTabSelected
      ? SCHULHOF_ROOM_NAME
      : (currentRoom?.name ?? currentRoom?.room_name),
  });

  const currentRoomRef = useRef<ActiveRoom | null>(null);
  const hasSupervisionRef = useRef(false);
  const groupNameToIdMapRef = useRef<Map<string, string>>(new Map());

  useEffect(() => {
    currentRoomRef.current = currentRoom;
  }, [currentRoom]);

  useEffect(() => {
    groupNameToIdMapRef.current = groupNameToIdMap;
  }, [groupNameToIdMap]);

  // Helper to update room student count - extracted to reduce nesting depth
  const updateRoomStudentCount = useCallback(
    (roomId: string, studentCount: number) => {
      setAllRooms((prev) =>
        prev.map((room) =>
          room.id === roomId ? { ...room, student_count: studentCount } : room,
        ),
      );
    },
    [],
  );

  // SSE is handled globally by TenantAuthWrapper - no page-level setup needed.
  // When relevant events occur, global SSE invalidates the aggregated
  // "active-supervision-dashboard-" cache, which triggers the SWR refetch.
  // NOTE: Do NOT call useGlobalSSE() here - it's already called in TenantAuthWrapper.
  // Calling it again would create a duplicate SSE connection.
  //
  // There is deliberately NO client-side fallback fan-out when the aggregate
  // fails: silently-empty partial payloads are the failure mode #2096
  // removed. Errors surface via dashboardError instead.

  // Get current room ID (the selected session's active group id)
  const currentRoomId = currentRoom?.id;

  // The aggregate is parameterized by the selected session (#2096). The SWR
  // key stays `active-supervision-dashboard-${refreshKey}` — the global SSE
  // invalidation contract and the page tests match on that prefix — so the
  // fetcher reads the selection from a ref; room switches re-run it via
  // mutateDashboard() instead of spawning per-room cache keys.
  const requestedGroupIdRef = useRef<string | null>(null);
  const now = useMinuteClock();
  const dashboardDay = berlinTodayISO(now);
  const previousDashboardDayRef = useRef(dashboardDay);
  const lastDashboardReconciliationRef = useRef<string | null>(null);

  // SWR-based aggregate fetching with caching
  // Cache key "active-supervision-dashboard" will be invalidated by global SSE on relevant events
  const {
    data: dashboardData,
    isLoading: isDashboardLoading,
    error: dashboardError,
    mutate: mutateDashboard,
  } = useSWRAuth<BFFDashboardResponse>(
    session?.user?.token ? `active-supervision-dashboard-${refreshKey}` : null,
    async () => {
      logger.debug("SWR fetching dashboard data");
      const start = performance.now();

      const fetchDashboard = async (groupId: string | null) => {
        const query =
          groupId && /^[1-9]\d{0,18}$/.test(groupId)
            ? `?group_id=${encodeURIComponent(groupId)}`
            : "";
        return fetch(`/api/active-supervision-dashboard${query}`, {
          headers: {
            Authorization: `Bearer ${session?.user?.token}`,
            "Content-Type": "application/json",
          },
        });
      };

      const requestedGroupId = requestedGroupIdRef.current;
      let response = await fetchDashboard(requestedGroupId);
      // A stale selection (supervision revoked, session ended) is a backend
      // 403 — retry once without group_id so the backend resolves the
      // caller's first supervised session; the sync effect then re-aligns
      // the selection from the response.
      if (!response.ok && response.status === 403 && requestedGroupId) {
        response = await fetchDashboard(null);
      }

      if (!response.ok) {
        // No silent fallback fan-out (#2096): a failed aggregate is an
        // error, never a partial-empty payload — admins included.
        throw new Error(`BFF request failed: ${response.status}`);
      }

      const bffData = (await response.json()) as {
        data: BFFDashboardResponse;
      };

      logger.debug("SWR fetch complete", {
        duration_ms: Math.round(performance.now() - start),
      });
      return bffData.data;
    },
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    },
  );

  // Pickup and arrival rows are resolved for the backend's current Berlin
  // calendar day. The SWR key deliberately stays stable for SSE invalidation,
  // so roll the aggregate forward explicitly when the minute clock crosses
  // Berlin midnight (or catches up after a backgrounded tab resumes).
  useEffect(() => {
    if (previousDashboardDayRef.current === dashboardDay) return;
    previousDashboardDayRef.current = dashboardDay;
    void mutateDashboard();
  }, [dashboardDay, mutateDashboard]);

  // Reconcile selection and aggregate: whenever the visible session changes
  // through any path (tab click, Schulhof open, URL/localStorage restore)
  // and the cached aggregate belongs to another session, re-run the fetch.
  useEffect(() => {
    requestedGroupIdRef.current = currentRoomId ?? null;
    if (!currentRoomId || !dashboardData) return;
    const resolved = dashboardData.selectedGroupId;
    if (resolved === currentRoomId) {
      lastDashboardReconciliationRef.current = null;
      return;
    }
    const reconciliation = `${resolved ?? "none"}:${currentRoomId}`;
    if (lastDashboardReconciliationRef.current === reconciliation) return;
    lastDashboardReconciliationRef.current = reconciliation;
    void Promise.resolve(mutateDashboard()).catch(() => {
      // Errors surface via dashboardError handling
    });
  }, [currentRoomId, dashboardData, mutateDashboard]);

  // Title + plan window per running session, so tab labels can show
  // "Aktivitätsname · Planzeit" (#2265). Sessions without a timetable
  // instance fall back to the session/room name.
  const sessionInfoByActiveGroup = useMemo(() => {
    const map = new Map<string, SupervisionSessionInfo>();
    for (const liveSession of dashboardData?.activeSessions ?? []) {
      map.set(liveSession.activeGroupId, {
        title: liveSession.title,
        timeRange: `${liveSession.startTime}–${liveSession.endTime}`,
      });
    }
    return map;
  }, [dashboardData?.activeSessions]);

  // #2161: the permanent Schulhof tab (one-tap "Beaufsichtigen") rides on the
  // generic spontaneous-start flow, so it is gated on the same capability.
  // Tenants without it see the yard as a normal room tab while a planned or
  // spontaneous session runs there. Synced into state below (like
  // schulhofStatus) so transient dashboard refetches don't drop the tab.
  const [schulhofTabEnabled, setSchulhofTabEnabled] = useState(false);
  const schulhofTabAvailable =
    schulhofTabEnabled && schulhofStatus?.exists === true;

  // Sync SWR dashboard data with local state
  useEffect(() => {
    if (!dashboardData) return;

    const data = dashboardData;
    setPlannedNow(data.plannedNow ?? []);

    // Set staff ID for UnclaimedRooms component
    if (data.currentStaff) {
      setCurrentStaffId(data.currentStaff.id);
    }

    // Set educational groups data (for OGS group permissions)
    const roomNames = data.educationalGroups
      .map((group) => group.room?.name)
      .filter((name): name is string => !!name);
    setMyGroupRooms(roomNames);

    const groupIds = data.educationalGroups.map((group) => group.id);
    setMyGroupIds(groupIds);

    const nameToIdMap = buildGroupNameToIdMap(data.educationalGroups);
    setGroupNameToIdMap(nameToIdMap);
    groupNameToIdMapRef.current = nameToIdMap;

    // Clear stale status as well: room discovery and status provisioning are
    // independent requests, so a missing status must not leave an old shortcut
    // active.
    setSchulhofStatus(dashboardError ? null : data.schulhofStatus);
    setSchulhofTabEnabled(
      data.capabilities?.webSpontaneousActivitiesEnabled === true,
    );

    // Cache active groups for UnclaimedRooms component
    if (data.supervisedGroups.length > 0) {
      const combinedGroups = [
        ...data.supervisedGroups.map((g) => ({
          id: g.id,
          room: g.room ? { name: g.room.name } : undefined,
        })),
        ...data.unclaimedGroups.map((g) => ({
          id: g.id,
          room: g.room,
        })),
      ];
      setCachedActiveGroups(combinedGroups);
    } else {
      setCachedActiveGroups([]);
    }

    // Check access
    if (
      data.supervisedGroups.length === 0 &&
      data.unclaimedGroups.length === 0
    ) {
      hasSupervisionRef.current = false;
      setHasAccess(true);
      setAllRooms([]);
      setIsLoading(false);
      return;
    }

    setHasAccess(true);

    // If no supervised groups but unclaimed groups exist
    if (data.supervisedGroups.length === 0) {
      hasSupervisionRef.current = false;
      setAllRooms([]);
      setIsLoading(false);
      return;
    }

    // Track if supervision was gained
    hasSupervisionRef.current = data.supervisedGroups.length > 0;

    const activeRooms = mapSupervisedGroupsToRooms(data.supervisedGroups);

    setAllRooms(activeRooms);

    // The aggregate carries the visits of the session the backend resolved
    // (requested group_id, or the first supervised session). Apply them only
    // when that session is the one the user is looking at — an SSE
    // revalidation while the user views another room must NOT overwrite
    // their current view.
    const firstRoom = activeRooms[0];
    // A payload without selectedGroupId (older backend) keeps the former
    // semantics: its visits belong to the first room.
    const selectedRoom = data.selectedGroupId
      ? activeRooms.find((r) => r.id === data.selectedGroupId)
      : firstRoom;

    // If the previously selected room no longer exists in the refreshed list
    // (e.g., supervision revoked, session ended), reset to the session the
    // backend resolved so the student data stays in sync with the UI.
    if (selectedRoomId && !activeRooms.some((r) => r.id === selectedRoomId)) {
      setSelectedRoomId(selectedRoom?.id ?? firstRoom?.id ?? null);
    }

    // Skip first-room preload when Schulhof tab is active — Schulhof uses
    // selectedRoomId=null intentionally, so !selectedRoomId would incorrectly
    // match and overwrite Schulhof students with first-room data.
    const isUrlTargetingDifferentRoom = sessionParam
      ? sessionParam !== SCHULHOF_TAB_ID &&
        activeRooms.some((room) => room.id === sessionParam) &&
        firstRoom?.id !== sessionParam
      : !!roomParam &&
        roomParam !== SCHULHOF_TAB_ID &&
        activeRooms.some((room) => room.room_id === roomParam) &&
        firstRoom?.room_id !== roomParam;

    if (
      !isSchulhofTabSelected &&
      !isUrlTargetingDifferentRoom &&
      selectedRoom &&
      (!selectedRoomId || selectedRoomId === selectedRoom.id)
    ) {
      // When no room is explicitly selected yet, lock in the resolved
      // session's ID so the URL-sync effect won't try to "switch" to it via
      // localStorage.
      if (!selectedRoomId) {
        setSelectedRoomId(selectedRoom.id);
      }
      const studentsFromVisits = mapVisitsToSupervisionStudents(
        data.firstRoomVisits,
        {
          roomName: selectedRoom.room_name,
          roomColor: selectedRoom.room_color,
          groupNameToId: nameToIdMap,
        },
      );
      setStudents(studentsFromVisits);
      updateRoomStudentCount(selectedRoom.id, studentsFromVisits.length);
    }

    // Schulhof tab: the aggregate resolves the Schulhof session when it was
    // requested (user supervising the yard) — apply its visits under the
    // Schulhof heading.
    if (
      isSchulhofTabSelected &&
      data.selectedGroupId &&
      data.schulhofStatus?.isUserSupervising &&
      data.schulhofStatus.activeGroupId === data.selectedGroupId
    ) {
      setStudents(
        mapVisitsToSupervisionStudents(data.firstRoomVisits, {
          roomName: SCHULHOF_ROOM_NAME,
          groupNameToId: nameToIdMap,
        }),
      );
    }

    setError(null);
    setIsLoading(false);
  }, [
    dashboardData,
    dashboardError,
    updateRoomStudentCount,
    selectedRoomId,
    isSchulhofTabSelected,
    sessionParam,
    roomParam,
  ]);

  // Sync the selected session with the URL / localStorage. The resolution
  // order (session param > legacy room param > saved session > saved room)
  // lives in resolveSupervisionSelection; this effect only executes the
  // target it returns. A "none" target keeps the current selection — the
  // resolver never switches between parallel sessions in the same room
  // just because a refresh re-resolved a room-keyed URL (#2265).
  useEffect(() => {
    const selectSchulhof = () => {
      if (isSchulhofTabSelected) return;
      setIsSchulhofTabSelected(true);
      setSelectedRoomId(null);
      // When supervising, the selection-reconciliation effect re-runs the
      // aggregate for the yard session and the sync effect applies its
      // visits; otherwise there is nothing to show.
      if (!(
        schulhofStatus?.isUserSupervising && schulhofStatus.activeGroupId
      )) {
        setStudents([]);
      }
    };

    const target = resolveSupervisionSelection({
      sessionParam,
      roomParam,
      savedSessionId: localStorage.getItem("supervision-last-session"),
      savedRoomId: localStorage.getItem("sidebar-last-room"),
      rooms: allRooms,
      currentSessionId: selectedRoomId,
      schulhofAvailable: schulhofTabAvailable,
    });

    if (target.kind === "schulhof") {
      selectSchulhof();
      return;
    }
    if (allRooms.length === 0) return;
    if (target.kind === "session") {
      if (isSchulhofTabSelected) {
        setIsSchulhofTabSelected(false);
      }
      localStorage.setItem("supervision-last-session", target.sessionId);
      void switchToRoom(target.sessionId);
      return;
    }
    if (target.kind === "persist-first") {
      // Nothing saved (e.g. fresh login) — persist the default so the next
      // sidebar click and reload land on the same session.
      const firstRoom = allRooms[0];
      if (firstRoom) {
        localStorage.setItem("supervision-last-session", firstRoom.id);
        if (firstRoom.room_id) {
          localStorage.setItem("sidebar-last-room", firstRoom.room_id);
        }
      }
    }
    // "none": already in sync
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    allRooms,
    sessionParam,
    roomParam,
    schulhofTabAvailable,
    schulhofStatus?.activeGroupId,
    schulhofStatus?.isUserSupervising,
  ]);

  // Per-room visits ride in the aggregate (#2096): SSE invalidates the
  // dashboard key, the fetcher re-requests the selected session, and the
  // sync effect above applies its visits — no separate per-room fetch.

  const timetableRosterKey = activeSupervisionRosterKey({
    selectedTimetableInstanceId,
    currentRoomId,
    missingRosterActiveGroupIds,
  });
  const {
    data: timetableRoster,
    isLoading: isTimetableRosterLoading,
    mutate: mutateRoster,
  } = useSWRAuth<TimetableRoster | null>(
    timetableRosterKey,
    async () => {
      try {
        if (selectedTimetableInstanceId) {
          return await timetableOperationsApi.roster(
            selectedTimetableInstanceId,
          );
        }
        if (!currentRoomId) return null;
        return await timetableOperationsApi.rosterByActiveGroup(currentRoomId);
      } catch (err) {
        if (
          err instanceof Error &&
          (err.message.includes("404") ||
            err.message.toLowerCase().includes("not found"))
        ) {
          if (!selectedTimetableInstanceId && currentRoomId) {
            setMissingRosterActiveGroupIds((current) => {
              if (current.has(currentRoomId)) return current;
              const next = new Set(current);
              next.add(currentRoomId);
              return next;
            });
          }
          return null;
        }
        throw err;
      }
    },
    { keepPreviousData: false, revalidateOnFocus: false },
  );
  const timetableRosterMatchesSelection =
    timetableRoster !== undefined &&
    timetableRoster !== null &&
    (selectedTimetableInstanceId
      ? timetableRoster.instance.id === selectedTimetableInstanceId
      : !!currentRoomId &&
        timetableRoster.instance.activeGroupId === currentRoomId);
  const currentTimetableRoster = timetableRosterMatchesSelection
    ? timetableRoster
    : null;
  const activeTimetableInstanceId =
    currentTimetableRoster?.instance?.id ?? null;
  const activeTimetableInstanceIdRef = useLatest(activeTimetableInstanceId);
  const addStudentResults =
    addStudentResult?.instanceId === activeTimetableInstanceId
      ? addStudentResult.students
      : [];
  const isWaitingForTimetableRoster =
    timetableRosterKey !== null &&
    (timetableRoster === undefined ||
      (timetableRoster !== null && !timetableRosterMatchesSelection)) &&
    isTimetableRosterLoading;

  // The move notice belongs to the session it happened in — drop it when the
  // supervisor switches to another session tab.
  useEffect(() => {
    setMoveNotice(null);
  }, [activeTimetableInstanceId]);

  useEffect(() => {
    if (!activeTimetableInstanceId || addStudentSearch.trim().length < 2) {
      setAddStudentResult(null);
      return;
    }

    setAddStudentResult(null);
    let cancelled = false;
    const timeout = window.setTimeout(() => {
      fetchStudents({
        search: addStudentSearch.trim(),
        page: 1,
        page_size: 5,
      })
        .then((result) => {
          if (!cancelled) {
            setAddStudentResult({
              instanceId: activeTimetableInstanceId,
              students: result.students,
            });
          }
        })
        .catch((err) => {
          if (cancelled) return;
          logger.warn("failed to search students for timetable roster", {
            error: err instanceof Error ? err.message : String(err),
          });
          setAddStudentResult(null);
        });
    }, 250);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [activeTimetableInstanceId, addStudentSearch]);

  // Tracking indicators, pickup times, and arrival times ride in the
  // aggregate for the selected session (#2096) — no separate fetches, no
  // extra tenant transactions. SSE invalidation of the dashboard key keeps
  // them fresh together with the visits they belong to.
  const trackingData = dashboardData?.trackingIndicators;

  const pickupTimesData = useMemo(() => {
    if (!dashboardData?.pickupTimes) return undefined;
    const map = new Map<string, BulkPickupTime>();
    for (const pickup of dashboardData.pickupTimes) {
      map.set(pickup.studentId, {
        studentId: pickup.studentId,
        date: pickup.date,
        weekdayName: pickup.weekdayName,
        pickupTime: pickup.pickupTime ?? undefined,
        isException: pickup.isException,
        notes: pickup.notes || undefined,
        dayNotes: pickup.dayNotes,
      });
    }
    return map;
  }, [dashboardData?.pickupTimes]);

  const arrivalTimesData = useMemo(() => {
    if (!dashboardData?.arrivalTimes) return undefined;
    const map = new Map<string, BulkArrivalTime>();
    for (const arrival of dashboardData.arrivalTimes) {
      map.set(arrival.studentId, {
        studentId: arrival.studentId,
        date: arrival.date,
        weekdayName: arrival.weekdayName,
        expectedArrival: arrival.expectedArrival ?? undefined,
        isException: arrival.isException,
        notes: arrival.notes || undefined,
        dayNotes: arrival.dayNotes,
      });
    }
    return map;
  }, [dashboardData?.arrivalTimes]);

  // Handle dashboard error
  useEffect(() => {
    if (dashboardError) {
      // Fail closed for the dedicated Schulhof workflow while SWR may still
      // expose the previous dashboard data during a failed revalidation.
      setSchulhofStatus(null);
      if (dashboardError.message.includes("403")) {
        setError("Sie haben aktuell keinen aktiven Raum zur Supervision.");
        setHasAccess(false);
      } else {
        setError("Fehler beim Laden der Aktivitätsdaten.");
      }
      setIsLoading(false);
    }
  }, [dashboardError]);

  useEffect(() => {
    if (schulhofTabAvailable || !isSchulhofTabSelected) return;

    setIsSchulhofTabSelected(false);
    setSelectedRoomId(allRooms[0]?.id ?? null);
    setSelectedTimetableInstanceId(null);
    setStudents([]);
  }, [allRooms, isSchulhofTabSelected, schulhofTabAvailable]);

  // Derive loading state from SWR
  useEffect(() => {
    if (isDashboardLoading && !dashboardData) {
      setIsLoading(true);
    }
  }, [isDashboardLoading, dashboardData]);

  // Auto-select Schulhof tab when it's the only available option
  useEffect(() => {
    if (
      allRooms.length === 0 &&
      schulhofTabAvailable &&
      !isSchulhofTabSelected
    ) {
      setIsSchulhofTabSelected(true);
    }
  }, [allRooms.length, schulhofTabAvailable, isSchulhofTabSelected]);

  // Callback when a room is claimed - triggers refresh
  const handleRoomClaimed = useCallback(() => {
    setRefreshKey((prev) => prev + 1);
  }, []);

  const handleStartPlannedInstance = useCallback(
    async (instance: PlannedTimetableInstance) => {
      try {
        setIsStartingInstance(instance.id);
        const result = await timetableOperationsApi.start(instance.id);
        const startedRoom = allRooms.find(
          (room) => room.room_id === instance.roomId,
        );
        setSelectedTimetableInstanceId(instance.id);
        setSelectedRoomId(result.activeGroupId);
        setIsSchulhofTabSelected(false);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", instance.roomId);
        if (startedRoom?.room_name) {
          localStorage.setItem("sidebar-last-room-name", startedRoom.room_name);
        } else {
          localStorage.removeItem("sidebar-last-room-name");
        }
        await mutateDashboard();
        setRefreshKey((prev) => prev + 1);
      } catch (err) {
        logger.error("failed to start planned timetable instance", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Geplante Aktivität konnte nicht gestartet werden.");
      } finally {
        setIsStartingInstance(null);
      }
    },
    [allRooms, mutateDashboard, router],
  );

  const handleStartSpontaneousActivity = useCallback(
    async (payload: SpontaneousActivityStartPayload) => {
      if (!currentStaffId) {
        setError(
          "Aktivität konnte nicht gestartet werden: kein Betreuerprofil.",
        );
        return;
      }

      try {
        setIsStartingSpontaneous(true);
        const window = spontaneousActivityWindow(new Date());
        const staffIds = Array.from(
          new Set([currentStaffId, ...payload.additionalStaffIds]),
        )
          .map(Number)
          .filter((id) => Number.isSafeInteger(id) && id > 0);
        if (staffIds.length === 0) {
          throw new Error("current staff id is not numeric");
        }
        const result = await timetableOperationsApi.createAndStartSpontaneous({
          date: window.date,
          start_time: window.startTime,
          end_time: window.endTime,
          title: payload.title,
          room_id: Number(payload.roomId),
          activity_group_id: payload.activityGroupId
            ? Number(payload.activityGroupId)
            : undefined,
          staff_ids: staffIds,
          student_ids: [],
        });
        setSelectedTimetableInstanceId(result.instanceId);
        setSelectedRoomId(result.activeGroupId);
        setIsSchulhofTabSelected(false);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
        localStorage.setItem("supervision-last-session", result.activeGroupId);
        localStorage.setItem("sidebar-last-room", payload.roomId);
        await mutateDashboard();
        setRefreshKey((prev) => prev + 1);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        const isRoomOccupied = message.includes("room is already occupied");
        const context = {
          title: payload.title,
          room_id: payload.roomId,
          error: message,
        };
        if (isRoomOccupied) {
          logger.warn("spontaneous timetable room already occupied", context);
        } else {
          logger.error(
            "failed to start spontaneous timetable instance",
            context,
          );
        }
        setError(
          isRoomOccupied
            ? "Der Raum ist bereits belegt."
            : "Spontane Aktivität konnte nicht gestartet werden.",
        );
      } finally {
        setIsStartingSpontaneous(false);
      }
    },
    [currentStaffId, mutateDashboard, router],
  );

  const handleOpenSchulhofSupervision = useCallback(() => {
    if (!schulhofStatusRef.current?.exists) {
      setError(
        "Die Schulhof-Aufsicht ist gerade nicht verfügbar. Bitte laden Sie die Seite neu.",
      );
      return;
    }
    setIsSchulhofTabSelected(true);
    setSelectedRoomId(null);
    setSelectedTimetableInstanceId(null);
    router.push(`/active-supervisions?session=${SCHULHOF_TAB_ID}`);
    localStorage.setItem("supervision-last-session", SCHULHOF_TAB_ID);
    localStorage.setItem("sidebar-last-room", SCHULHOF_TAB_ID);
    localStorage.setItem("sidebar-last-room-name", SCHULHOF_ROOM_NAME);
    // The selection-reconciliation effect re-runs the aggregate for the
    // Schulhof session and is the single owner of loading its visits. A
    // second manual request can land later and overwrite a fresher SSE
    // revalidation.
    setStudents([]);
  }, [router, schulhofStatusRef]);

  const handleRosterAction = useCallback(
    async (action: RosterAction, row: TimetableRosterRow) => {
      if (!activeTimetableInstanceId) return;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      let roster: TimetableRoster | null;
      try {
        roster = await runRosterActionRequest(
          action,
          instanceId,
          row.studentId,
        );
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        logger.error("failed timetable roster action", {
          action,
          student_id: row.studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Aktion im Betreuungsplan konnte nicht ausgeführt werden.");
        return;
      }
      if (activeTimetableInstanceIdRef.current !== instanceId) return;
      try {
        if (action === "check-in" && roster) {
          setMoveNotice(moveNoticeFromRoster(roster, row.studentId));
        }
        await (roster
          ? mutateRoster(roster, { revalidate: false })
          : mutateRoster());
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        logger.warn("timetable_roster_sync_failed_after_successful_action", {
          action,
          student_id: row.studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        void logger.flush();
        window.location.reload();
      }
    },
    [activeTimetableInstanceId, activeTimetableInstanceIdRef, mutateRoster],
  );

  const confirmCompleteTimetableInstance = useCallback(async () => {
    if (!activeTimetableInstanceId) return;
    try {
      setIsCompletingInstance(true);
      const completed = await timetableOperationsApi.complete(
        activeTimetableInstanceId,
        currentTimetableRoster?.rows
          .filter((row) => row.currentlyPresent)
          .map((row) => row.studentId) ?? [],
      );
      const expiresAt = completed.reopenUntil
        ? Date.parse(completed.reopenUntil)
        : Date.now() + REOPEN_WINDOW_MS;
      if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
        clearStoredReopenBanner();
        setStoredReopen(null);
      } else {
        writeStoredReopenBanner(activeTimetableInstanceId, expiresAt);
        setStoredReopen({
          instanceId: activeTimetableInstanceId,
          expiresAt,
        });
      }
      setShowCompleteConfirmation(false);
      setSelectedTimetableInstanceId(null);
      await mutateDashboard();
      setRefreshKey((prev) => prev + 1);
    } catch (err) {
      logger.error("failed to complete timetable instance", {
        instance_id: activeTimetableInstanceId,
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Aktivität konnte nicht beendet werden.");
    } finally {
      setIsCompletingInstance(false);
    }
  }, [activeTimetableInstanceId, currentTimetableRoster, mutateDashboard]);

  const handleCompleteTimetableInstance = useCallback(async () => {
    setShowCompleteConfirmation(true);
  }, []);

  const handleReopenTimetableInstance = useCallback(async () => {
    if (!reopenableInstanceId) return;
    try {
      const result = await timetableOperationsApi.reopen(reopenableInstanceId);
      clearStoredReopenBanner();
      setStoredReopen(null);
      setSelectedTimetableInstanceId(result.instanceId);
      await mutateDashboard();
      setRefreshKey((previous) => previous + 1);
    } catch (err) {
      if (isReopenUnavailableError(err)) {
        clearStoredReopenBanner();
        setStoredReopen(null);
      }
      setError(
        err instanceof Error
          ? err.message
          : "Aktivität konnte nicht wieder geöffnet werden.",
      );
    }
  }, [mutateDashboard, reopenableInstanceId]);

  const handleConfirmExpectedStudents = useCallback(
    async (rows: TimetableRosterRow[]) => {
      if (!activeTimetableInstanceId || rows.length === 0) return;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      try {
        setIsConfirmingExpected(true);
        let nextRoster: TimetableRoster | null = null;
        const notices: string[] = [];
        for (const row of rows) {
          nextRoster = await runOwnAttendanceMutation(
            "student_checkin",
            row.studentId,
            () => timetableOperationsApi.checkIn(instanceId, row.studentId),
          );
          if (activeTimetableInstanceIdRef.current !== instanceId) continue;
          const notice = moveNoticeFromRoster(nextRoster, row.studentId);
          if (notice) notices.push(notice);
        }
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        if (notices.length > 0) setMoveNotice(notices.join(" "));
        if (nextRoster) {
          await mutateRoster(nextRoster, { revalidate: false });
        } else {
          await mutateRoster();
        }
        await mutateDashboard();
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return;
        logger.error("failed to confirm expected timetable students", {
          instance_id: instanceId,
          count: rows.length,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Erwartete Kinder konnten nicht bestätigt werden.");
      } finally {
        setIsConfirmingExpected(false);
      }
    },
    [
      activeTimetableInstanceId,
      activeTimetableInstanceIdRef,
      mutateDashboard,
      mutateRoster,
    ],
  );

  const handleAddUnplannedStudent = useCallback(
    async (studentId: string) => {
      if (!activeTimetableInstanceId) return false;
      const instanceId = activeTimetableInstanceId;
      setMoveNotice(null);
      try {
        setIsAddingStudent(true);
        const roster = await runOwnAttendanceMutation(
          "student_checkin",
          studentId,
          () => timetableOperationsApi.checkIn(instanceId, studentId),
        );
        if (activeTimetableInstanceIdRef.current !== instanceId) return false;
        setMoveNotice(moveNoticeFromRoster(roster, studentId));
        setAddStudentSearch("");
        setAddStudentResult(null);
        await mutateRoster(roster, { revalidate: false });
        return true;
      } catch (err) {
        if (activeTimetableInstanceIdRef.current !== instanceId) return false;
        logger.error("failed to add unplanned timetable student", {
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Kind konnte nicht zur Aktivität hinzugefügt werden.");
        return false;
      } finally {
        setIsAddingStudent(false);
      }
    },
    [activeTimetableInstanceId, activeTimetableInstanceIdRef, mutateRoster],
  );

  // Handle releasing Schulhof supervision
  const handleReleaseSupervision = useCallback(async () => {
    if (!currentRoom || !currentStaffId) return;

    try {
      setIsReleasingSupervision(true);

      // Get all supervisors for this active group
      const supervisors = await activeService.getActiveGroupSupervisors(
        currentRoom.id,
      );

      // Find the supervisor record for the current user (using cached staff ID)
      const mySupervision = supervisors.find(
        (sup) => sup.staffId === currentStaffId && sup.isActive,
      );

      if (mySupervision) {
        await activeService.endSupervision(mySupervision.id);
      } else {
        logger.warn("no active supervision found for current user");
      }

      setShowReleaseModal(false);

      // Refresh the page to show updated state
      setRefreshKey((prev) => prev + 1);
    } catch (err) {
      logger.error("failed to release Schulhof supervision", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Fehler beim Abgeben der Schulhof-Aufsicht.");
    } finally {
      setIsReleasingSupervision(false);
    }
  }, [currentRoom, currentStaffId]);

  // Start a fresh Schulhof session via the generic spontaneous flow (#2161).
  // A "room is already occupied" conflict means another session won the race
  // between status fetch and start — join that session instead of failing.
  const startSchulhofSpontaneously = useCallback(async () => {
    const status = schulhofStatusRef.current;
    if (!status?.roomId) {
      throw new Error("Schulhof room is not provisioned");
    }
    if (!currentStaffId) {
      throw new Error("no staff profile for spontaneous Schulhof start");
    }
    const window = spontaneousActivityWindow(new Date());
    try {
      await timetableOperationsApi.createAndStartSpontaneous({
        date: window.date,
        start_time: window.startTime,
        end_time: window.endTime,
        title: SCHULHOF_ROOM_NAME,
        room_id: Number(status.roomId),
        activity_group_id: status.activityGroupId
          ? Number(status.activityGroupId)
          : undefined,
        staff_ids: [Number(currentStaffId)],
        student_ids: [],
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (!message.includes("room is already occupied")) throw err;
      const fresh = await activeService.getSchulhofStatus();
      if (!fresh.activeGroupId) throw err;
      await activeService.claimActiveGroup(fresh.activeGroupId);
    }
  }, [currentStaffId, schulhofStatusRef]);

  // Handle toggling Schulhof supervision (start/stop). Since #2161 this rides
  // on the generic mechanics: claim the open session, start a spontaneous one
  // when the yard is empty, end the own supervision to stop.
  const handleToggleSchulhof = useCallback(async () => {
    if (!schulhofStatus) return;

    try {
      setIsTogglingSchulhof(true);
      if (schulhofStatus.isUserSupervising) {
        if (!schulhofStatus.supervisionId) {
          throw new Error("no supervision id in Schulhof status");
        }
        await activeService.endSupervision(schulhofStatus.supervisionId);
      } else if (schulhofStatus.activeGroupId) {
        await activeService.claimActiveGroup(schulhofStatus.activeGroupId);
      } else {
        await startSchulhofSpontaneously();
      }

      // Refresh to get updated status
      // Note: Don't reset isTogglingSchulhof here - let the useEffect below handle it
      // when schulhofStatus actually updates, to avoid flickering
      setRefreshKey((prev) => prev + 1);
    } catch (err) {
      logger.error("failed to toggle Schulhof supervision", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        schulhofStatus.isUserSupervising
          ? "Fehler beim Abgeben der Schulhof-Aufsicht."
          : "Fehler beim Übernehmen der Schulhof-Aufsicht.",
      );
      // Only reset loading state on error - success case handled by useEffect
      setIsTogglingSchulhof(false);
    }
  }, [schulhofStatus, startSchulhofSpontaneously]);

  // Reset toggling state when schulhofStatus updates (prevents flicker after successful toggle)
  // Also includes a timeout fallback to prevent stuck loading state if SWR refresh fails
  useEffect(() => {
    if (isTogglingSchulhof && schulhofStatus) {
      // When SWR has updated the data, reset the loading state
      setIsTogglingSchulhof(false);
    }
    // Only react to schulhofStatus changes, not isTogglingSchulhof
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [schulhofStatus?.isUserSupervising]);

  // Safety timeout: Reset loading state after 5s if SWR refresh doesn't update status
  // This prevents stuck loading state when refresh fails or returns stale data
  useEffect(() => {
    if (!isTogglingSchulhof) return;

    const timeout = setTimeout(() => {
      logger.warn("Schulhof toggle timeout: resetting loading state after 5s");
      setIsTogglingSchulhof(false);
    }, 5000);

    return () => clearTimeout(timeout);
  }, [isTogglingSchulhof]);

  // Function to switch between rooms (by ID — stable across re-sorts)
  const switchToRoom = async (roomId: string) => {
    if (roomId === selectedRoomId) return;
    const selectedRoom = allRooms.find((r) => r.id === roomId);
    if (!selectedRoom) return;

    setIsLoading(true);
    setSelectedTimetableInstanceId(null);
    setSelectedRoomId(roomId);
    setStudents([]); // Clear current students

    try {
      // Re-run the aggregate for the newly selected session; the sync
      // effect applies its visits and student count when the data arrives.
      requestedGroupIdRef.current = roomId;
      await mutateDashboard();
      setError(null);
    } catch (err) {
      // Handle 403 gracefully - show message but don't break the UI
      if (err instanceof Error && err.message.includes("403")) {
        setError(
          `Keine Berechtigung für "${selectedRoom.name}". Kontaktieren Sie einen Administrator.`,
        );
        setStudents([]); // Show empty list instead of crashing
      } else {
        setError("Fehler beim Laden der Raumdaten.");
        logger.error("failed to load room data", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
    } finally {
      setIsLoading(false);
    }
  };

  // Apply filters to students (ensure students is an array)
  const filteredStudents = (Array.isArray(students) ? students : []).filter(
    (student) =>
      matchesStudentFilters(student, searchTerm, groupFilter, selectedYear),
  );
  const isWaitingForUrlRoomSelection = sessionParam
    ? sessionParam !== SCHULHOF_TAB_ID &&
      allRooms.some((room) => room.id === sessionParam) &&
      currentRoom?.id !== sessionParam
    : !!roomParam &&
      roomParam !== SCHULHOF_TAB_ID &&
      allRooms.some((room) => room.room_id === roomParam) &&
      // A selected session inside the named room settles a room-keyed URL —
      // parallel sessions share the room, so never wait for a "better" match.
      currentRoom?.room_id !== roomParam;

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(() => {
    // Compute available groups inside useMemo to ensure proper updates
    const groups = Array.from(
      new Set(
        students
          .map((student) => student.group_name)
          .filter((name): name is string => !!name),
      ),
    ).sort((a, b) => a.localeCompare(b, "de"));

    return [
      {
        id: "year",
        label: "Klassenstufe",
        type: "buttons",
        value: selectedYear,
        onChange: (value) => setSelectedYear(value as string),
        options: [...SCHOOL_YEAR_FILTER_OPTIONS],
      },
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: groupFilter,
        onChange: (value) => setGroupFilter(value as string),
        options: [
          { value: "all", label: "Alle Gruppen" },
          ...groups.map((groupName) => ({
            value: groupName,
            label: groupName,
          })),
        ],
      },
    ];
  }, [selectedYear, groupFilter, students]);

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (selectedYear !== "all") {
      filters.push({
        id: "year",
        label: `Jahr ${selectedYear}`,
        onRemove: () => setSelectedYear("all"),
      });
    }

    if (groupFilter !== "all") {
      filters.push({
        id: "group",
        label: `Gruppe: ${groupFilter}`,
        onRemove: () => setGroupFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, selectedYear, groupFilter]);

  if (status === "loading" || isLoading || hasAccess === null) {
    return <ActiveSupervisionLoadingView />;
  }

  // Show empty state if no active supervision
  if (!hasAccess) {
    return <NoActiveSupervisionAccessView />;
  }

  const spontaneousStartBanner = dashboardData?.capabilities
    ?.webSpontaneousActivitiesEnabled ? (
    <SpontaneousActivityStart
      currentStaffId={currentStaffId}
      defaultRoomId={currentRoom?.room_id}
      isStarting={isStartingSpontaneous}
      occupiedRoomIds={occupiedRoomIds}
      onStart={(payload) => void handleStartSpontaneousActivity(payload)}
    />
  ) : null;
  const reopenBanner = reopenableInstanceId ? (
    <div className="mb-4">
      <Alert
        type="success"
        message="Aktivität wurde beendet. Die Rücknahme ist fünf Minuten lang möglich."
        action={
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => void handleReopenTimetableInstance()}
          >
            Rückgängig
          </Button>
        }
      />
    </div>
  ) : null;

  // Show unclaimed rooms banner when user has no supervised groups and no Schulhof
  // If the Schulhof tab is available, we'll show the main view with just that tab
  if (
    allRooms.length === 0 &&
    !schulhofTabAvailable &&
    plannedNow.length === 0
  ) {
    return (
      <div className="w-full">
        {reopenBanner}
        {spontaneousStartBanner}
        <EmptyRoomsView
          onClaimed={handleRoomClaimed}
          cachedActiveGroups={cachedActiveGroups}
          currentStaffId={currentStaffId}
          searchTerm={searchTerm}
          setSearchTerm={setSearchTerm}
          setGroupFilter={setGroupFilter}
          setSelectedYear={setSelectedYear}
          filterConfigs={filterConfigs}
          activeFilters={activeFilters}
        />
        {/* The day review must survive the empty state: after the last block
            ends, supervisors land exactly here (#2335). */}
        <PastBlocksSection />
      </div>
    );
  }

  // Render helper for student grid content
  const renderStudentContent = () => {
    if (isWaitingForUrlRoomSelection || isWaitingForTimetableRoster) {
      return <ActiveSupervisionLoadingView withHeader={false} />;
    }

    if (currentTimetableRoster) {
      return (
        <>
          {moveNotice && (
            <div className="mb-4">
              <Alert type="info" message={moveNotice} />
            </div>
          )}
          <TimetableRosterContent
            addStudentResults={addStudentResults}
            addStudentSearch={addStudentSearch}
            attendanceWebEnabled={attendanceWebEnabled}
            isAddingStudent={isAddingStudent}
            isCompletingInstance={isCompletingInstance}
            isConfirmingExpected={isConfirmingExpected}
            roster={currentTimetableRoster}
            showTimetableCounts={showTimetableCounts}
            onAddStudent={handleAddUnplannedStudent}
            onComplete={handleCompleteTimetableInstance}
            onConfirmExpected={handleConfirmExpectedStudents}
            onRosterAction={handleRosterAction}
            onSearchChange={(value) => {
              setAddStudentSearch(value);
              setAddStudentResult(null);
            }}
          />
        </>
      );
    }

    if (students.length === 0) {
      return (
        <div className="py-8 text-center">
          <div className="flex flex-col items-center gap-3">
            <MotoConceptIcon concept="children" size={40} />
            <div>
              <h3 className="text-sm font-medium text-gray-600">
                Keine Kinder in diesem Raum
              </h3>
              <p className="mt-1 text-xs text-gray-500">
                Es wurden noch keine Kinder eingecheckt
              </p>
            </div>
          </div>
        </div>
      );
    }

    if (filteredStudents.length > 0) {
      return (
        <div>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
            {filteredStudents.map((student) => {
              const studentPickup = pickupTimesData?.get(student.id.toString());
              const studentArrival = arrivalTimesData?.get(
                student.id.toString(),
              );
              const presentStudent = withActiveSupervisionPresence(student);
              const arrivalExceptionAbsent =
                (studentArrival?.isException ?? false) &&
                !studentArrival?.expectedArrival;

              return (
                <StudentCard
                  key={student.id}
                  studentId={student.id}
                  firstName={student.first_name}
                  lastName={student.second_name}
                  photoUrl={student.photo_url ?? null}
                  onClick={() =>
                    router.push(
                      `/students/${student.id}?from=/active-supervisions`,
                    )
                  }
                  locationBadge={
                    <StudentPresenceBadge
                      student={presentStudent}
                      displayMode="contextAware"
                      userGroups={myGroupIds}
                      groupRooms={myGroupRooms}
                      variant="modern"
                      size="md"
                    />
                  }
                  extraContent={
                    <>
                      {student.school_class && (
                        <StudentInfoRow icon={<SchoolClassIcon />}>
                          {student.school_class}
                        </StudentInfoRow>
                      )}
                      {student.group_name && (
                        <StudentInfoRow icon={<GroupIcon />}>
                          Gruppe: {student.group_name}
                        </StudentInfoRow>
                      )}
                      {student.pending_excused_note !== undefined && (
                        <StudentPendingExcusedRow
                          note={student.pending_excused_note}
                        />
                      )}
                      {(() => {
                        const absence = getStudentAbsence({
                          sick: presentStudent.sick,
                          classTrip: presentStudent.class_trip,
                          excused: presentStudent.excused,
                        });
                        if (absence && !presentStudent.actual_pickup_time) {
                          return <StudentAbsenceRow label={absence.label} />;
                        }
                        const dayPlanningNotComingLabel =
                          getDayPlanningNotComingLabel(presentStudent);
                        if (
                          dayPlanningNotComingLabel &&
                          !presentStudent.actual_pickup_time
                        ) {
                          return (
                            <StudentAbsenceRow
                              label={dayPlanningNotComingLabel}
                            />
                          );
                        }
                        return (
                          <>
                            <ArrivalTimeRow
                              arrivalTime={studentArrival?.expectedArrival}
                              actualTime={student.actual_arrival_time}
                              isException={
                                !arrivalExceptionAbsent &&
                                (studentArrival?.isException ?? false)
                              }
                              isAbsent={false}
                              notes={
                                studentArrival && !arrivalExceptionAbsent
                                  ? combineTimeNotes(
                                      studentArrival.notes,
                                      studentArrival.dayNotes,
                                    )
                                  : undefined
                              }
                              now={now}
                            />
                            <PickupTimeRow
                              pickupTime={studentPickup?.pickupTime}
                              actualTime={student.actual_pickup_time}
                              isException={studentPickup?.isException ?? false}
                              notes={
                                studentPickup
                                  ? combineTimeNotes(
                                      studentPickup.notes,
                                      studentPickup.dayNotes,
                                    )
                                  : undefined
                              }
                              now={now}
                            />
                          </>
                        );
                      })()}
                    </>
                  }
                  trackingIndicators={
                    trackingData?.labels.length ? (
                      <TrackingIndicators
                        labels={trackingData.labels}
                        results={trackingData.results[student.id] ?? []}
                      />
                    ) : undefined
                  }
                />
              );
            })}
          </div>
        </div>
      );
    }

    return (
      <EmptyStudentResults
        totalCount={students.length}
        filteredCount={filteredStudents.length}
      />
    );
  };

  return (
    <div className="w-full">
      {reopenBanner}
      <ConfirmationModal
        isOpen={showCompleteConfirmation}
        onClose={() => setShowCompleteConfirmation(false)}
        onConfirm={() => void confirmCompleteTimetableInstance()}
        title="Aktivität wirklich beenden?"
        confirmText="Aktivität beenden"
        isConfirmLoading={isCompletingInstance}
        isDismissDisabled={isCompletingInstance}
      >
        <div className="space-y-3 text-sm text-gray-700">
          <p>
            <strong>{currentTimetableRoster?.instance.title}</strong> endet laut
            Plan um {currentTimetableRoster?.instance.endTime} Uhr.
          </p>
          <p>
            Aktuell anwesend:{" "}
            {currentTimetableRoster?.rows.filter((row) => row.currentlyPresent)
              .length ?? 0}
          </p>
          {(currentTimetableRoster?.rows.filter((row) => row.currentlyPresent)
            .length ?? 0) > 0 ? (
            <ul className="list-disc space-y-1 pl-5">
              {currentTimetableRoster?.rows
                .filter((row) => row.currentlyPresent)
                .map((row) => (
                  <li key={row.studentId}>{row.studentName}</li>
                ))}
            </ul>
          ) : null}
        </div>
      </ConfirmationModal>
      {/* Unclaimed Rooms Section - Shows rooms available for claiming */}
      <UnclaimedRooms
        onClaimed={handleRoomClaimed}
        activeGroups={
          cachedActiveGroups.length > 0 ? cachedActiveGroups : undefined
        }
        currentStaffId={currentStaffId}
      />

      <PlannedNowSection
        plannedNow={plannedNow}
        hasActiveTimetableSession={currentTimetableRoster !== null}
        isStartingInstance={isStartingInstance}
        onStart={(instance) => void handleStartPlannedInstance(instance)}
      />

      {spontaneousStartBanner}

      {/* Modern Header with PageHeaderWithSearch component */}
      {/* With the permanent tab enabled, exclude only the active group already
          represented by schulhofStatus. Other parallel Schulhof sessions stay
          reachable as normal supervision tabs. */}
      {(() => {
        const roomsOutsideStatus = roomsOutsideSchulhofStatus(allRooms, {
          schulhofTabEnabled,
          statusActiveGroupId: schulhofStatus?.activeGroupId,
        });
        const totalSupervisions =
          roomsOutsideStatus.length + (schulhofTabAvailable ? 1 : 0);

        return (
          <PageHeaderWithSearch
            title={
              // Mobile only: Show title when exactly 1 supervision
              // 1 supervision = title, 2+ supervisions = tabs (dropdown)
              !isDesktop && totalSupervisions === 1
                ? isSchulhofTabSelected
                  ? SCHULHOF_ROOM_NAME
                  : currentRoom
                    ? supervisionTabLabel(
                        currentRoom,
                        sessionInfoByActiveGroup.get(currentRoom.id) ?? null,
                      )
                    : "Aktuelle Aufsicht"
                : ""
            }
            badge={{
              icon: (
                <svg
                  className="h-5 w-5 text-gray-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                  />
                </svg>
              ),
              count: isSchulhofTabSelected
                ? (schulhofStatus?.studentCount ?? 0)
                : (currentRoom?.student_count ?? 0),
              label: "Kinder",
            }}
            tabs={
              // Show tabs (dropdown) when 2+ supervisions
              totalSupervisions >= 2 && !isDesktop
                ? {
                    items: [
                      // Regular supervised sessions, including any parallel
                      // Schulhof group not represented by the permanent tab.
                      ...roomsOutsideStatus.map((room) => ({
                        id: room.id,
                        label: supervisionTabLabel(
                          room,
                          sessionInfoByActiveGroup.get(room.id) ?? null,
                        ),
                      })),
                      // Schulhof permanent tab (only with the spontaneous
                      // capability, #2161)
                      ...(schulhofTabAvailable
                        ? [
                            {
                              id: SCHULHOF_TAB_ID,
                              label: SCHULHOF_ROOM_NAME,
                            },
                          ]
                        : []),
                    ],
                    activeTab: isSchulhofTabSelected
                      ? SCHULHOF_TAB_ID
                      : (currentRoom?.id ?? ""),
                    onTabChange: (tabId) => {
                      if (tabId === SCHULHOF_TAB_ID) {
                        handleOpenSchulhofSupervision();
                      } else {
                        // Switch to the chosen session (keyed by active
                        // group, not by room — parallel sessions can share
                        // one room, #2265)
                        setIsSchulhofTabSelected(false);
                        const room = allRooms.find((r) => r.id === tabId);
                        if (room) {
                          router.push(`/active-supervisions?session=${tabId}`);
                          localStorage.setItem(
                            "supervision-last-session",
                            tabId,
                          );
                          if (room.room_id) {
                            localStorage.setItem(
                              "sidebar-last-room",
                              room.room_id,
                            );
                          }
                          if (room.room_name) {
                            localStorage.setItem(
                              "sidebar-last-room-name",
                              room.room_name,
                            );
                          }
                          void switchToRoom(tabId);
                        }
                      }
                    },
                  }
                : undefined
            }
            search={{
              value: searchTerm,
              onChange: setSearchTerm,
              placeholder: "Name suchen...",
            }}
            filters={filterConfigs}
            activeFilters={activeFilters}
            onClearAllFilters={() => {
              setSearchTerm("");
              setGroupFilter("all");
              setSelectedYear("all");
            }}
            actionButton={
              // Only show release button when user IS supervising Schulhof
              // "Beaufsichtigen" button is shown in the empty state instead (no duplicate)
              isSchulhofTabSelected && schulhofStatus?.isUserSupervising ? (
                <button
                  type="button"
                  onClick={() => setShowReleaseModal(true)}
                  className="flex h-10 items-center gap-2 rounded-full border border-red-200 bg-red-50 px-4 text-red-600 transition-colors hover:bg-red-100"
                  aria-label="Aufsicht abgeben"
                >
                  <LogOut className="h-5 w-5" aria-hidden="true" />
                  <span className="text-sm font-medium">Aufsicht abgeben</span>
                </button>
              ) : undefined
            }
            mobileActionButton={
              // Only show release button when user IS supervising Schulhof
              isSchulhofTabSelected && schulhofStatus?.isUserSupervising ? (
                <button
                  type="button"
                  onClick={() => setShowReleaseModal(true)}
                  className="flex h-8 w-8 items-center justify-center rounded-full border border-red-200 bg-red-50 text-red-600 transition-colors hover:bg-red-100"
                  aria-label="Aufsicht abgeben"
                >
                  <LogOut className="h-4 w-4" aria-hidden="true" />
                </button>
              ) : undefined
            }
          />
        );
      })()}

      {/* Schulhof Release Supervision Modal */}
      <ReleaseSupervisionModal
        isOpen={showReleaseModal}
        onClose={() => setShowReleaseModal(false)}
        onConfirm={() => handleReleaseSupervision().catch(() => undefined)}
        isConfirmLoading={isReleasingSupervision}
      />

      {/* Mobile Error Display */}
      {error && (
        <div className="mb-4 md:hidden">
          <Alert type="error" message={error} />
        </div>
      )}

      {/* Schulhof Not Supervising View - matches suggestions page empty state style */}
      {isSchulhofTabSelected &&
        schulhofStatus &&
        !schulhofStatus.isUserSupervising && (
          <SchulhofNotSupervisingView
            supervisorCount={schulhofStatus.supervisorCount}
            supervisorNames={schulhofStatus.supervisors.map((s) => s.name)}
            isToggling={isTogglingSchulhof}
            onToggle={() => handleToggleSchulhof().catch(() => undefined)}
          />
        )}

      {currentRoom &&
      (!isSchulhofTabSelected || schulhofStatus?.isUserSupervising) ? (
        <div className="mb-4">
          <Suspense fallback={null}>
            <TransitStudentsSection
              fromReferrer="/active-supervisions"
              collapsible
            />
          </Suspense>
        </div>
      ) : null}

      {/* Student Grid - Mobile Optimized */}
      {(!isSchulhofTabSelected || schulhofStatus?.isUserSupervising) &&
        renderStudentContent()}

      {/* Read-only end-of-day review of finished and expired blocks (#2335) */}
      <PastBlocksSection />
    </div>
  );
}

// Gate component: allows caregivers always, everyone else only when the
// server confirmed the school-wide overview covers them (#2380).
function ActiveSupervisionGate({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const { overviewEnabled, isLoadingSupervision } =
    useOptionalSupervision();

  if (status === "loading" || isLoadingSupervision) {
    return <ActiveSupervisionLoadingView />;
  }

  // Caregivers (user/teacher role) always have access
  if (isCaregiver(session)) {
    return <>{children}</>;
  }

  // Admins only when /api/active/supervisors/all returned OK, i.e. the
  // school-wide overview covers them. Checking supervisedRooms.length would
  // incorrectly let admins through when the scope is "own" but a synthetic
  // Schulhof entry is present.
  if (isAdmin(session) && overviewEnabled) {
    return <>{children}</>;
  }

  return <ForbiddenPage />;
}

// Main component with Suspense wrapper. BinaryModeGuard runs first so
// binary-mode tenants get a 404 before the supervision gate tries to load
// data that depends on detailed-mode room visits.
export default function MeinRaumPage() {
  return (
    <BinaryModeGuard>
      <Suspense
        fallback={
          <SkeletonRegion label="Mein Raum wird geladen">
            <PageHeaderSkeleton actions={1} />
            <CardGridSkeleton
              cards={6}
              rowsPerCard={2}
              className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3"
            />
          </SkeletonRegion>
        }
      >
        <ActiveSupervisionGate>
          <SSEErrorBoundary>
            <MeinRaumPageContent />
          </SSEErrorBoundary>
        </ActiveSupervisionGate>
      </Suspense>
    </BinaryModeGuard>
  );
}
