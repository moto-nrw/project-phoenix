"use client";

import {
  useState,
  useEffect,
  Suspense,
  useMemo,
  useCallback,
  useRef,
} from "react";
import { CheckCircle2, UserPlus } from "lucide-react";
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
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { Loading } from "~/components/ui/loading";
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
import { fetchBulkPickupTimes } from "~/lib/pickup-schedule-api";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";
import { fetchBulkArrivalTimes } from "~/lib/student-arrival-api";
import type { BulkArrivalTime } from "~/lib/student-arrival-api";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { activeService } from "~/lib/active-api";
import { fetchStudents } from "~/lib/student-api";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
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
  withActiveSupervisionPresence,
} from "~/components/active-supervisions/view-model";
import type {
  ActiveSupervisionRoom,
  ActiveSupervisionStudent,
  MinimalActiveGroup,
  SchulhofStatusResponse,
} from "~/components/active-supervisions/view-model";

const logger = createLogger({ component: "ActiveSupervisionsPage" });

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
  plannedNow: PlannedTimetableInstance[];
}

const GROUP_CARD_GRADIENT = "from-blue-50/80 to-cyan-100/80";

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
          className="rounded-md bg-[#83CD2D] px-3 py-2 text-sm font-medium text-white"
        >
          Einchecken
        </button>
      ) : null}
      {!row.currentlyPresent && row.status !== "expected" ? (
        <button
          type="button"
          onClick={() => runAction("check-in")}
          className="rounded-md bg-[#83CD2D] px-3 py-2 text-sm font-medium text-white"
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
            className="rounded-md border border-[#D89A16] px-3 py-2 text-sm font-medium text-[#A66F00]"
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
          <div className="mt-1 text-sm text-[#D89A16]">{attendanceDetail}</div>
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
    <div className="moto-content-surface overflow-hidden rounded-2xl border border-[#83CD2D]/30 shadow-sm backdrop-blur-md">
      <div className="flex flex-col gap-3 border-b border-[#83CD2D]/20 bg-[#83CD2D]/10 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/20 text-[#4A7A15]">
            <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-[#4A7A15] uppercase">
              Aktiv
            </p>
            <h2 className="truncate text-base font-semibold text-gray-900">
              {roster.instance.title}
            </h2>
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
              className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-[#83CD2D] px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-[#74B827] focus-visible:ring-2 focus-visible:ring-[#83CD2D]/30 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
              {confirmLabel}
            </button>
          ) : null}
          {attendanceWebEnabled ? (
            <button
              type="button"
              disabled={isCompletingInstance}
              onClick={handleCompleteClick}
              className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              Beenden
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
  readonly onAdd: (studentId: string) => Promise<void>;
  readonly onSearchChange: (value: string) => void;
}

function AddUnplannedStudentForm({
  isAddingStudent,
  results,
  search,
  onAdd,
  onSearchChange,
}: AddUnplannedStudentFormProps) {
  const addStudent = async (studentId: string) => {
    await onAdd(studentId);
  };
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const onlyResult = results[0];
    if (onlyResult && results.length === 1) {
      await addStudent(onlyResult.id.toString());
    }
  };

  return (
    <form
      className="moto-content-surface rounded-2xl border p-4 shadow-sm"
      onSubmit={handleSubmit}
    >
      <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <UserPlus className="h-4 w-4 text-gray-400" aria-hidden="true" />
        Kind ungeplant hinzufügen
      </div>
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          type="search"
          name="unplanned-student-search"
          aria-label="Kind ungeplant suchen"
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Weiteres Kind suchen..."
          className="min-h-10 flex-1 rounded-lg border border-gray-300 px-3 text-sm focus:border-[#83CD2D] focus:ring-2 focus:ring-[#83CD2D]/20 focus:outline-none"
        />
        <button
          type="submit"
          disabled={isAddingStudent || results.length !== 1}
          className="rounded-lg bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-[#74B827] disabled:opacity-50"
        >
          Hinzufügen
        </button>
      </div>
      {results.length > 0 ? (
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {results.map((student) => (
            <button
              key={student.id}
              type="button"
              disabled={isAddingStudent}
              onClick={() => addStudent(student.id.toString())}
              className="rounded-md border border-gray-200 px-3 py-2 text-left text-sm hover:border-[#83CD2D] disabled:opacity-50"
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
            </button>
          ))}
        </div>
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
  readonly onAddStudent: (studentId: string) => Promise<void>;
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

  // Pre-select room from URL param (?room=<id>)
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
  const [isConfirmingExpected, setIsConfirmingExpected] = useState(false);
  const [addStudentSearch, setAddStudentSearch] = useState("");
  const [addStudentResults, setAddStudentResults] = useState<Student[]>([]);
  const [isAddingStudent, setIsAddingStudent] = useState(false);

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

  // True when Schulhof is the active view — either via the permanent tab flag
  // or because the sidebar navigated with the room's actual ID (not "schulhof")
  const isSchulhofActive =
    isSchulhofTabSelected || currentRoom?.room_name === SCHULHOF_ROOM_NAME;
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

  // Set breadcrumb so header shows current room name
  useSetBreadcrumb({
    activeSupervisionName: isSchulhofActive
      ? SCHULHOF_ROOM_NAME
      : currentRoom?.room_name,
  });

  // Helper function to load visits for a specific room
  const loadRoomVisits = useCallback(
    async (
      roomId: string,
      roomName?: string,
      groupNameToId?: Map<string, string>,
      roomColor?: string | null,
    ): Promise<StudentWithVisit[]> => {
      try {
        // Use bulk endpoint to fetch visits with display data for specific room
        const visits =
          await activeService.getActiveGroupVisitsWithDisplay(roomId);

        return mapVisitsToSupervisionStudents(visits, {
          roomName,
          roomColor,
          groupNameToId,
        });
      } catch (error) {
        // Handle 403 Forbidden gracefully - user might not have group access
        if (error instanceof Error && error.message.includes("403")) {
          logger.warn("no permission to view group", { group_id: roomId });
          return []; // Return empty array instead of throwing
        }
        // Re-throw other errors
        throw error;
      }
    },
    [],
  );

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
  // When student_checkin/checkout events occur, global SSE invalidates "visit*" caches,
  // which triggers SWR refetch for supervision-visits-* keys automatically.
  // NOTE: Do NOT call useGlobalSSE() here - it's already called in TenantAuthWrapper.
  // Calling it again would create a duplicate SSE connection.

  // Admin fallback: when BFF fails, load supervised groups directly
  const fetchAdminDashboardFallback =
    useCallback(async (): Promise<BFFDashboardResponse> => {
      const [groupsRes, schulhofRes] = await Promise.all([
        fetch("/api/active/supervisors/all", {
          headers: { "Content-Type": "application/json" },
          cache: "no-store",
        }),
        fetch("/api/active/schulhof/status", {
          headers: { "Content-Type": "application/json" },
          cache: "no-store",
        }).catch(() => null),
      ]);

      let supervisedGroups: BFFDashboardResponse["supervisedGroups"] = [];
      if (groupsRes.ok) {
        const json = (await groupsRes.json()) as {
          data?: Array<{
            id: number;
            name?: string;
            room_id?: number;
            room?: { id: number; name: string };
          }>;
        };
        supervisedGroups = (json.data ?? []).map((g) => ({
          id: g.id.toString(),
          name: g.name ?? "",
          room_id: g.room_id?.toString(),
          room: g.room
            ? { id: g.room.id.toString(), name: g.room.name }
            : undefined,
        }));
      }

      let schulhofStatus: BFFDashboardResponse["schulhofStatus"] = null;
      if (schulhofRes?.ok) {
        const json = (await schulhofRes.json()) as {
          data?: {
            data?: {
              exists: boolean;
              room_id?: number;
              room_name: string;
              active_group_id?: number;
              is_user_supervising: boolean;
              supervision_id?: number;
              supervisor_count: number;
              student_count: number;
              supervisors?: Array<{
                id: number;
                staff_id: number;
                name: string;
                is_current_user: boolean;
              }>;
            };
          };
        };
        const s = json.data?.data;
        if (s?.exists) {
          schulhofStatus = {
            exists: true,
            roomId: s.room_id?.toString() ?? null,
            roomName: s.room_name,
            activityGroupId: null,
            activeGroupId: s.active_group_id?.toString() ?? null,
            isUserSupervising: s.is_user_supervising,
            supervisionId: s.supervision_id?.toString() ?? null,
            supervisorCount: s.supervisor_count,
            studentCount: s.student_count,
            supervisors: (s.supervisors ?? []).map((sup) => ({
              id: sup.id.toString(),
              staffId: sup.staff_id.toString(),
              name: sup.name,
              isCurrentUser: sup.is_current_user,
            })),
          };
        }
      }

      return {
        supervisedGroups,
        unclaimedGroups: [],
        currentStaff: null,
        educationalGroups: [],
        firstRoomVisits: [],
        firstRoomId:
          supervisedGroups.length > 0
            ? (supervisedGroups[0]?.room_id ?? null)
            : null,
        schulhofStatus,
        plannedNow: [],
      };
    }, []);

  // Get current room ID for per-room SWR subscription
  const currentRoomId = currentRoom?.id;

  // SWR-based BFF data fetching with caching
  // Cache key "active-supervision-dashboard" will be invalidated by global SSE on relevant events
  const {
    data: dashboardData,
    isLoading: isDashboardLoading,
    error: dashboardError,
    mutate: mutateDashboard,
  } = useSWRAuth<BFFDashboardResponse>(
    session?.user?.token ? `active-supervision-dashboard-${refreshKey}` : null,
    async () => {
      logger.debug("SWR fetching BFF data");
      const start = performance.now();

      const response = await fetch("/api/active-supervision-dashboard", {
        headers: {
          Authorization: `Bearer ${session?.user?.token}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        // Admin fallback: load data directly from individual endpoints
        if (isAdmin(session)) {
          logger.info("bff_failed_admin_fallback", {
            status: response.status,
          });
          return await fetchAdminDashboardFallback();
        }
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

    // Use pre-loaded visits from BFF for the first room
    // IMPORTANT: Only apply first room visits when the first room is selected.
    // When SSE triggers revalidation while user views another room, we must NOT
    // overwrite their current view with the first room's data.
    const firstRoom = activeRooms[0];

    // If the previously selected room no longer exists in the refreshed list
    // (e.g., supervision revoked, session ended), reset to the first room so
    // the student data stays in sync with what the UI displays.
    if (selectedRoomId && !activeRooms.some((r) => r.id === selectedRoomId)) {
      setSelectedRoomId(firstRoom?.id ?? null);
    }

    // Skip first-room preload when Schulhof tab is active — Schulhof uses
    // selectedRoomId=null intentionally, so !selectedRoomId would incorrectly
    // match and overwrite Schulhof students with first-room data.
    const isUrlTargetingDifferentRoom =
      !!roomParam &&
      roomParam !== SCHULHOF_TAB_ID &&
      activeRooms.some((room) => room.room_id === roomParam) &&
      firstRoom?.room_id !== roomParam;

    if (
      !isSchulhofTabSelected &&
      !isUrlTargetingDifferentRoom &&
      (!selectedRoomId || selectedRoomId === firstRoom?.id)
    ) {
      // When no room is explicitly selected yet, lock in the first room's ID
      // so the URL-sync effect won't try to "switch" to it via localStorage.
      if (!selectedRoomId && firstRoom) {
        setSelectedRoomId(firstRoom.id);
      }
      if (firstRoom && data.firstRoomVisits.length > 0) {
        const studentsFromVisits = mapVisitsToSupervisionStudents(
          data.firstRoomVisits,
          {
            roomName: firstRoom.room_name,
            roomColor: firstRoom.room_color,
            groupNameToId: nameToIdMap,
          },
        );

        setStudents(studentsFromVisits);
        updateRoomStudentCount(firstRoom.id, studentsFromVisits.length);
      } else if (firstRoom) {
        setStudents([]);
        updateRoomStudentCount(firstRoom.id, 0);
      }
    }

    setError(null);
    setIsLoading(false);
  }, [
    dashboardData,
    dashboardError,
    updateRoomStudentCount,
    selectedRoomId,
    isSchulhofTabSelected,
    roomParam,
  ]);

  // Sync selected room with URL param.
  // The sidebar navigates with the correct ?room= param at click-time,
  // so this effect only needs to react to URL changes.
  // When no param is present (e.g. fresh login), persist the default (first room)
  // so localStorage stays in sync and the sidebar picks it up on next click.
  useEffect(() => {
    // Handle Schulhof param specially
    if (roomParam === "schulhof" && schulhofStatus?.exists) {
      if (!isSchulhofTabSelected) {
        setIsSchulhofTabSelected(true);
        setSelectedRoomId(null);
        // Load Schulhof visits if supervising
        if (schulhofStatus.isUserSupervising && schulhofStatus.activeGroupId) {
          loadRoomVisits(
            schulhofStatus.activeGroupId,
            SCHULHOF_ROOM_NAME,
            groupNameToIdMapRef.current,
          )
            .then(setStudents)
            .catch(() => {
              // Error already handled in loadRoomVisits
            });
        } else {
          setStudents([]);
        }
      }
      return;
    }

    if (allRooms.length === 0) return;

    if (roomParam) {
      // Switch away from Schulhof if selecting a different room
      if (isSchulhofTabSelected) {
        setIsSchulhofTabSelected(false);
      }
      const targetRoom = allRooms.find((r) => r.room_id === roomParam);
      if (targetRoom && targetRoom.id !== selectedRoomId) {
        void switchToRoom(targetRoom.id);
      }
    } else {
      // No ?room= param (e.g. after login or browser back) — restore from
      // localStorage so the user returns to their previously selected room.
      const savedRoomId = localStorage.getItem("sidebar-last-room");

      // Handle Schulhof restore from localStorage
      if (savedRoomId === SCHULHOF_TAB_ID && schulhofStatus?.exists) {
        if (!isSchulhofTabSelected) {
          setIsSchulhofTabSelected(true);
          setSelectedRoomId(null);
          if (
            schulhofStatus.isUserSupervising &&
            schulhofStatus.activeGroupId
          ) {
            loadRoomVisits(
              schulhofStatus.activeGroupId,
              SCHULHOF_ROOM_NAME,
              groupNameToIdMapRef.current,
            )
              .then(setStudents)
              .catch(() => {
                // Error already handled in loadRoomVisits
              });
          } else {
            setStudents([]);
          }
        }
        return;
      }

      const savedRoom = savedRoomId
        ? allRooms.find((r) => r.room_id === savedRoomId)
        : undefined;
      if (savedRoom && savedRoom.id !== selectedRoomId) {
        void switchToRoom(savedRoom.id);
      } else if (!savedRoom) {
        // Nothing saved or saved room no longer exists — persist first room
        const firstRoom = allRooms[0];
        if (firstRoom?.room_id) {
          localStorage.setItem("sidebar-last-room", firstRoom.room_id);
        }
      }
      // When savedRoom.id === selectedRoomId, do nothing — already in sync
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    allRooms,
    roomParam,
    schulhofStatus?.exists,
    schulhofStatus?.activeGroupId,
    schulhofStatus?.isUserSupervising,
  ]);

  // SWR-based per-room visit subscription for real-time updates.
  // When global SSE invalidates "visit*" or "supervision*" caches, this triggers a refetch.
  // This ensures non-first rooms also receive real-time check-in/checkout updates.
  const { data: swrVisitsData } = useSWRAuth<StudentWithVisit[]>(
    hasAccess && currentRoomId ? `supervision-visits-${currentRoomId}` : null,
    async () => {
      if (!currentRoom) return [];

      const visits = await activeService.getActiveGroupVisitsWithDisplay(
        currentRoomId!,
      );

      return mapVisitsToSupervisionStudents(visits, {
        roomName: currentRoom.room_name,
        roomColor: currentRoom.room_color,
        groupNameToId: groupNameToIdMapRef.current,
      });
    },
    {
      // Never expose one room's children under another room's heading while
      // the new key loads. Same-key SSE revalidation still keeps its cache.
      keepPreviousData: false,
      revalidateOnFocus: false, // Handled by global SSE
    },
  );

  // Sync SWR visit data with local state
  // This runs when SSE triggers cache invalidation, ensuring real-time updates for ALL rooms
  useEffect(() => {
    if (swrVisitsData && currentRoomId) {
      setStudents(swrVisitsData);
      updateRoomStudentCount(currentRoomId, swrVisitsData.length);
    }
  }, [swrVisitsData, currentRoomId, updateRoomStudentCount]);

  const timetableRosterKey = activeSupervisionRosterKey({
    selectedTimetableInstanceId,
    currentRoomId,
    isSchulhofActive,
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
  const isWaitingForTimetableRoster =
    timetableRosterKey !== null &&
    (timetableRoster === undefined ||
      (timetableRoster !== null && !timetableRosterMatchesSelection)) &&
    isTimetableRosterLoading;

  useEffect(() => {
    if (!activeTimetableInstanceId || addStudentSearch.trim().length < 2) {
      setAddStudentResults([]);
      return;
    }

    const timeout = window.setTimeout(() => {
      fetchStudents({
        search: addStudentSearch.trim(),
        page: 1,
        page_size: 5,
      })
        .then((result) => {
          setAddStudentResults(result.students);
        })
        .catch((err) => {
          logger.warn("failed to search students for timetable roster", {
            error: err instanceof Error ? err.message : String(err),
          });
          setAddStudentResults([]);
        });
    }, 250);

    return () => window.clearTimeout(timeout);
  }, [activeTimetableInstanceId, addStudentSearch]);

  // Tracking indicators: fetch when student list changes (SSE-driven via SWR revalidation)
  const trackingStudentIds = useMemo(
    () => students.map((s) => s.id),
    [students],
  );
  const { data: trackingData } = useSWRAuth<TrackingIndicatorsResponse>(
    trackingStudentIds.length > 0
      ? `tracking-supervisions-${currentRoomId}-${trackingStudentIds.join(",")}`
      : null,
    async () => activeService.getTrackingIndicators(trackingStudentIds),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Current time for pickup urgency calculation (updates every minute)
  const now = useMinuteClock();

  // Pickup times: fetch when student list or date changes
  const todayKey = toISODate(now);
  const { data: pickupTimesData } = useSWRAuth<Map<string, BulkPickupTime>>(
    trackingStudentIds.length > 0 && currentRoomId
      ? `pickup-supervisions-${todayKey}-${trackingStudentIds.join(",")}`
      : null,
    async () => fetchBulkPickupTimes(trackingStudentIds),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Arrival times: fetch when student list or date changes
  const { data: arrivalTimesRaw } = useSWRAuth<Map<string, BulkArrivalTime>>(
    trackingStudentIds.length > 0 && currentRoomId
      ? `arrival-supervisions-${todayKey}-${trackingStudentIds.join(",")}`
      : null,
    async () => fetchBulkArrivalTimes(trackingStudentIds),
    { keepPreviousData: true, revalidateOnFocus: false },
  );
  const arrivalTimesData: Map<string, BulkArrivalTime> | undefined =
    arrivalTimesRaw instanceof Map ? arrivalTimesRaw : undefined;

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
    if (schulhofStatus?.exists || !isSchulhofTabSelected) return;

    setIsSchulhofTabSelected(false);
    setSelectedRoomId(allRooms[0]?.id ?? null);
    setSelectedTimetableInstanceId(null);
    setStudents([]);
  }, [allRooms, isSchulhofTabSelected, schulhofStatus?.exists]);

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
      schulhofStatus?.exists &&
      !isSchulhofTabSelected
    ) {
      setIsSchulhofTabSelected(true);
    }
  }, [allRooms.length, schulhofStatus?.exists, isSchulhofTabSelected]);

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
        router.push(`/active-supervisions?room=${instance.roomId}`);
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
        router.push(`/active-supervisions?room=${payload.roomId}`);
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
    router.push("/active-supervisions?room=schulhof");
    localStorage.setItem("sidebar-last-room", SCHULHOF_TAB_ID);
    localStorage.setItem("sidebar-last-room-name", SCHULHOF_ROOM_NAME);
    // The keyed SWR subscription becomes active with the Schulhof tab and is
    // the single owner of loading its visits. A second manual request can land
    // later and overwrite a fresher SSE revalidation.
    setStudents([]);
  }, [router, schulhofStatusRef]);

  const handleRosterAction = useCallback(
    async (
      action: "check-in" | "check-out" | "excused" | "absent" | "expected",
      row: TimetableRosterRow,
    ) => {
      if (!activeTimetableInstanceId) return;
      try {
        if (action === "check-in") {
          const roster = await timetableOperationsApi.checkIn(
            activeTimetableInstanceId,
            row.studentId,
          );
          await mutateRoster(roster, { revalidate: false });
        } else if (action === "check-out") {
          const roster = await timetableOperationsApi.checkOut(
            activeTimetableInstanceId,
            row.studentId,
          );
          await mutateRoster(roster, { revalidate: false });
        } else if (action === "expected") {
          await timetableOperationsApi.patchAttendance(
            activeTimetableInstanceId,
            row.studentId,
            { status: "expected", substatus: null, note: null },
          );
          await mutateRoster();
        } else {
          await timetableOperationsApi.patchAttendance(
            activeTimetableInstanceId,
            row.studentId,
            action === "excused"
              ? { status: "absent", substatus: "excused" }
              : { status: "absent" },
          );
          await mutateRoster();
        }
      } catch (err) {
        logger.error("failed timetable roster action", {
          action,
          student_id: row.studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Aktion im Betreuungsplan konnte nicht ausgeführt werden.");
      }
    },
    [activeTimetableInstanceId, mutateRoster],
  );

  const handleCompleteTimetableInstance = useCallback(async () => {
    if (!activeTimetableInstanceId) return;
    try {
      setIsCompletingInstance(true);
      await timetableOperationsApi.complete(activeTimetableInstanceId);
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
  }, [activeTimetableInstanceId, mutateDashboard]);

  const handleConfirmExpectedStudents = useCallback(
    async (rows: TimetableRosterRow[]) => {
      if (!activeTimetableInstanceId || rows.length === 0) return;
      try {
        setIsConfirmingExpected(true);
        let nextRoster: TimetableRoster | null = null;
        for (const row of rows) {
          nextRoster = await timetableOperationsApi.checkIn(
            activeTimetableInstanceId,
            row.studentId,
          );
        }
        if (nextRoster) {
          await mutateRoster(nextRoster, { revalidate: false });
        } else {
          await mutateRoster();
        }
        await mutateDashboard();
        setRefreshKey((prev) => prev + 1);
      } catch (err) {
        logger.error("failed to confirm expected timetable students", {
          instance_id: activeTimetableInstanceId,
          count: rows.length,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Erwartete Kinder konnten nicht bestätigt werden.");
      } finally {
        setIsConfirmingExpected(false);
      }
    },
    [activeTimetableInstanceId, mutateDashboard, mutateRoster],
  );

  const handleAddUnplannedStudent = useCallback(
    async (studentId: string) => {
      if (!activeTimetableInstanceId) return;
      try {
        setIsAddingStudent(true);
        const roster = await timetableOperationsApi.checkIn(
          activeTimetableInstanceId,
          studentId,
        );
        setAddStudentSearch("");
        setAddStudentResults([]);
        await mutateRoster(roster, { revalidate: false });
      } catch (err) {
        logger.error("failed to add unplanned timetable student", {
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Kind konnte nicht zur Aktivität hinzugefügt werden.");
      } finally {
        setIsAddingStudent(false);
      }
    },
    [activeTimetableInstanceId, mutateRoster],
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

  // Handle toggling Schulhof supervision (start/stop)
  const handleToggleSchulhof = useCallback(async () => {
    if (!schulhofStatus) return;

    try {
      setIsTogglingSchulhof(true);
      const action = schulhofStatus.isUserSupervising ? "stop" : "start";
      await activeService.toggleSchulhofSupervision(action);

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
  }, [schulhofStatus]);

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
      // Use bulk endpoint to fetch visits for selected room
      const studentsFromVisits = await loadRoomVisits(
        selectedRoom.id,
        selectedRoom.room_name,
        groupNameToIdMapRef.current,
        selectedRoom.room_color,
      );

      // Set students state
      setStudents([...studentsFromVisits]);

      // Update room with actual student count
      setAllRooms((prev) =>
        prev.map((room) =>
          room.id === roomId
            ? { ...room, student_count: studentsFromVisits.length }
            : room,
        ),
      );

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
  const isWaitingForUrlRoomSelection =
    !!roomParam &&
    roomParam !== SCHULHOF_TAB_ID &&
    allRooms.some((room) => room.room_id === roomParam) &&
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
      schulhofSupervisionAvailable={schulhofStatus?.exists === true}
      onStart={(payload) => void handleStartSpontaneousActivity(payload)}
      onOpenSchulhofSupervision={handleOpenSchulhofSupervision}
    />
  ) : null;

  // Show unclaimed rooms banner when user has no supervised groups and no Schulhof
  // If Schulhof exists, we'll show the main view with just the Schulhof tab
  if (
    allRooms.length === 0 &&
    !schulhofStatus?.exists &&
    plannedNow.length === 0
  ) {
    return (
      <div className="w-full">
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
      </div>
    );
  }

  // Render helper for student grid content
  const renderStudentContent = () => {
    if (isWaitingForUrlRoomSelection || isWaitingForTimetableRoster) {
      return <ActiveSupervisionLoadingView />;
    }

    if (currentTimetableRoster) {
      return (
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
          onSearchChange={setAddStudentSearch}
        />
      );
    }

    if (students.length === 0) {
      return (
        <div className="py-8 text-center">
          <div className="flex flex-col items-center gap-3">
            <svg
              className="h-10 w-10 text-gray-300"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
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
                  gradient={GROUP_CARD_GRADIENT}
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
      {/* Count rooms EXCLUDING Schulhof (to avoid double-counting with schulhofStatus) */}
      {(() => {
        const roomsWithoutSchulhof = allRooms.filter(
          (room) => room.room_name !== SCHULHOF_ROOM_NAME,
        );
        const totalSupervisions =
          roomsWithoutSchulhof.length + (schulhofStatus?.exists ? 1 : 0);

        return (
          <PageHeaderWithSearch
            title={
              // Mobile only: Show title when exactly 1 supervision
              // 1 supervision = title, 2+ supervisions = tabs (dropdown)
              !isDesktop && totalSupervisions === 1
                ? isSchulhofActive
                  ? SCHULHOF_ROOM_NAME
                  : (currentRoom?.room_name ?? "Aktuelle Aufsicht")
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
              count: isSchulhofActive
                ? (schulhofStatus?.studentCount ?? 0)
                : (currentRoom?.student_count ?? 0),
              label: "Kinder",
            }}
            tabs={
              // Show tabs (dropdown) when 2+ supervisions
              totalSupervisions >= 2 && !isDesktop
                ? {
                    items: [
                      // Regular supervised rooms (excluding Schulhof)
                      ...roomsWithoutSchulhof.map((room) => ({
                        id: room.id,
                        label: room.room_name ?? room.name,
                      })),
                      // Schulhof permanent tab (always shown if exists)
                      ...(schulhofStatus?.exists
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
                        // Switch to regular room
                        setIsSchulhofTabSelected(false);
                        const room = allRooms.find((r) => r.id === tabId);
                        if (room) {
                          if (room.room_id) {
                            router.push(
                              `/active-supervisions?room=${room.room_id}`,
                            );
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
              isSchulhofActive && schulhofStatus?.isUserSupervising ? (
                <button
                  type="button"
                  onClick={() => setShowReleaseModal(true)}
                  className="flex h-10 items-center gap-2 rounded-full border border-red-200 bg-red-50 px-4 text-red-600 transition-colors hover:bg-red-100"
                  aria-label="Aufsicht abgeben"
                >
                  <svg
                    className="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                    />
                  </svg>
                  <span className="text-sm font-medium">Aufsicht abgeben</span>
                </button>
              ) : undefined
            }
            mobileActionButton={
              // Only show release button when user IS supervising Schulhof
              isSchulhofActive && schulhofStatus?.isUserSupervising ? (
                <button
                  type="button"
                  onClick={() => setShowReleaseModal(true)}
                  className="flex h-8 w-8 items-center justify-center rounded-full border border-red-200 bg-red-50 text-red-600 transition-colors hover:bg-red-100"
                  aria-label="Aufsicht abgeben"
                >
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                    />
                  </svg>
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
      {isSchulhofActive &&
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
      (!isSchulhofActive || schulhofStatus?.isUserSupervising) ? (
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
      {(!isSchulhofActive || schulhofStatus?.isUserSupervising) &&
        renderStudentContent()}
    </div>
  );
}

// Gate component: allows caregivers always, admins only when they have supervised rooms
// (i.e., admin_supervision_overview setting is active and there are active groups)
function ActiveSupervisionGate({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const { adminOverviewEnabled, isLoadingSupervision } =
    useOptionalSupervision();

  if (status === "loading" || isLoadingSupervision) {
    return <Loading fullPage={false} />;
  }

  // Caregivers (user/teacher role) always have access
  if (isCaregiver(session)) {
    return <>{children}</>;
  }

  // Admins only when the admin_supervision_overview setting is confirmed
  // enabled (i.e. /api/active/supervisors/all returned OK). Checking
  // supervisedRooms.length would incorrectly let admins through when the
  // setting is OFF but a synthetic Schulhof entry is present.
  if (isAdmin(session) && adminOverviewEnabled) {
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
      <Suspense fallback={<Loading fullPage={false} />}>
        <ActiveSupervisionGate>
          <SSEErrorBoundary>
            <MeinRaumPageContent />
          </SSEErrorBoundary>
        </ActiveSupervisionGate>
      </Suspense>
    </BinaryModeGuard>
  );
}
