"use client";

import React, {
  useState,
  useEffect,
  useCallback,
  useMemo,
  Suspense,
  useRef,
} from "react";
import dynamic from "next/dynamic";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { hasPermission } from "~/lib/auth-utils";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  SkeletonRegion,
  CardSkeleton,
  TableSkeleton,
} from "~/components/ui/page-skeletons";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
import { Modal } from "~/components/ui/modal";
import { OriginChip } from "~/components/ui/origin-chip";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import {
  formatSignedDuration,
  ViewToggle,
  type ViewMode,
} from "~/components/staff/staff-time-views";
import { StaffSessionTable } from "~/components/staff/staff-session-table";
import { StaffExportButton } from "~/components/staff/staff-export-button";
import { BetreuungsplanHeuteCard } from "~/components/time-tracking/betreuungsplan-heute-card";
import { Monatskarte } from "~/components/time-tracking/monatskarte";
import { LeaveRequestsCard } from "~/components/time-tracking/leave-requests-card";
import type { StaffHistorySession, StaffAbsenceRow } from "~/lib/staff-api";
import { ownShiftService } from "~/lib/shift-api";
import type { StaffShift } from "~/lib/shift-helpers";
import {
  berlinDayStart,
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { useToast } from "~/contexts/ToastContext";
import {
  usePeriodMetrics,
  type PeriodMetrics,
} from "~/lib/hooks/use-period-metrics";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { staffScheduleService } from "~/lib/staff-api";
import {
  adaptAbsenceForMetrics,
  adaptHistorySessionForMetrics,
  isEffectiveAbsenceStatus,
  isHalfAbsenceBoundary,
  startOfYear,
} from "~/lib/staff-metrics-helpers";
import {
  DEVIATION_REASON_REQUIRED_CODE,
  PLANNED_START_NOT_REACHED_CODE,
  timeTrackingService,
} from "~/lib/time-tracking-api";
import { userContextService } from "~/lib/usercontext-api";
import type { ApiError } from "~/lib/auth-api";
import {
  type MonthSummary,
  type StaffAbsence,
  type WorkSession,
  type WorkSessionBreak,
  type WeeklySummary,
  type WorkSessionHistory,
  absenceDisplayLabel,
  formatDuration,
  formatTime,
  getWeekDays,
  getWeekNumber,
  balanceSessionEnd,
  calculateNetMinutes,
  indexWorkSessionMinutesByBerlinDate,
  OPEN_MONTH_REFRESH_MS,
} from "~/lib/time-tracking-helpers";
import { useAbsenceTypeSelect } from "~/components/staff/use-absence-type-select";
import { absenceRequestFor, selectValueFor } from "~/lib/absence-type-select";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TimeTrackingPage" });

const WeekChart = dynamic(() => import("./week-chart"), {
  ssr: false,
  loading: () => <Skeleton className="min-h-[280px] rounded-2xl" aria-hidden />,
});

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDateGerman(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  const year = date.getFullYear();
  return `${day}.${month}.${year}`;
}

// Extracts error message string from unknown error types
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === "string") return err;
  return "";
}

// Reads the stable machine code of a failed request. `buildApiError` puts it on
// `ApiError.code` and leaves `message` as the human-readable backend text, so
// the code is never in the message; the JSON fallback covers the callers that
// surface the raw response body as the message instead.
function getErrorCode(err: unknown): string | undefined {
  if (err !== null && typeof err === "object" && "code" in err) {
    const code = (err as { code?: unknown }).code;
    if (typeof code === "string" && code !== "") return code;
  }
  return /"code":"([^"]+)"/.exec(getErrorMessage(err))?.[1];
}

// Maps backend error messages to user-friendly German messages
function friendlyError(err: unknown, fallback: string): string {
  const msg = getErrorMessage(err);

  // Stable machine codes win over the human-readable message: their text can
  // carry dynamic content (e.g. the conflicting interval of an overlap).
  const stableCode = getErrorCode(err);
  if (stableCode === "work_session_overlap") {
    return "Der Zeitraum überschneidet sich mit einem anderen Arbeitsblock an diesem Tag.";
  }

  // Extract backend error from nested API error format
  const match = /"error":"([^"]+)"/.exec(msg);
  const code = match?.[1] ?? msg;

  const map: Record<string, string> = {
    "already checked in": "Du bist bereits eingestempelt.",
    "already checked out today": "Du hast heute bereits gearbeitet.",
    "no active session found": "Kein aktiver Eintrag vorhanden.",
    "no session found for today": "Kein Eintrag für heute vorhanden.",
    "session not found": "Eintrag nicht gefunden.",
    "can only update own sessions": "Du kannst nur eigene Einträge bearbeiten.",
    "break already active": "Eine Pause läuft bereits.",
    "no active break found": "Keine aktive Pause vorhanden.",
    "absence not found": "Abwesenheit nicht gefunden.",
    "can only update own absences":
      "Du kannst nur eigene Abwesenheiten bearbeiten.",
    "can only delete own absences":
      "Du kannst nur eigene Abwesenheiten löschen.",
    "invalid absence type": "Ungültiger Abwesenheitstyp.",
  };

  // Exact match first
  if (map[code]) return map[code];

  // Prefix-based matches for messages with dynamic content
  if (
    code.startsWith("absence overlaps") ||
    code.startsWith("updated dates overlap")
  ) {
    return "Für diesen Zeitraum ist bereits eine andere Abwesenheitsart eingetragen.";
  }
  if (code.startsWith("invalid session data:")) {
    return "Bitte prüfe Start- und Endzeit.";
  }
  if (code === "planned start not reached") {
    return "Einstempeln ist noch nicht möglich. Bitte prüfe deine geplante Startzeit.";
  }

  return fallback;
}

const DAY_NAMES_LONG = [
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
  "Samstag",
  "Sonntag",
];

function extractTimeFromISO(isoString: string): string {
  try {
    const date = new Date(isoString);
    if (Number.isNaN(date.getTime())) return "";
    return `${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
  } catch {
    return "";
  }
}

// ─── ClockInCard ──────────────────────────────────────────────────────────────

// Renders today's planned Dienstplan shifts under the Stempeluhr title: each
// active shift with its Schichtart chip, a "Vertretung" tag for replacement
// shifts, plus a muted line for shifts that do not take place ("entfällt"),
// so Dienstplan-level changes are visible to the staff member (#1844).
function PlannedShiftsInfo({
  plannedShifts,
  cancelledShifts,
}: {
  readonly plannedShifts?: readonly StaffShift[];
  readonly cancelledShifts?: readonly StaffShift[];
}) {
  const active = plannedShifts ?? [];
  const cancelled = cancelledShifts ?? [];
  if (active.length === 0 && cancelled.length === 0) return null;
  return (
    <div className="-mt-3 mb-5 space-y-1 text-sm text-gray-500">
      {active.map((shift) => (
        <div key={shift.id} className="flex flex-wrap items-center gap-1.5">
          <span className="text-gray-400">Geplant:</span>
          {shift.shiftTypeName && (
            // Schichtart pill: the color is tenant-defined data the backend
            // embeds (#1844), not a brand hue; without one the pill falls
            // back to the neutral HOME gray.
            <StatusDotBadge
              label={shift.shiftTypeName}
              color={shift.shiftTypeColor ?? LOCATION_COLORS.HOME}
            />
          )}
          <span className="tabular-nums">
            {shift.startTime}–{shift.endTime}
          </span>
          {shift.originShiftId && (
            <span className="bg-moto-blue/10 text-moto-blue rounded-full px-2 py-0.5 text-xs font-medium">
              Vertretung
            </span>
          )}
          {shift.breakMinutes > 0 && (
            <span className="text-gray-400">
              · Pause {shift.breakMinutes} min
            </span>
          )}
        </div>
      ))}
      {cancelled.map((shift) => (
        <div
          key={shift.id}
          className="flex flex-wrap items-center gap-1.5 text-gray-400"
        >
          <span className="tabular-nums line-through">
            {shift.startTime}–{shift.endTime}
          </span>
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
            Entfällt
          </span>
          {shift.changeReason && (
            <span className="italic">{shift.changeReason}</span>
          )}
        </div>
      ))}
    </div>
  );
}

function formatTimeFromDate(date: Date): string {
  return `${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
}

const DEFAULT_BREAK_MINUTES = 30;
const BREAK_STEP_MINUTES = 15;
const MIN_BREAK_MINUTES = 15;
const MAX_BREAK_MINUTES = 240;

// Session status type (only present or home_office - no absent for active sessions)
type SessionStatus = "present" | "home_office";

// Work mode type for clock-in status (includes absent option)
type WorkMode = SessionStatus | "absent";

// Calculate elapsed minutes from a start time to now
function calcElapsedMinutes(startTime: string | Date, now: Date): number {
  return Math.max(
    0,
    Math.floor((now.getTime() - new Date(startTime).getTime()) / 60000),
  );
}

// Calculate elapsed seconds from a start time to now
function calcElapsedSeconds(startTime: string | Date, now: Date): number {
  return Math.max(
    0,
    Math.floor((now.getTime() - new Date(startTime).getTime()) / 1000),
  );
}

// Calculate live break minutes including active break
function calcLiveBreakMins(
  breakMins: number,
  activeBreak: WorkSessionBreak | null,
  now: Date,
): number {
  if (!activeBreak) return breakMins;
  const activeBreakMins = calcElapsedMinutes(activeBreak.startedAt, now);
  return breakMins + activeBreakMins;
}

// Get break compliance warning based on work time and breaks taken
function getBreakWarning(
  netMinutes: number,
  liveBreakMins: number,
): string | null {
  if (netMinutes > 540 && liveBreakMins < 45) {
    return "45 Min Pause ab 9h nötig (§4 ArbZG)";
  }
  if (netMinutes > 360 && liveBreakMins < 30) {
    return "30 Min Pause ab 6h nötig (§4 ArbZG)";
  }
  return null;
}

// Format countdown as MM:SS
function formatCountdown(totalSecs: number): string {
  const m = Math.floor(totalSecs / 60);
  const s = totalSecs % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

// Returns the kit StatusBadge tone and label for the current session state.
function getSessionStatusBadge(
  isOnBreak: boolean,
  status: string | undefined,
): { tone: StatusBadgeTone; label: string } {
  if (isOnBreak) {
    return { tone: "orange", label: "Pause" };
  }
  if (status === "home_office") {
    return { tone: "blue", label: "Homeoffice" };
  }
  return { tone: "green", label: "In der OGS" };
}

// Vor Ort / Homeoffice / Abwesend as a kit SegmentedControl. Tones map to
// LOCATION_COLORS: present → GROUP_ROOM green, home_office → OTHER_ROOM blue,
// absent → HOME red. `value` stays null until the staff member picks one
// (Issue #1368 — no pre-selection on an audit-relevant choice).
const WORK_MODE_ITEMS: readonly SegmentedControlItem<WorkMode>[] = [
  { value: "present", label: "In der OGS", tone: "green" },
  { value: "home_office", label: "Homeoffice", tone: "blue" },
  { value: "absent", label: "Abwesend", tone: "red" },
];

// The circular stamp controls are deliberately larger than any kit Button size
// (64px / 48px touch targets on a tablet mounted in the hallway), so they pass
// their geometry to ui/Button via className while keeping the kit's focus,
// disabled and transition treatment. Colors are brand hexes only.
//
// `!rounded-full` is load-bearing: Button applies a radius per size
// (`rounded-lg` on the page sizes), and Tailwind resolves competing
// border-radius utilities by stylesheet order, not by the order they appear in
// the class attribute. Without the important modifier these buttons render as
// rounded squares. Same modifier as the break stepper below.
const STAMP_BUTTON_BASE =
  "!rounded-full border-2 !p-0 shadow-none hover:shadow-none active:scale-95";

// Returns className for the check-in button. `mode === null` renders muted and
// disabled — the staff member must pick a status first (Issue #1368).
function getCheckInButtonClassName(mode: WorkMode | null): string {
  const base = `${STAMP_BUTTON_BASE} h-16 w-16 disabled:cursor-not-allowed`;
  if (mode === null)
    return `${base} border-[#6B7280]/40 text-[#6B7280]/60 cursor-not-allowed`;
  if (mode === "home_office")
    return `${base} border-moto-blue text-moto-blue hover:bg-moto-blue/5`;
  return `${base} border-moto-green text-moto-green hover:bg-moto-green/5`;
}

// Returns className for the break/pause button. Orange is SCHOOLYARD #F78C10
// from LOCATION_COLORS — the app's warning tone.
function getBreakButtonClassName(
  isOnBreak: boolean,
  breakMins: number,
): string {
  const base = `${STAMP_BUTTON_BASE} h-12 w-12 disabled:opacity-60`;
  if (isOnBreak) return `${base} border-[#F78C10] text-[#F78C10]`;
  if (breakMins > 0)
    return `${base} border-[#F78C10] text-[#F78C10] hover:bg-[#F78C10]/10`;
  return `${base} border-gray-200 text-gray-400 hover:border-gray-300 hover:text-gray-500`;
}

// Returns formatted duration for today footer
function getTodayDisplayValue(
  isCheckedIn: boolean,
  isCheckedOut: boolean,
  netMinutes: number,
  checkedOutNet: number | null,
): string {
  if (isCheckedIn) return formatDuration(netMinutes);
  if (isCheckedOut && checkedOutNet !== null)
    return formatDuration(checkedOutNet);
  return "--";
}

// Gross minutes of the running block that fall on TODAY. A block started
// before midnight keeps running past it, but its earlier minutes belong to
// the previous Berlin day — the page books them there via the Berlin-date
// split, so counting them again here would credit a whole night to today.
//
// `end` is the block's live limit (balanceSessionEnd), never the raw clock: a
// block whose check-out never happened stopped counting there, server-side as
// well, so counting it through `now` would print a live figure the Saldo, the
// Monatskarte and the chart all contradict.
function calcGrossMinutes(
  isCheckedIn: boolean,
  session: WorkSession | null,
  now: Date,
  end: Date,
): number {
  if (!isCheckedIn || !session) return 0;
  const start = new Date(session.checkInTime);
  if (Number.isNaN(start.getTime())) return 0;
  const dayStart = berlinDayStart(now);
  const from = start > dayStart ? start : dayStart;
  if (end <= from) return 0;
  return calcElapsedMinutes(from, end);
}

// Break minutes of the running block that fall inside [from, now]. Used for a
// block that started before midnight: its cached breakMinutes covers the whole
// block, and deducting yesterday's pause from today's live figure would
// understate today's work.
function calcBreakMinutesSince(
  breaks: readonly WorkSessionBreak[],
  from: Date,
  now: Date,
): number {
  let totalMs = 0;
  for (const workBreak of breaks) {
    const start = new Date(workBreak.startedAt).getTime();
    const end = workBreak.endedAt
      ? new Date(workBreak.endedAt).getTime()
      : now.getTime();
    if (Number.isNaN(start) || Number.isNaN(end)) continue;
    const overlap =
      Math.min(end, now.getTime()) - Math.max(start, from.getTime());
    if (overlap > 0) totalMs += overlap;
  }
  return Math.floor(totalMs / 60_000);
}

// Calculate countdown remaining seconds for break timer
// Prefers plannedEndTime from backend (persists across refresh)
// Falls back to plannedMins for immediate feedback before backend responds
function calcCountdownSecs(
  isOnBreak: boolean,
  plannedEndTime: string | null,
  plannedMins: number | null,
  elapsedSecs: number,
  now: Date,
): number | null {
  if (!isOnBreak) return null;

  // Use backend's plannedEndTime if available (survives refresh)
  if (plannedEndTime) {
    const endTime = new Date(plannedEndTime).getTime();
    const remaining = Math.floor((endTime - now.getTime()) / 1000);
    return Math.max(0, remaining);
  }

  // Fallback to local state for immediate feedback
  if (plannedMins === null) return null;
  return Math.max(0, plannedMins * 60 - elapsedSecs);
}

// Check if auto-end break should trigger
function shouldAutoEndBreak(
  countdownSecs: number | null,
  isOnBreak: boolean,
  loading: boolean,
): boolean {
  return countdownSecs !== null && countdownSecs <= 0 && isOnBreak && !loading;
}

// Get break warning only when checked in
function getBreakWarningIfActive(
  isCheckedIn: boolean,
  session: WorkSession | null,
  netMins: number,
  breakMins: number,
): string | null {
  if (!isCheckedIn || !session) return null;
  return getBreakWarning(netMins, breakMins);
}

// Calculate checked-out net minutes
function getCheckedOutNetMins(
  isCheckedOut: boolean,
  session: WorkSession | null,
): number | null {
  if (!isCheckedOut || !session) return null;
  return calculateNetMinutes(
    session.checkInTime,
    session.checkOutTime,
    session.breakMinutes,
  );
}

// Timer display content based on break/work state
function renderTimerContent(
  isOnBreak: boolean,
  countdownRemainingSecs: number | null,
  plannedBreakDurationMins: number | null,
  plannedBreakEndsAt: string | null,
  activeBreakElapsedSecs: number,
  displayMinutes: number,
  breakWarning: string | null,
): React.ReactNode {
  // Break with countdown timer
  if (
    isOnBreak &&
    countdownRemainingSecs !== null &&
    plannedBreakDurationMins
  ) {
    return (
      <>
        <span className="text-4xl font-light text-[#F78C10] tabular-nums">
          {formatCountdown(countdownRemainingSecs)}
        </span>
        <span className="mt-0.5 text-xs font-medium text-[#F78C10]">
          Pause ({plannedBreakDurationMins} Min)
        </span>
        <span className="text-moto-amber-strong mt-0.5 text-center text-xs font-medium">
          {plannedBreakEndsAt
            ? `Automatisch weiter um ${formatTime(plannedBreakEndsAt)}`
            : "Automatisch weiter nach der Pause"}
        </span>
      </>
    );
  }
  // Break without countdown (manual)
  if (isOnBreak) {
    return (
      <>
        <span className="text-4xl font-light text-[#F78C10] tabular-nums">
          {formatCountdown(activeBreakElapsedSecs)}
        </span>
        <span className="mt-0.5 text-xs font-medium text-[#F78C10]">
          Pause läuft
        </span>
      </>
    );
  }
  // Working time display
  return (
    <>
      <span className="text-4xl font-light text-gray-900 tabular-nums">
        {formatDuration(displayMinutes)}
      </span>
      {breakWarning && (
        <span className="text-moto-amber-strong mt-0.5 text-xs font-medium">
          {breakWarning}
        </span>
      )}
    </>
  );
}

function ClockInCard({
  currentSession,
  breaks,
  onCheckIn,
  onCheckOut,
  onStartBreak,
  onEndBreak,
  weeklyMinutes,
  completedTodayMinutes,
  completedTodayBreakMinutes,
  onAddAbsence,
  metrics,
  plannedShifts,
  cancelledShifts,
}: {
  readonly currentSession: WorkSession | null;
  readonly breaks: WorkSessionBreak[];
  readonly onCheckIn: (status: SessionStatus) => Promise<void>;
  readonly onCheckOut: () => Promise<void>;
  readonly onStartBreak: (durationMinutes: number) => Promise<void>;
  readonly onEndBreak: () => Promise<void>;
  readonly weeklyMinutes: number;
  readonly completedTodayMinutes: number;
  readonly completedTodayBreakMinutes: number;
  readonly onAddAbsence: () => void;
  readonly metrics?: PeriodMetrics | null;
  readonly plannedShifts?: readonly StaffShift[];
  readonly cancelledShifts?: readonly StaffShift[];
}) {
  // Null until the staff member explicitly picks Vor Ort / Homeoffice / Abwesend.
  // No pre-selection per Issue #1368 — silent defaults are unacceptable for an
  // audit-relevant choice.
  const [mode, setMode] = useState<WorkMode | null>(null);
  const [actionLoading, setActionLoading] = useState(false);
  const [tick, setTick] = useState(0); // forces re-render for live times
  const [breakMenuOpen, setBreakMenuOpen] = useState(false);
  const [plannedBreakMinutes, setPlannedBreakMinutes] = useState<number | null>(
    null,
  );
  const [individualBreakMinutes, setIndividualBreakMinutes] = useState(
    DEFAULT_BREAK_MINUTES,
  );
  const [isMobileViewport, setIsMobileViewport] = useState(false);
  // Mutex to prevent race condition in auto-end break effect
  const autoEndInFlightRef = useRef(false);
  // Anchor for the desktop break-duration popover; dismissal is handled by
  // document listeners scoped to this subtree (see effect below).
  const breakMenuRef = useRef<HTMLDivElement>(null);

  const isCheckedIn =
    currentSession !== null && currentSession.checkOutTime === null;
  const isCheckedOut =
    currentSession !== null && currentSession.checkOutTime !== null;

  // Derive active break from breaks array
  const activeBreak = breaks.find((b) => !b.endedAt) ?? null;
  const isOnBreak = activeBreak !== null;

  // Tick every second during break (for the seconds countdown) and every
  // 15s otherwise. 15s is fast enough that the "5h 29min" timer flips to
  // "5h 30min" within seconds of the actual minute change, but slow enough
  // to avoid pointless re-renders.
  useEffect(() => {
    if (!isCheckedIn) return;
    const interval = setInterval(
      () => setTick((t) => t + 1),
      isOnBreak ? 1000 : 15000,
    );
    return () => clearInterval(interval);
  }, [isCheckedIn, isOnBreak]);

  // Use `tick` to make `now` reactive (tick triggers re-render)
  // eslint-disable-next-line no-unused-expressions
  tick;
  const now = new Date();

  const breakMins = currentSession?.breakMinutes ?? 0;
  // Same midnight boundary as calcGrossMinutes: for an overnight block the
  // pauses are re-derived from the break rows so only today's share is
  // deducted from today's live minutes.
  const dayStart = berlinDayStart(now);
  const startedBeforeToday =
    isCheckedIn &&
    currentSession !== null &&
    new Date(currentSession.checkInTime).getTime() < dayStart.getTime();

  // The block's own end: `now` while it is running, its live limit once it has
  // passed it. Work AND break minutes are measured against this single instant
  // so the live figure cannot drift away from the server-side Saldo, which
  // caps the very same block (balanceSessionEnd).
  const liveEnd =
    isCheckedIn && currentSession !== null
      ? balanceSessionEnd(
          {
            date: currentSession.date,
            checkIn: new Date(currentSession.checkInTime),
            checkOut: null,
          },
          now,
        )
      : now;

  // Calculate derived time values using extracted helpers
  const liveBreakMins = startedBeforeToday
    ? calcBreakMinutesSince(breaks, dayStart, liveEnd)
    : calcLiveBreakMins(breakMins, activeBreak, liveEnd);
  const grossMinutes = calcGrossMinutes(
    isCheckedIn,
    currentSession,
    now,
    liveEnd,
  );
  const netMinutes = Math.max(0, grossMinutes - liveBreakMins);
  const dayNetMinutes = completedTodayMinutes + netMinutes;
  const dayBreakMinutes = completedTodayBreakMinutes + liveBreakMins;
  const displayMinutes = isCheckedIn ? dayNetMinutes : 0;
  const activeBreakElapsedSecs = activeBreak
    ? calcElapsedSeconds(activeBreak.startedAt, now)
    : 0;
  const countdownRemainingSecs = calcCountdownSecs(
    isOnBreak,
    activeBreak?.plannedEndTime ?? null,
    plannedBreakMinutes,
    activeBreakElapsedSecs,
    now,
  );

  // Calculate planned break duration from backend or local state
  // Backend: plannedEndTime - startedAt, Local: plannedBreakMinutes (for immediate feedback)
  const plannedBreakDurationMins: number | null = (() => {
    if (activeBreak?.plannedEndTime && activeBreak.startedAt) {
      const start = new Date(activeBreak.startedAt).getTime();
      const end = new Date(activeBreak.plannedEndTime).getTime();
      return Math.round((end - start) / 60000);
    }
    return plannedBreakMinutes;
  })();

  // Auto-end break when countdown reaches 0
  useEffect(() => {
    if (
      shouldAutoEndBreak(countdownRemainingSecs, isOnBreak, actionLoading) &&
      !autoEndInFlightRef.current
    ) {
      autoEndInFlightRef.current = true;
      void (async () => {
        setActionLoading(true);
        try {
          await onEndBreak();
          setPlannedBreakMinutes(null);
        } finally {
          setActionLoading(false);
          autoEndInFlightRef.current = false;
        }
      })();
    }
  }, [countdownRemainingSecs, isOnBreak, actionLoading, onEndBreak]);

  // Clear planned break when break ends externally
  useEffect(() => {
    if (!isOnBreak) {
      setPlannedBreakMinutes(null);
    }
  }, [isOnBreak]);

  // Dismiss the desktop break-duration popover on outside click or Escape.
  // Capture the outside click before its target can act, so closing the picker
  // cannot also trigger a button behind it. The mobile picker is portaled by
  // Drawer, so it manages dismissal through its own open state instead.
  useEffect(() => {
    if (!breakMenuOpen || isMobileViewport) return;
    const onOutsideClick = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (target === null) return;
      if (breakMenuRef.current?.contains(target)) return;
      event.preventDefault();
      event.stopPropagation();
      setBreakMenuOpen(false);
    };
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") setBreakMenuOpen(false);
    };
    document.addEventListener("click", onOutsideClick, true);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("click", onOutsideClick, true);
      document.removeEventListener("keydown", onKey);
    };
  }, [breakMenuOpen, isMobileViewport]);

  useEffect(() => {
    const updateViewport = () => {
      setIsMobileViewport(window.innerWidth < 640);
    };
    updateViewport();
    window.addEventListener("resize", updateViewport);
    return () => window.removeEventListener("resize", updateViewport);
  }, []);

  // Break compliance warning (only shown when checked in)
  const breakWarning = getBreakWarningIfActive(
    isCheckedIn,
    currentSession,
    dayNetMinutes,
    dayBreakMinutes,
  );

  // Checked-out net
  const checkedOutNet = getCheckedOutNetMins(isCheckedOut, currentSession);

  // Session status badge (computed once, not in IIFE)
  const statusBadge =
    isCheckedIn && currentSession
      ? getSessionStatusBadge(isOnBreak, currentSession.status)
      : null;

  const handleCheckIn = async () => {
    // mode === null: user has not yet chosen — the button is disabled in the
    // UI, but guard here too in case of stray double-clicks.
    // mode === "absent": that path opens the absence modal, not check-in.
    if (mode === null || mode === "absent") return;
    setActionLoading(true);
    try {
      await onCheckIn(mode);
    } finally {
      setActionLoading(false);
    }
  };

  const handleCheckOut = async () => {
    setActionLoading(true);
    try {
      await onCheckOut();
    } finally {
      setActionLoading(false);
    }
  };

  const handleSelectBreakDuration = async (minutes: number) => {
    setBreakMenuOpen(false);
    setPlannedBreakMinutes(minutes);
    setActionLoading(true);
    try {
      await onStartBreak(minutes);
    } finally {
      setActionLoading(false);
    }
  };

  const handleBreakStepperChange = (direction: 1 | -1) => {
    setIndividualBreakMinutes((current) =>
      Math.min(
        MAX_BREAK_MINUTES,
        Math.max(MIN_BREAK_MINUTES, current + direction * BREAK_STEP_MINUTES),
      ),
    );
  };

  const handleEndBreakEarly = async () => {
    setActionLoading(true);
    try {
      await onEndBreak();
      setPlannedBreakMinutes(null);
    } finally {
      setActionLoading(false);
    }
  };

  const showBreakDurationPicker = breakMenuOpen && !isOnBreak;
  const breakDurationStepper = (
    <div className="mx-auto grid w-full max-w-xs grid-cols-[4.5rem_1fr_4.5rem] items-center gap-1 sm:flex sm:max-w-sm sm:justify-center sm:gap-5">
      <Button
        type="button"
        size="icon"
        variant="ghost"
        aria-label="Pausendauer verringern"
        disabled={actionLoading || individualBreakMinutes <= MIN_BREAK_MINUTES}
        onClick={() => handleBreakStepperChange(-1)}
        className="h-[4.5rem] w-[4.5rem] !rounded-[1.75rem] border border-gray-200 shadow-none sm:h-12 sm:w-12 sm:!rounded-xl"
      >
        <svg
          className="h-9 w-9 sm:h-7 sm:w-7"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
          aria-hidden="true"
        >
          <path d="M5 12h14" />
        </svg>
      </Button>
      <div className="flex h-20 min-w-32 items-center justify-center px-3 sm:h-14 sm:min-w-24">
        <span className="text-5xl font-semibold text-gray-900 tabular-nums sm:text-3xl">
          {individualBreakMinutes}
        </span>
        <span className="ml-2 text-lg font-medium text-gray-500 sm:text-sm">
          min
        </span>
      </div>
      <Button
        type="button"
        size="icon"
        variant="ghost"
        aria-label="Pausendauer erhöhen"
        disabled={actionLoading || individualBreakMinutes >= MAX_BREAK_MINUTES}
        onClick={() => handleBreakStepperChange(1)}
        className="h-[4.5rem] w-[4.5rem] !rounded-[1.75rem] border border-gray-200 shadow-none sm:h-12 sm:w-12 sm:!rounded-xl"
      >
        <svg
          className="h-9 w-9 sm:h-7 sm:w-7"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
          aria-hidden="true"
        >
          <path d="M12 5v14" />
          <path d="M5 12h14" />
        </svg>
      </Button>
    </div>
  );
  const breakStartButton = (
    <Button
      type="button"
      size="md"
      variant="primary"
      disabled={actionLoading}
      onClick={() => void handleSelectBreakDuration(individualBreakMinutes)}
      className="mx-auto w-full max-w-sm text-sm shadow-none"
    >
      Starten
    </Button>
  );
  const breakDurationControls = (
    <>
      {breakDurationStepper}
      <Button
        type="button"
        size="md"
        variant="primary"
        disabled={actionLoading}
        onClick={() => void handleSelectBreakDuration(individualBreakMinutes)}
        className="mx-auto mt-6 w-full max-w-sm text-sm shadow-none"
      >
        Starten
      </Button>
    </>
  );

  return (
    <div className="moto-content-surface relative overflow-hidden rounded-2xl border shadow-sm">
      <div className="relative p-4 sm:p-6">
        {/* Title + status badge */}
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-base font-bold text-gray-900 sm:text-lg">
            Stempeluhr
          </h2>
          {statusBadge && (
            <StatusBadge label={statusBadge.label} tone={statusBadge.tone} />
          )}
        </div>

        {/* Heute geplante Schichten (Dienstplan) — dezente Zeilen unter dem
            Titel: Schichtart als farbiger Chip, Vertretungen und entfallene
            Schichten sichtbar (#1844). Geteilte Dienste zeigen jede Schicht. */}
        <PlannedShiftsInfo
          plannedShifts={plannedShifts}
          cancelledShifts={cancelledShifts}
        />

        {/* ── Not checked in: start controls ── */}
        {!isCheckedIn && !isCheckedOut && (
          <div className="flex flex-col items-center gap-5">
            {/* Mode toggle */}
            <SegmentedControl
              ariaLabel="Arbeitsmodus"
              variant="pills"
              items={WORK_MODE_ITEMS}
              value={mode}
              onChange={setMode}
            />

            {/* Action button: play (check-in) or calendar (absence) */}
            {mode === "absent" ? (
              <Button
                type="button"
                variant="ghost"
                onClick={onAddAbsence}
                className={`${STAMP_BUTTON_BASE} h-16 w-16 border-[#FF3130] text-[#FF3130] hover:bg-[#FF3130]/5`}
                aria-label="Abwesenheit melden"
              >
                <svg
                  className="h-7 w-7"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={1.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 012.25-2.25h13.5A2.25 2.25 0 0121 7.5v11.25m-18 0A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75m-18 0v-7.5A2.25 2.25 0 015.25 9h13.5A2.25 2.25 0 0121 11.25v7.5"
                  />
                </svg>
              </Button>
            ) : (
              <Button
                type="button"
                variant="ghost"
                onClick={handleCheckIn}
                disabled={actionLoading || mode === null}
                className={getCheckInButtonClassName(mode)}
                aria-label="Einstempeln"
              >
                {actionLoading ? (
                  <div className="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" />
                ) : (
                  <svg
                    className="ml-0.5 h-7 w-7"
                    fill="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path d="M8 5v14l11-7z" />
                  </svg>
                )}
              </Button>
            )}

            <span className="text-xs text-gray-400 sm:text-sm">
              {mode === null
                ? "Bitte Status wählen"
                : mode === "absent"
                  ? "Abwesenheit melden"
                  : "Einstempeln"}
            </span>
          </div>
        )}

        {/* ── Checked in: timer controls ── */}
        {isCheckedIn && (
          <>
            {/* Control row: pause | timer | stop */}
            <div className="mb-5 flex items-center justify-between">
              {/* Pause button (opens menu) or End Break button */}
              <div className="relative" ref={breakMenuRef}>
                {isOnBreak ? (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={handleEndBreakEarly}
                    disabled={actionLoading}
                    className={getBreakButtonClassName(true, breakMins)}
                    aria-label="Pause beenden"
                  >
                    <svg
                      className="h-5 w-5"
                      fill="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path d="M8 5v14l11-7z" />
                    </svg>
                  </Button>
                ) : (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() => setBreakMenuOpen(!breakMenuOpen)}
                    disabled={actionLoading}
                    className={getBreakButtonClassName(false, breakMins)}
                    aria-label="Pause starten"
                    aria-expanded={breakMenuOpen}
                    aria-haspopup="dialog"
                  >
                    <svg
                      className="h-5 w-5"
                      fill="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
                    </svg>
                  </Button>
                )}

                {/* The capture-phase listener above consumes the first desktop
                    outside click, so dismissing this picker cannot also
                    trigger an action behind it. */}
                {showBreakDurationPicker && !isMobileViewport && (
                  <div className="moto-popover-surface absolute top-full left-0 z-20 mt-2 w-[calc(100vw-2rem)] max-w-72 rounded-xl border p-3 sm:w-72">
                    {breakDurationControls}
                  </div>
                )}
              </div>

              {showBreakDurationPicker && isMobileViewport && (
                <Drawer
                  open
                  onOpenChange={(next) => {
                    if (!next) setBreakMenuOpen(false);
                  }}
                  direction="bottom"
                  shouldScaleBackground
                >
                  <DrawerContent
                    aria-label="Pause stempeln"
                    aria-describedby={undefined}
                    className="max-h-[calc(100vh-7rem)] min-h-[22rem] overflow-visible px-5 pb-[calc(env(safe-area-inset-bottom)+1rem)]"
                  >
                    <DrawerHeader className="flex items-center justify-between px-0 pt-4 pb-4 text-left">
                      <DrawerTitle>Pause stempeln</DrawerTitle>
                      <DrawerClose
                        type="button"
                        aria-label="Pausendauer schließen"
                        className="flex h-10 w-10 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-900"
                      >
                        <svg
                          className="h-5 w-5"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          aria-hidden="true"
                        >
                          <path d="M6 6l12 12" />
                          <path d="M18 6L6 18" />
                        </svg>
                      </DrawerClose>
                    </DrawerHeader>
                    <div className="flex min-h-56 flex-1 flex-col pt-2 pb-1">
                      <div className="flex flex-1 items-start pt-8">
                        {breakDurationStepper}
                      </div>
                      {breakStartButton}
                    </div>
                  </DrawerContent>
                </Drawer>
              )}

              {/* Timer display */}
              <div className="flex flex-col items-center">
                {renderTimerContent(
                  isOnBreak,
                  countdownRemainingSecs,
                  plannedBreakDurationMins,
                  activeBreak?.plannedEndTime ?? null,
                  activeBreakElapsedSecs,
                  displayMinutes,
                  breakWarning,
                )}
              </div>

              {/* Stop / check-out button */}
              <Button
                type="button"
                variant="ghost"
                onClick={handleCheckOut}
                disabled={actionLoading || isOnBreak}
                className={`${STAMP_BUTTON_BASE} h-12 w-12 border-gray-300 text-gray-500 hover:border-[#FF3130] hover:text-[#FF3130] disabled:opacity-50`}
                aria-label="Ausstempeln"
              >
                {actionLoading ? (
                  <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                ) : (
                  <svg
                    className="h-4 w-4"
                    fill="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <rect x="6" y="6" width="12" height="12" rx="1" />
                  </svg>
                )}
              </Button>
            </div>

            {/* ── Activity log rows ── */}
            <div className="border-t border-gray-100">
              <BreakActivityLog
                checkInTime={currentSession?.checkInTime ?? ""}
                breaks={breaks}
                isCheckedIn={isCheckedIn}
                plannedBreakMinutes={plannedBreakMinutes}
              />
            </div>
          </>
        )}

        {/* ── Checked out: summary rows ── */}
        {isCheckedOut && currentSession && (
          <div className="border-t border-gray-100">
            <div className="flex items-center justify-between py-2.5 text-sm">
              <span className="w-14 shrink-0 font-medium text-gray-700">
                Arbeit
              </span>
              <span className="w-12 shrink-0 text-center text-gray-600 tabular-nums">
                {formatTime(currentSession.checkInTime)}
              </span>
              <span className="shrink-0 text-gray-300">&rarr;</span>
              <span className="w-12 shrink-0 text-center text-gray-600 tabular-nums">
                {formatTime(currentSession.checkOutTime)}
              </span>
              <span className="w-16 shrink-0 text-right font-medium text-gray-700 tabular-nums">
                {checkedOutNet === null ? "--" : formatDuration(checkedOutNet)}
              </span>
            </div>

            {currentSession.breakMinutes > 0 && (
              <div className="flex items-center justify-between border-t border-gray-50 py-2.5 text-sm">
                <span className="w-14 shrink-0 font-medium text-gray-500">
                  Pause
                </span>
                <span className="w-12 shrink-0" />
                <span className="shrink-0 opacity-0">&rarr;</span>
                <span className="w-12 shrink-0" />
                <span className="w-16 shrink-0 text-right text-gray-500 tabular-nums">
                  {formatDuration(currentSession.breakMinutes)}
                </span>
              </div>
            )}
          </div>
        )}

        {/* Stats footer — three compact inline stats (Diese Woche / Monat /
            Saldo). Falls metrics fehlt, fallback auf die alte Heute/Woche-Zeile
            damit die Card defensiv lesbar bleibt. */}
        {metrics ? (
          <div className="mt-5 border-t border-gray-100 pt-4">
            <ClockInStatsStrip metrics={metrics} />
            {/* Herkunfts-Chip am Soll-Wert (Planung-Redesign, docs/04 6.2):
                genau einer pro Oberfläche, solange die Soll-Quellen-Frage
                offen ist. */}
            <div className="mt-3 flex justify-end">
              <OriginChip
                label="Soll aus Arbeitszeitmodell"
                title="Wochensaldo und Stundenkonto rechnen gegen das im Arbeitszeitmodell hinterlegte Soll."
              />
            </div>
          </div>
        ) : (
          <div className="mt-4 flex flex-col gap-y-1 text-xs text-gray-400 sm:flex-row sm:flex-wrap sm:gap-x-6">
            <span>
              Heute:{" "}
              <span className="font-medium text-gray-600">
                {getTodayDisplayValue(
                  isCheckedIn,
                  isCheckedOut,
                  netMinutes,
                  checkedOutNet,
                )}
              </span>
            </span>
            <span>
              Woche:{" "}
              <span className="font-medium text-gray-600">
                {formatDuration(weeklyMinutes + (isCheckedIn ? netMinutes : 0))}
              </span>
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

function ClockInStatsStrip({
  metrics,
}: {
  // Every figure is date-valid, straight from the backend month model
  // (usePeriodMetrics, #1842). Never computeStaffMetrics: that one prices
  // historical days at the CURRENT schedule and contradicts the Monatskarte
  // further down the page after a contract change. null = loading.
  readonly metrics: PeriodMetrics;
}) {
  const { week, month, accountBalanceMinutes } = metrics;

  const weekPct = week && week.soll > 0 ? (week.ist / week.soll) * 100 : 0;
  const monthPct = month && month.soll > 0 ? (month.ist / month.soll) * 100 : 0;

  const accountTone: StatusTone =
    accountBalanceMinutes === null
      ? "gray"
      : accountBalanceMinutes > 0
        ? "amber"
        : accountBalanceMinutes < -60
          ? "gray"
          : "green";

  return (
    <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-3 sm:gap-4">
      <InlineStat
        label="Diese Woche"
        primary={week === null ? "–" : formatDuration(week.ist)}
        secondary={
          week === null ? undefined : `von ${formatDuration(week.soll)}`
        }
        progressPct={week === null ? undefined : weekPct}
        tone="green"
      />
      <InlineStat
        label="Dieser Monat"
        primary={month === null ? "–" : formatDuration(month.ist)}
        secondary={
          month === null ? undefined : `von ${formatDuration(month.soll)}`
        }
        progressPct={month === null ? undefined : monthPct}
        tone="green"
      />
      <InlineStat
        label="Stundenkonto"
        primary={
          accountBalanceMinutes === null
            ? "–"
            : formatSignedDuration(accountBalanceMinutes)
        }
        secondary={`seit ${metrics.accountStart.toLocaleDateString("de-DE", {
          timeZone: "Europe/Berlin",
          day: "numeric",
          month: "short",
        })}`}
        tone={accountTone}
      />
    </div>
  );
}

function InlineStat({
  label,
  primary,
  secondary,
  progressPct,
  tone,
}: {
  readonly label: string;
  readonly primary: string;
  readonly secondary?: string;
  readonly progressPct?: number;
  readonly tone: StatusTone;
}) {
  return (
    <div className="text-center">
      <div className="flex items-baseline justify-center gap-2">
        <span className="text-[10px] font-semibold tracking-wider text-gray-400 uppercase sm:text-[11px]">
          {label}
        </span>
        {progressPct !== undefined && (
          <span className="text-[10px] text-gray-400">
            {Math.round(progressPct)}%
          </span>
        )}
      </div>
      <p className={`mt-1 text-base font-bold sm:text-lg ${STATUS_TEXT[tone]}`}>
        {primary}
      </p>
      {secondary && (
        <p className="mt-0.5 text-[10px] text-gray-500 sm:text-[11px]">
          {secondary}
        </p>
      )}
      {progressPct !== undefined && (
        <div className="mx-auto mt-1.5 h-1 max-w-[180px] overflow-hidden rounded-full bg-gray-100">
          <div
            className={`h-full rounded-full transition-all ${STATUS_BAR[tone]}`}
            style={{ width: `${Math.min(100, Math.max(0, progressPct))}%` }}
          />
        </div>
      )}
    </div>
  );
}

// ─── BreakActivityLog ────────────────────────────────────────────────────────

function BreakActivityLog({
  checkInTime,
  breaks,
  isCheckedIn,
  plannedBreakMinutes,
}: {
  readonly checkInTime: string;
  readonly breaks: WorkSessionBreak[];
  readonly isCheckedIn: boolean;
  readonly plannedBreakMinutes?: number | null;
}) {
  const now = new Date();

  // Build timeline segments: work → break → work → break → work...
  const segments: Array<{
    type: "work" | "break";
    start: Date;
    end: Date | null;
    durationMinutes: number;
    isActive: boolean;
  }> = [];

  const checkIn = new Date(checkInTime);
  let cursor = checkIn;

  for (const brk of breaks) {
    const breakStart = new Date(brk.startedAt);
    const breakEnd = brk.endedAt ? new Date(brk.endedAt) : null;

    // Work segment before this break
    if (breakStart > cursor) {
      const workDuration = Math.max(
        0,
        Math.floor((breakStart.getTime() - cursor.getTime()) / 60000),
      );
      segments.push({
        type: "work",
        start: cursor,
        end: breakStart,
        durationMinutes: workDuration,
        isActive: false,
      });
    }

    // Break segment
    const isActiveBreak = !breakEnd;
    const breakDuration = (() => {
      if (breakEnd) return brk.durationMinutes;
      if (isActiveBreak && plannedBreakMinutes) return plannedBreakMinutes;
      return Math.max(
        0,
        Math.floor((now.getTime() - breakStart.getTime()) / 60000),
      );
    })();
    segments.push({
      type: "break",
      start: breakStart,
      end: breakEnd,
      durationMinutes: breakDuration,
      isActive: isActiveBreak,
    });

    cursor = breakEnd ?? now;
  }

  // Final work segment after last break (if checked in and not on active break)
  const lastBreak = breaks.at(-1);
  const hasActiveBreak = lastBreak && !lastBreak.endedAt;
  if (isCheckedIn && !hasActiveBreak) {
    const workDuration = Math.max(
      0,
      Math.floor((now.getTime() - cursor.getTime()) / 60000),
    );
    segments.push({
      type: "work",
      start: cursor,
      end: null,
      durationMinutes: workDuration,
      isActive: true,
    });
  }

  // If no breaks at all, show single work row
  if (segments.length === 0 && isCheckedIn) {
    const workDuration = Math.max(
      0,
      Math.floor((now.getTime() - checkIn.getTime()) / 60000),
    );
    segments.push({
      type: "work",
      start: checkIn,
      end: null,
      durationMinutes: workDuration,
      isActive: true,
    });
  }

  const MAX_VISIBLE = 3;
  const [expanded, setExpanded] = useState(false);
  const hasHidden = segments.length > MAX_VISIBLE;
  const hiddenSegments = hasHidden ? segments.slice(0, -MAX_VISIBLE) : [];
  const visibleSegments = hasHidden ? segments.slice(-MAX_VISIBLE) : segments;

  // Returns row text color class based on segment type and state
  const getSegmentRowColor = (seg: {
    type: "work" | "break";
    isActive: boolean;
  }) => {
    if (seg.type === "break" && seg.isActive) return "text-moto-amber-strong";
    if (seg.type === "break") return "text-gray-500";
    return "";
  };

  // Returns label text color class based on segment type and state
  const getSegmentLabelColor = (seg: {
    type: "work" | "break";
    isActive: boolean;
  }) => {
    if (seg.type === "work" && seg.isActive) return "text-moto-green";
    if (seg.type === "work") return "text-gray-600";
    return "";
  };

  // Returns time text color class based on segment type and state
  const getSegmentTimeColor = (seg: {
    type: "work" | "break";
    isActive: boolean;
  }) => {
    if (seg.type === "work" && seg.isActive) return "text-[#70b525]";
    if (seg.type === "work") return "text-gray-600";
    return "";
  };

  const renderRow = (
    seg: (typeof segments)[number],
    index: number,
    showBorder: boolean,
  ) => (
    <div
      key={`${seg.type}-${seg.start.toISOString()}`}
      className={`flex items-center justify-between py-2.5 text-sm ${
        showBorder ? "border-t border-gray-50" : ""
      } ${getSegmentRowColor(seg)}`}
    >
      <span
        className={`w-14 shrink-0 font-medium ${getSegmentLabelColor(seg)}`}
      >
        {seg.type === "work" ? "Arbeitszeit" : "Pause"}
      </span>
      <span
        className={`w-12 shrink-0 text-center tabular-nums ${getSegmentTimeColor(seg)}`}
      >
        {formatTimeFromDate(seg.start)}
      </span>
      <span
        className={`shrink-0 ${seg.type === "break" && seg.isActive ? "text-[#F78C10]/60" : "text-gray-300"}`}
      >
        &rarr;
      </span>
      <span className="w-12 shrink-0 text-center tabular-nums">
        {seg.end ? formatTimeFromDate(seg.end) : "···"}
      </span>
      <span
        className={`w-16 shrink-0 text-right tabular-nums ${
          seg.type === "work" ? "font-medium text-gray-700" : ""
        }`}
      >
        {formatDuration(seg.durationMinutes)}
      </span>
    </div>
  );

  return (
    <>
      {/* Collapsible older segments */}
      {hasHidden && (
        <div
          className="grid transition-[grid-template-rows] duration-300 ease-in-out"
          style={{
            gridTemplateRows: expanded ? "1fr" : "0fr",
          }}
        >
          <div className="overflow-hidden">
            {hiddenSegments.map((seg, i) => renderRow(seg, i, i > 0))}
          </div>
        </div>
      )}
      {/* Always-visible recent segments */}
      {visibleSegments.map((seg, i) => {
        const globalIndex = hiddenSegments.length + i;
        return renderRow(seg, globalIndex, i > 0 || hasHidden);
      })}
      {/* Toggle button at the bottom */}
      {hasHidden && (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() => setExpanded(!expanded)}
          aria-expanded={expanded}
          className="w-full !rounded-none border-t border-gray-50 text-gray-400 hover:bg-transparent hover:text-gray-600"
        >
          {expanded
            ? "Weniger anzeigen"
            : `${hiddenSegments.length} weitere anzeigen`}
        </Button>
      )}
    </>
  );
}

// ─── StaffStatusPanel ────────────────────────────────────────────────────────
// Kompakte Status-Sidebar neben der Stempeluhr. Eine Card mit drei Zeilen
// statt drei einzelner Karten: Diese Woche / Dieser Monat / Stundenkonto.

type StatusTone = "green" | "amber" | "gray";

// Standalone value text on a white card, so it follows the app's tone map —
// the same one KpiCard (staff-time-views), detail-modal-components and
// school-overview-section use, so the identical Saldo reads the same on
// /staff/[id] and here. The kit's darkened orange (#8A5600) is deliberately
// NOT used: it is a foreground for TINTED surfaces (Alert warning, StatusBadge
// orange) and turns brown on white.
const STATUS_TEXT: Record<StatusTone, string> = {
  green: "text-moto-green",
  amber: "text-moto-amber-strong",
  gray: "text-gray-700",
};

const STATUS_BAR: Record<StatusTone, string> = {
  green: "bg-[#83CD2D]",
  amber: "bg-[#F78C10]",
  gray: "bg-gray-400",
};

// OwnZeiterfassungSection — wrappt die StaffSessionTable mit RangeNav und
// Card-Chrome, in derselben Struktur wie auf /staff/[id]. Drift zwischen
// MA-Sicht und Admin-Sicht wird so eliminiert. Die externe onEditDay-Logik
// bleibt aktiv damit der existing EditSessionModal (inkl. Absence-Edit)
// weiter genutzt wird.
function OwnZeiterfassungSection({
  ownStaffId,
  schedule,
  onEditDay,
}: {
  readonly ownStaffId: string | null;
  readonly schedule: import("~/lib/staff-api").StaffSchedule | null;
  readonly onEditDay: (
    date: Date,
    session: WorkSessionHistory | null,
    absence: StaffAbsence | null,
  ) => void;
}) {
  // The Berlin day, not the browser's, and re-rendered on the rollover: this
  // section stays mounted all day, and `new Date()` frozen at mount would keep
  // "Diesen Monat" and the open-month poll pointing at yesterday's month after
  // midnight — and at the wrong month entirely from a non-Berlin browser
  // (#1842). The backend derives its month from timezone.TodayDate().
  const todayISO = useBerlinToday();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);
  const [viewMode, setViewMode] = useState<ViewMode>("week");
  const [weekAnchor, setWeekAnchor] = useState<Date>(() => {
    const d = parseISODate(berlinTodayISO());
    const day = (d.getDay() + 6) % 7; // Mon = 0
    d.setDate(d.getDate() - day);
    return d;
  });
  const [monthAnchor, setMonthAnchor] = useState<Date>(() => {
    const d = parseISODate(berlinTodayISO());
    d.setDate(1);
    return d;
  });

  const visibleFrom = useMemo(() => {
    if (viewMode === "month") {
      return new Date(monthAnchor.getFullYear(), monthAnchor.getMonth(), 1);
    }
    return new Date(weekAnchor);
  }, [viewMode, monthAnchor, weekAnchor]);

  const visibleTo = useMemo(() => {
    if (viewMode === "month") {
      return new Date(monthAnchor.getFullYear(), monthAnchor.getMonth() + 1, 0);
    }
    const e = new Date(weekAnchor);
    e.setDate(e.getDate() + 6);
    return e;
  }, [viewMode, monthAnchor, weekAnchor]);

  const visibleFromKey = toISODate(visibleFrom);
  const visibleToKey = toISODate(visibleTo);
  // One day of head start, shared with usePeriodMetrics so SWR dedupes the two
  // fetches into one request. It is NOT what makes a block starting before the
  // range visible — /history bounds `from` against check_out_time, so every
  // block reaching into the range comes back however early it started
  // (TestWorkSessionRepository_ListOverlappingByStaffID_KeepsEarlierStarts).
  const tableHistoryFromKey = useMemo(() => {
    const previousDay = new Date(visibleFrom);
    previousDay.setDate(previousDay.getDate() - 1);
    return toISODate(previousDay);
  }, [visibleFrom]);

  // Dedicated table-range fetches. Independent from the WeekChart's
  // 10-workday history fetch, so navigating the table (especially in
  // month mode) does not enlarge the chart's data window.
  const { data: tableData, isLoading: tableLoading } = useSWRAuth<{
    sessions: WorkSessionHistory[];
    weeklySummaries: WeeklySummary[];
  }>(
    `time-tracking-table-${tableHistoryFromKey}-${visibleToKey}`,
    () => timeTrackingService.getHistory(tableHistoryFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );
  const tableHistory = useMemo(() => tableData?.sessions ?? [], [tableData]);

  const {
    data: tableAbsenceData,
    isLoading: tableAbsencesLoading,
    error: tableAbsencesError,
  } = useSWRAuth<StaffAbsence[]>(
    `time-tracking-table-absences-${visibleFromKey}-${visibleToKey}`,
    () => timeTrackingService.getAbsences(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );
  const tableAbsences = useMemo(
    () => tableAbsenceData ?? [],
    [tableAbsenceData],
  );

  const adaptedSessions = useMemo<readonly StaffHistorySession[]>(
    () => tableHistory.map(adaptHistorySessionForMetrics),
    [tableHistory],
  );
  const adaptedAbsences = useMemo<readonly StaffAbsenceRow[]>(
    () => tableAbsences.map(adaptAbsenceForMetrics),
    [tableAbsences],
  );

  // Own Dienstplan shifts for the visible range → the "Plan (Schicht)"
  // column, same as the admin view (#1842 AC1). Loading and error state are
  // surfaced explicitly: rendering an unresolved or failed request as []
  // would show "–" in every Plan cell, presenting a fetch failure as proof
  // that no shifts were planned.
  const {
    data: tableShifts,
    isLoading: shiftsLoading,
    error: shiftsError,
  } = useSWRAuth(
    `time-tracking-table-shifts-${visibleFromKey}-${visibleToKey}`,
    () => ownShiftService.getOwnShifts(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Servergerechnete Tagesprojektion für den sichtbaren Zeitraum (#1842,
  // #2443): Soll, Gutschrift, Ist und Saldo je Tag, aus derselben Rechnung wie
  // die Monatskarte. In BEIDEN Ansichtsmodi geladen — die `schedule`-Prop
  // beschreibt nur den AKTUELLEN Plan, und ein Saldo aus „Ist minus Soll"
  // kennt weder Gutschriften noch Abwesenheitstage ohne Session.
  // `isLoading` is the staleness signal, not a spinner: with keepPreviousData
  // SWR serves the PREVIOUS range's map while the new one is in flight, and the
  // table must not fall back to today's plan for the days it doesn't cover.
  const {
    data: dailyProjection,
    error: dailyProjectionError,
    isLoading: dailyProjectionLoading,
  } = useSWRAuth(
    `time-tracking-schedule-targets-${visibleFromKey}-${visibleToKey}`,
    () => timeTrackingService.getDailyProjection(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Monatskarte (#1842): server-computed month aggregate, read-only for the
  // staff member. Only fetched in month mode. Check-ins, checkouts and
  // absence changes invalidate this key (refreshTableData); the poll on top
  // covers a still-running session, whose Ist grows server-side.
  const monthYear = monthAnchor.getFullYear();
  const monthNumber = monthAnchor.getMonth() + 1;
  const isCurrentMonth =
    monthYear === today.getFullYear() && monthNumber === today.getMonth() + 1;
  // Gesetzliche Feiertage im sichtbaren Zeitraum (#1418 3a): eigenes Badge
  // in der Tabelle + Feiertagsarbeit-Warnung. Das Soll dieser Tage liefert
  // der Server bereits als 0; ein Fetch-Fehler lässt die Markierung weg,
  // ohne die Tabelle zu blockieren.
  const { data: tableHolidays } = useSWRAuth(
    `time-tracking-holidays-${visibleFromKey}-${visibleToKey}`,
    () => timeTrackingService.getHolidays(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );
  // OGS-Schließtage im sichtbaren Zeitraum (#1418 3b): eigenes Badge in der
  // Tabelle, Soll kommt bereits als 0 vom Server.
  const { data: tableClosingDays } = useSWRAuth(
    `time-tracking-closing-days-${visibleFromKey}-${visibleToKey}`,
    () => timeTrackingService.getClosingDays(visibleFromKey, visibleToKey),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  const {
    data: monthSummary,
    isLoading: monthSummaryLoading,
    error: monthSummaryError,
  } = useSWRAuth<MonthSummary>(
    viewMode === "month"
      ? `time-tracking-month-summary-${monthYear}-${monthNumber}`
      : null,
    () => timeTrackingService.getMonthSummary(monthYear, monthNumber),
    { refreshInterval: isCurrentMonth ? OPEN_MONTH_REFRESH_MS : 0 },
  );

  const {
    data: timeTrackingConfig,
    isLoading: timeTrackingConfigLoading,
    error: timeTrackingConfigError,
  } = useSWRAuth(
    "time-tracking-config",
    () => timeTrackingService.getConfig(),
    { revalidateOnFocus: false },
  );
  // A month wholly before the configured account start is summarized standalone
  // by the backend (full month, zero carry) and its closing value never enters
  // the account chain — so the Monatskarte must not sell it as a carry or as
  // the Stundenkonto (#1842).
  const accountStartDate = timeTrackingConfig?.accountStartDate ?? "";
  const isPreAccountMonth =
    accountStartDate !== "" &&
    `${monthYear}-${String(monthNumber).padStart(2, "0")}` <
      accountStartDate.slice(0, 7);
  // A start later THIS month (same month, future day) is not "pre-account" by
  // the month comparison above, but the account still hasn't begun — the card
  // must not print a "Stundenkonto Stand" for it (#1842). ISO dates compare
  // lexicographically; both are "YYYY-MM-DD".
  const accountStartsInFuture =
    accountStartDate !== "" && accountStartDate > todayISO;

  // Self-scoped audit-trail fetcher so staff read their own Abweichungsgründe
  // without time_tracking:manage (#1842 AC8).
  const fetchOwnEdits = useCallback(
    (_staffId: string, sessionId: string) =>
      timeTrackingService.getSessionEdits(sessionId),
    [],
  );

  // The table names the exact block to edit — a day can carry several
  // (#2402), so re-deriving "the session of the day" from the date alone
  // would open an arbitrary block. `tableSession` is the table's snake_case
  // row; the edit modal needs the camelCase history entry, matched by id.
  // Both ids are the SAME backend int64 kept as a string end to end
  // (mapWorkSessionResponse / adaptHistorySessionForMetrics), so the find()
  // compares canonical decimals with no lossy Number() round trip.
  const handleEdit = (
    date: Date,
    tableSession: { id?: string } | null = null,
  ) => {
    const dateKey = toISODate(date);
    const session =
      tableSession?.id != null
        ? (tableHistory.find((h) => h.id === tableSession.id) ?? null)
        : null;
    const absence =
      tableAbsences.find(
        (a) => a.dateStart <= dateKey && a.dateEnd >= dateKey,
      ) ?? null;
    onEditDay(date, session, absence);
  };

  const handlePrev = () => {
    if (viewMode === "month") {
      setMonthAnchor(
        (prev) => new Date(prev.getFullYear(), prev.getMonth() - 1, 1),
      );
    } else {
      setWeekAnchor((prev) => {
        const d = new Date(prev);
        d.setDate(d.getDate() - 7);
        return d;
      });
    }
  };
  const handleNext = () => {
    if (viewMode === "month") {
      setMonthAnchor(
        (prev) => new Date(prev.getFullYear(), prev.getMonth() + 1, 1),
      );
    } else {
      setWeekAnchor((prev) => {
        const d = new Date(prev);
        d.setDate(d.getDate() + 7);
        return d;
      });
    }
  };
  const handleToday = () => {
    if (viewMode === "month") {
      setMonthAnchor(new Date(today.getFullYear(), today.getMonth(), 1));
    } else {
      const d = new Date(today);
      const day = (d.getDay() + 6) % 7;
      d.setDate(d.getDate() - day);
      d.setHours(0, 0, 0, 0);
      setWeekAnchor(d);
    }
  };
  const isOnCurrent = useMemo(() => {
    if (viewMode === "month") {
      return isCurrentMonth;
    }
    const cur = new Date(today);
    const day = (cur.getDay() + 6) % 7;
    cur.setDate(cur.getDate() - day);
    cur.setHours(0, 0, 0, 0);
    return cur.getTime() === weekAnchor.getTime();
  }, [viewMode, isCurrentMonth, weekAnchor, today]);

  const labelRange = useMemo(() => {
    if (viewMode === "month") {
      return monthAnchor.toLocaleDateString("de-DE", {
        timeZone: "Europe/Berlin",
        month: "long",
        year: "numeric",
      });
    }
    const weekNum = getWeekNumber(visibleFrom);
    const start = visibleFrom.toLocaleDateString("de-DE", {
      timeZone: "Europe/Berlin",
      day: "numeric",
      month: "short",
    });
    const end = visibleTo.toLocaleDateString("de-DE", {
      timeZone: "Europe/Berlin",
      day: "numeric",
      month: "short",
      year: "numeric",
    });
    return `KW ${weekNum}: ${start} bis ${end}`;
  }, [viewMode, monthAnchor, visibleFrom, visibleTo]);

  const todayLabel = viewMode === "month" ? "Diesen Monat" : "Diese Woche";

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <h2 className="text-base font-bold text-gray-900 sm:text-lg">
          Zeiterfassung
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          <ViewToggle value={viewMode} onChange={setViewMode} />
          {ownStaffId && (
            <StaffExportButton
              staffId={ownStaffId}
              yearStart={startOfYear(today)}
            />
          )}
        </div>
      </div>
      <div className="mb-4 flex flex-col gap-3 sm:grid sm:grid-cols-3 sm:items-center">
        <div className="hidden sm:block" />
        <div className="flex min-w-0 items-center justify-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={handlePrev}
            aria-label={
              viewMode === "month" ? "Vorheriger Monat" : "Vorherige Woche"
            }
            className="!rounded-full"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 19l-7-7 7-7"
              />
            </svg>
          </Button>
          <h3 className="min-w-0 flex-1 text-center text-sm font-semibold text-gray-800 sm:min-w-[14rem]">
            {labelRange}
          </h3>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={handleNext}
            aria-label={
              viewMode === "month" ? "Nächster Monat" : "Nächste Woche"
            }
            className="!rounded-full"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 5l7 7-7 7"
              />
            </svg>
          </Button>
        </div>
        <div className="flex justify-center sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={handleToday}
            disabled={isOnCurrent}
            className="!rounded-full shadow-none disabled:opacity-40"
          >
            {todayLabel}
          </Button>
        </div>
      </div>
      {viewMode === "month" && (
        <div className="mb-4">
          <Monatskarte
            summary={monthSummary ?? null}
            isLoading={monthSummaryLoading}
            error={
              monthSummaryError
                ? "Die Monatskarte konnte nicht geladen werden."
                : null
            }
            isCurrentMonth={isCurrentMonth}
            isPreAccountMonth={isPreAccountMonth}
            accountStartsInFuture={accountStartsInFuture}
            accountStartDate={accountStartDate}
          />
        </div>
      )}
      {(tableLoading && tableHistory.length === 0) || shiftsLoading ? (
        // NOT the kit <Loading />: page.test.tsx pins this placeholder by its
        // literal "..." text ("shows loading indicator in weekly total when
        // history is loading"), and that test predates this migration.
        <div className="py-10 text-center text-sm text-gray-400">...</div>
      ) : (
        <>
          {shiftsError ? (
            <div className="mb-4">
              <Alert
                type="error"
                message="Der Dienstplan konnte nicht geladen werden. Die Plan-Spalte ist deshalb unvollständig; bitte die Seite neu laden."
              />
            </div>
          ) : null}
          <StaffSessionTable
            staffId={ownStaffId ?? ""}
            from={visibleFrom}
            to={visibleTo}
            sessions={adaptedSessions}
            absences={adaptedAbsences}
            absencesUnresolved={
              tableAbsencesLoading ||
              tableAbsencesError != null ||
              tableAbsenceData === undefined
            }
            schedule={schedule}
            dailyProjection={dailyProjection}
            dailyProjectionError={dailyProjectionError != null}
            dailyProjectionPending={dailyProjectionLoading}
            holidays={tableHolidays}
            closingDays={tableClosingDays}
            accountStartDate={timeTrackingConfig?.accountStartDate ?? null}
            accountStartDatePending={timeTrackingConfigLoading}
            accountStartDateError={
              timeTrackingConfig === undefined &&
              timeTrackingConfigError != null
            }
            today={today}
            isAdminView={ownStaffId !== null}
            onEditDay={(date, session) => handleEdit(date, session)}
            plannedShifts={tableShifts ?? []}
            fetchEdits={fetchOwnEdits}
          />
        </>
      )}
    </div>
  );
}

// ─── ExportDropdown ───────────────────────────────────────────────────────────

// ─── WeekTable Helpers ────────────────────────────────────────────────────────

/** Pre-computed data for rendering a single day in the week table */

/** Compute all derived values for a day cell */

/** Get session status badge styling for week table (mobile) */

/** Get checkout time display for desktop table */

/** Get row background class based on state */

/** Render desktop edit actions cell */

// ─── EditModal ────────────────────────────────────────────────────────────────

const BREAK_DURATION_OPTIONS = [0, 15, 30, 45, 60] as const;

const EDIT_SECTION_ITEMS: readonly SegmentedControlItem<
  "session" | "absence"
>[] = [
  { value: "session", label: "Arbeitszeit" },
  { value: "absence", label: "Abwesenheit" },
];

// One-tap presets for the mandatory "Grund der Änderung" field.
const EDIT_REASON_PRESETS = [
  "Vergessen auszustempeln",
  "Vergessen einzustempeln",
  "Zeitkorrektur",
  "Krankheit",
  "Ort-Änderung",
] as const;

function timeInputToMinutes(value: string): number | null {
  const [hoursRaw, minutesRaw] = value.split(":");
  const hours = Number(hoursRaw);
  const minutes = Number(minutesRaw);
  if (
    !Number.isInteger(hours) ||
    !Number.isInteger(minutes) ||
    hours < 0 ||
    hours > 23 ||
    minutes < 0 ||
    minutes > 59
  ) {
    return null;
  }
  return hours * 60 + minutes;
}

/** Calculate compliance warnings for edited session times */
function calcEditComplianceWarnings(
  startTime: string,
  endTime: string,
  breakMinutes: number,
): string[] {
  if (!startTime || !endTime) return [];

  const [sh, sm] = startTime.split(":").map(Number);
  const [eh, em] = endTime.split(":").map(Number);

  if (
    sh === undefined ||
    sm === undefined ||
    eh === undefined ||
    em === undefined
  ) {
    return [];
  }

  const totalMins = eh * 60 + em - (sh * 60 + sm);
  const netMins = totalMins - breakMinutes;
  const warnings: string[] = [];

  if (netMins > 600) {
    warnings.push("Arbeitszeit > 10h (§3 ArbZG)");
  }
  if (netMins > 540 && breakMinutes < 45) {
    warnings.push("Pausenzeit < 45 Min bei > 9h Arbeitszeit (§4 ArbZG)");
  } else if (netMins > 360 && breakMinutes < 30) {
    warnings.push("Pausenzeit < 30 Min bei > 6h Arbeitszeit (§4 ArbZG)");
  }

  return warnings;
}

/** Get modal title based on content */
function getEditModalTitle(hasSession: boolean, hasAbsence: boolean): string {
  if (hasSession && hasAbsence) return "Tag bearbeiten";
  if (hasAbsence) return "Abwesenheit bearbeiten";
  return "Eintrag bearbeiten";
}

/** Build ISO timestamp from date and time string with local timezone */
function buildIsoTimestamp(date: Date, time: string): string | undefined {
  if (!time) return undefined;
  const dateStr = toISODate(date);
  const tzOffset = -new Date().getTimezoneOffset();
  const tzSign = tzOffset >= 0 ? "+" : "-";
  const tzH = String(Math.floor(Math.abs(tzOffset) / 60)).padStart(2, "0");
  const tzM = String(Math.abs(tzOffset) % 60).padStart(2, "0");
  return `${dateStr}T${time}:00${tzSign}${tzH}:${tzM}`;
}

// Shared Abbrechen/confirm footer for every Modal on this page — six
// hand-rolled copies of the same two <button> elements before. Uses the kit
// Button at size "md", the modal-footer height documented in the UI-kit rule.
function ModalActions({
  onCancel,
  onConfirm,
  confirmLabel,
  cancelLabel = "Abbrechen",
  confirmDisabled = false,
  cancelDisabled = false,
}: {
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
  readonly confirmLabel: string;
  readonly cancelLabel?: string;
  readonly confirmDisabled?: boolean;
  readonly cancelDisabled?: boolean;
}) {
  return (
    <div className="flex w-full flex-col-reverse gap-3 sm:flex-row">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onCancel}
        disabled={cancelDisabled}
        className="flex-1"
      >
        {cancelLabel}
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={onConfirm}
        disabled={confirmDisabled}
        className="flex-1"
      >
        {confirmLabel}
      </Button>
    </div>
  );
}

/** Get modal footer based on context */
function getEditModalFooter(
  hasBothSections: boolean,
  hasAbsence: boolean,
  activeTab: "session" | "absence",
  sessionFooter: React.ReactNode,
  absenceFooter: React.ReactNode,
): React.ReactNode {
  if (hasBothSections) {
    return activeTab === "session" ? sessionFooter : absenceFooter;
  }
  return hasAbsence ? absenceFooter : sessionFooter;
}

function MissingSessionHint({
  date,
  canManage,
}: {
  readonly date: Date;
  readonly canManage: boolean;
}) {
  const dayIndex = (date.getDay() + 6) % 7;
  const dayName = DAY_NAMES_LONG[dayIndex] ?? "";

  return (
    <div className="space-y-3 py-2 text-sm text-gray-600">
      <p>
        Für {dayName}, den {formatDate(toISODate(date))} ist keine Arbeitszeit
        eingetragen.
      </p>
      {canManage ? (
        <p>
          Mit „Nachtragen“ öffnen Sie die passende Seite. Dort tragen Sie die
          Arbeitszeit für diesen Tag ein.
        </p>
      ) : (
        <p>
          Nur die Leitung kann fehlende Tage nachtragen. Bitte melden Sie sich
          dort.
        </p>
      )}
    </div>
  );
}

function BackfillFooter({
  onClose,
  onBackfill,
}: {
  readonly onClose: () => void;
  readonly onBackfill: (() => void) | null;
}) {
  return (
    <div className="flex w-full flex-col-reverse gap-3 sm:flex-row">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        className="flex-1"
      >
        Schließen
      </Button>
      {onBackfill && (
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={onBackfill}
          className="flex-1"
        >
          Nachtragen
        </Button>
      )}
    </div>
  );
}

function EditSessionModal({
  isOpen,
  onClose,
  session,
  date,
  onSave,
  absence,
  onUpdateAbsence,
  onDeleteAbsence,
  canManage,
  ownStaffId,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly session: WorkSessionHistory | null;
  readonly date: Date | null;
  // Nachtragen ist bewusst admin-only (#2361). Mit time_tracking:manage
  // bietet der Hinweis-Dialog den Absprung in die eigene Verwaltungs-Ansicht.
  readonly canManage: boolean;
  readonly ownStaffId: string | null;
  readonly onSave: (
    id: string,
    updates: {
      checkInTime?: string;
      checkOutTime?: string;
      breakMinutes?: number;
      status?: SessionStatus;
      notes?: string;
      breaks?: Array<{ id: string; durationMinutes: number }>;
    },
  ) => Promise<void>;
  readonly absence: StaffAbsence | null;
  readonly onUpdateAbsence: (
    id: string,
    req: {
      absence_type?: string;
      absence_type_id?: string | null;
      date_start?: string;
      date_end?: string;
      half_day?: boolean;
      note?: string;
    },
  ) => Promise<void>;
  readonly onDeleteAbsence: (id: string) => Promise<void>;
}) {
  // Session state
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [breakMins, setBreakMins] = useState("0");
  const [breakDurations, setBreakDurations] = useState<Map<string, number>>(
    new Map(),
  );
  const [status, setStatus] = useState<SessionStatus>("present");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);

  // Absence state. absType is an option value, not an AbsenceType: it also
  // carries the school's own Abwesenheitsarten (#2403).
  const [absType, setAbsType] = useState<string>("sick");
  const [absDateStart, setAbsDateStart] = useState("");
  const [absDateEnd, setAbsDateEnd] = useState("");
  const [absHalfDay, setAbsHalfDay] = useState(false);
  const [absNote, setAbsNote] = useState("");
  const [absenceSaving, setAbsenceSaving] = useState(false);
  const [absenceDeleting, setAbsenceDeleting] = useState(false);
  const absenceTypeSelect = useAbsenceTypeSelect({
    value: absType,
    onChange: setAbsType,
    canManage,
  });

  const [activeTab, setActiveTab] = useState<"session" | "absence">("session");
  const router = useTenantRouter();

  const hasIndividualBreaks = (session?.breaks.length ?? 0) > 0;
  const hasSession = session !== null;
  const hasAbsence = absence !== null;
  const hasBoth = hasSession && hasAbsence;
  const isManagerControlledAbsence = absence?.absenceType === "comp_time";

  useEffect(() => {
    if (isOpen) {
      setActiveTab("session");
    }
  }, [isOpen]);

  useEffect(() => {
    if (session && isOpen) {
      setStartTime(extractTimeFromISO(session.checkInTime));
      setEndTime(
        session.checkOutTime ? extractTimeFromISO(session.checkOutTime) : "",
      );
      setBreakMins(session.breakMinutes.toString());
      setStatus(session.status);
      setNotes("");

      // Initialize per-break durations
      if (session.breaks.length > 0) {
        const durations = new Map<string, number>();
        for (const brk of session.breaks) {
          durations.set(brk.id, brk.durationMinutes);
        }
        setBreakDurations(durations);
      } else {
        setBreakDurations(new Map());
      }
    }
  }, [session, isOpen]);

  // Initialize absence state when modal opens
  useEffect(() => {
    if (absence && isOpen) {
      setAbsType(selectValueFor(absence.absenceType, absence.absenceTypeId));
      setAbsDateStart(absence.dateStart);
      setAbsDateEnd(absence.dateEnd);
      setAbsHalfDay(absence.halfDay);
      setAbsNote(absence.note);
    }
  }, [absence, isOpen]);

  // Clamp absDateEnd when absDateStart moves past it
  useEffect(() => {
    if (absDateStart && absDateEnd && absDateEnd < absDateStart) {
      setAbsDateEnd(absDateStart);
    }
  }, [absDateStart, absDateEnd]);

  if (!date) return null;

  const dayIndex = (date.getDay() + 6) % 7;
  const dayName = DAY_NAMES_LONG[dayIndex] ?? "";
  const dateKey = toISODate(date);
  const absenceIsHalfDay = absence
    ? isHalfAbsenceBoundary(
        adaptAbsenceForMetrics(absence),
        dateKey,
        absence.dateStart.slice(0, 10),
        absence.dateEnd.slice(0, 10),
      )
    : false;
  const onBackfill =
    canManage && ownStaffId
      ? () =>
          router.push(`/staff/${ownStaffId}?tab=zeiterfassung&date=${dateKey}`)
      : null;
  const hasBackfillableAbsence =
    !hasSession &&
    hasAbsence &&
    (absenceIsHalfDay || !isEffectiveAbsenceStatus(absence.status)) &&
    onBackfill !== null;
  const hasBothSections = hasBoth || hasBackfillableAbsence;

  // Tag ohne Buchung und ohne Abwesenheit: kein stiller No-Op mehr (#2361).
  // Nachtragen bleibt admin-only — der Dialog erklärt den Weg, Berechtigte
  // springen direkt in die eigene Verwaltungs-Ansicht ab.
  if (!hasSession && !hasAbsence) {
    return (
      <Modal
        isOpen={isOpen}
        onClose={onClose}
        title="Kein Eintrag vorhanden"
        footer={<BackfillFooter onClose={onClose} onBackfill={onBackfill} />}
      >
        <MissingSessionHint date={date} canManage={onBackfill !== null} />
      </Modal>
    );
  }

  // Calculate total break from individual breaks or fallback dropdown
  const editedBreak = hasIndividualBreaks
    ? Array.from(breakDurations.values()).reduce((sum, d) => sum + d, 0)
    : Number.parseInt(breakMins, 10) || 0;

  // Compliance check for the edited values
  const warnings = calcEditComplianceWarnings(startTime, endTime, editedBreak);
  const startMinutes = timeInputToMinutes(startTime);
  const endMinutes = timeInputToMinutes(endTime);
  const hasInvalidTimeRange =
    startMinutes !== null && endMinutes !== null && endMinutes <= startMinutes;

  const handleBreakDurationChange = (breakId: string, minutes: number) => {
    setBreakDurations((prev) => {
      const next = new Map(prev);
      next.set(breakId, minutes);
      return next;
    });
  };

  const handleSave = async () => {
    if (!session) return;
    if (hasInvalidTimeRange) return;
    setSaving(true);
    try {
      // Use session's actual date, not the clicked day (they may differ)
      const parts = session.date.split("-");
      const sessionDate = new Date(
        Number(parts[0]),
        Number(parts[1]) - 1,
        Number(parts[2]),
      );
      const checkInTime = buildIsoTimestamp(sessionDate, startTime);
      const checkOutTime = buildIsoTimestamp(sessionDate, endTime);

      if (hasIndividualBreaks) {
        // Build breaks array with only changed durations
        const changedBreaks = session.breaks
          .filter((brk) => {
            const newDuration = breakDurations.get(brk.id);
            return (
              newDuration !== undefined && newDuration !== brk.durationMinutes
            );
          })
          .map((brk) => ({
            id: brk.id,
            durationMinutes: breakDurations.get(brk.id)!,
          }));

        await onSave(session.id, {
          checkInTime,
          checkOutTime,
          status,
          notes: notes || undefined,
          breaks: changedBreaks.length > 0 ? changedBreaks : undefined,
        });
      } else {
        await onSave(session.id, {
          checkInTime,
          checkOutTime,
          breakMinutes: editedBreak,
          status,
          notes: notes || undefined,
        });
      }
      onClose();
    } finally {
      setSaving(false);
    }
  };

  const handleAbsenceSave = async () => {
    if (!absence) return;
    setAbsenceSaving(true);
    try {
      await onUpdateAbsence(absence.id, {
        ...absenceRequestFor(absType),
        date_start: absDateStart,
        date_end: absDateEnd,
        half_day: absHalfDay,
        note: absNote.trim() || undefined,
      });
      onClose();
    } finally {
      setAbsenceSaving(false);
    }
  };

  const handleAbsenceDelete = async () => {
    if (!absence) return;
    setAbsenceDeleting(true);
    try {
      await onDeleteAbsence(absence.id);
      onClose();
    } finally {
      setAbsenceDeleting(false);
    }
  };

  const modalTitle = getEditModalTitle(
    hasSession || hasBackfillableAbsence,
    hasAbsence,
  );

  // With only one of the two present there is no switcher, and the visible
  // section is pinned to whichever exists — `activeTab` alone would keep an
  // absence-only day on the (empty) "session" section.
  const effectiveTab: "session" | "absence" = hasBothSections
    ? activeTab
    : hasAbsence
      ? "absence"
      : "session";

  // Footer: tab-aware for dual-section, standard for single-section
  const sessionFooter = (
    <ModalActions
      onCancel={onClose}
      onConfirm={handleSave}
      confirmLabel={saving ? "Speichern..." : "Speichern"}
      confirmDisabled={
        saving || !startTime || !notes.trim() || hasInvalidTimeRange
      }
    />
  );

  const backfillFooter = (
    <BackfillFooter onClose={onClose} onBackfill={onBackfill} />
  );

  const absenceFooter = (
    <ModalActions
      onCancel={onClose}
      onConfirm={handleAbsenceSave}
      confirmLabel={absenceSaving ? "Speichern..." : "Speichern"}
      confirmDisabled={absenceSaving || !absDateStart || !absDateEnd}
    />
  );

  const readOnlyAbsenceFooter = (
    <Button type="button" variant="outline" size="md" onClick={onClose}>
      Schließen
    </Button>
  );

  const footer = getEditModalFooter(
    hasBothSections,
    hasAbsence,
    activeTab,
    hasSession ? sessionFooter : backfillFooter,
    isManagerControlledAbsence ? readOnlyAbsenceFooter : absenceFooter,
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={modalTitle} footer={footer}>
      <div className="space-y-4">
        <p className="text-sm font-medium text-gray-500">
          {dayName}, {formatDateGerman(date)}
        </p>

        {/* Section switcher for a session or a backfillable half-day alongside
            an absence. The kit
            SegmentedControl, NOT ui/Tabs: Radix tabs activate on mousedown, and
            the existing modal tests drive this switcher with fireEvent.click. */}
        {hasBothSections && (
          <SegmentedControl
            ariaLabel="Abschnitt"
            fullWidth
            items={EDIT_SECTION_ITEMS}
            value={effectiveTab}
            onChange={setActiveTab}
          />
        )}

        {!hasSession &&
          hasBackfillableAbsence &&
          effectiveTab === "session" && (
            <MissingSessionHint date={date} canManage={onBackfill !== null} />
          )}

        {/* ── Session section ──────────────────────────────────────────── */}
        {hasSession && effectiveTab === "session" && (
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Input
                id="edit-start"
                name="edit-start"
                label="Start"
                controlSize="compact"
                type="time"
                value={startTime}
                onChange={(e) => setStartTime(e.target.value)}
              />
              <Input
                id="edit-end"
                name="edit-end"
                label="Ende"
                controlSize="compact"
                type="time"
                value={endTime}
                onChange={(e) => setEndTime(e.target.value)}
              />
            </div>

            {hasInvalidTimeRange && (
              <Alert type="error" message="Ende muss nach Start liegen." />
            )}

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {/* Break section */}
              <div>
                {hasIndividualBreaks ? (
                  <div>
                    <span className="mb-1 block text-sm font-medium text-gray-700">
                      Pausen
                    </span>
                    <div className="space-y-2">
                      {session.breaks.map((brk) => (
                        <div key={brk.id} className="flex items-center gap-2">
                          <span className="w-12 shrink-0 text-xs text-gray-500 tabular-nums">
                            {formatTime(brk.startedAt)}
                          </span>
                          <div className="flex-1">
                            <CustomSelect
                              ariaLabel={`Pausendauer ab ${formatTime(brk.startedAt)}`}
                              value={(
                                breakDurations.get(brk.id) ??
                                brk.durationMinutes
                              ).toString()}
                              onChange={(next) =>
                                handleBreakDurationChange(
                                  brk.id,
                                  Number.parseInt(next, 10),
                                )
                              }
                              options={BREAK_DURATION_OPTIONS.map((m) => ({
                                value: m.toString(),
                                label: `${m} min`,
                              }))}
                            />
                          </div>
                        </div>
                      ))}
                      <div className="flex items-center justify-between border-t border-gray-100 pt-1.5">
                        <span className="text-xs font-medium text-gray-500">
                          Gesamt
                        </span>
                        <span className="text-sm font-medium text-gray-700 tabular-nums">
                          {editedBreak} min
                        </span>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div>
                    <label
                      id="edit-break-label"
                      htmlFor="edit-break"
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Pause (Min)
                    </label>
                    <CustomSelect
                      id="edit-break"
                      ariaLabelledBy="edit-break-label"
                      value={breakMins}
                      onChange={setBreakMins}
                      options={[0, 15, 30, 45, 60].map((m) => ({
                        value: m.toString(),
                        label: `${m} min`,
                      }))}
                    />
                  </div>
                )}
              </div>
              <div>
                <label
                  id="edit-status-label"
                  htmlFor="edit-status"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Ort
                </label>
                <CustomSelect
                  id="edit-status"
                  ariaLabelledBy="edit-status-label"
                  value={status}
                  onChange={(next) => setStatus(next as SessionStatus)}
                  options={[
                    { value: "present", label: "In der OGS" },
                    { value: "home_office", label: "Homeoffice" },
                  ]}
                />
              </div>
            </div>

            <div>
              <label
                htmlFor="edit-notes"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Grund der Änderung <span className="text-moto-red">*</span>
              </label>
              <div className="mb-2 flex flex-wrap gap-1.5">
                {EDIT_REASON_PRESETS.map((reason) => (
                  <Button
                    key={reason}
                    type="button"
                    variant={notes === reason ? "primary" : "secondary"}
                    size="compact"
                    aria-pressed={notes === reason}
                    onClick={() => setNotes(reason)}
                    className="!rounded-full shadow-none hover:shadow-none"
                  >
                    {reason}
                  </Button>
                ))}
              </div>
              <Textarea
                id="edit-notes"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={2}
                maxLength={2000}
                placeholder="Oder eigenen Grund eingeben..."
              />
            </div>

            {warnings.length > 0 && (
              <div className="space-y-2">
                {warnings.map((w) => (
                  <Alert key={w} type="warning" message={w} />
                ))}
              </div>
            )}
          </div>
        )}

        {/* ── Absence section ──────────────────────────────────────────── */}
        {hasAbsence && effectiveTab === "absence" && (
          <div className="space-y-4">
            {isManagerControlledAbsence ? (
              <div className="space-y-4">
                <Alert
                  type="info"
                  message="Freizeitausgleich wird von der Leitung eingetragen und kann hier nicht geändert oder gelöscht werden."
                />
                <dl className="grid grid-cols-1 gap-3 rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm sm:grid-cols-2">
                  <div>
                    <dt className="font-medium text-gray-500">Zeitraum</dt>
                    <dd className="mt-1 text-gray-900">
                      {absence.dateStart} bis {absence.dateEnd}
                    </dd>
                  </div>
                  <div>
                    <dt className="font-medium text-gray-500">Umfang</dt>
                    <dd className="mt-1 text-gray-900">
                      {absence.halfDay ? "Halber Tag" : "Ganzer Tag"}
                    </dd>
                  </div>
                  {absence.note && (
                    <div className="sm:col-span-2">
                      <dt className="font-medium text-gray-500">Bemerkung</dt>
                      <dd className="mt-1 text-gray-900">{absence.note}</dd>
                    </div>
                  )}
                </dl>
              </div>
            ) : (
              <>
                {/* Absence type */}
                <div>
                  <label
                    id="edit-abs-type-label"
                    htmlFor="edit-abs-type"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    Art der Abwesenheit
                  </label>
                  <ListboxDropdown
                    {...absenceTypeSelect}
                    id="edit-abs-type"
                    ariaLabelledBy="edit-abs-type-label"
                    testId="edit-absence-type-select"
                  />
                </div>

                {/* Date range */}
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div>
                    <label
                      htmlFor="edit-abs-start"
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Von
                    </label>
                    <ISODatePicker
                      id="edit-abs-start"
                      value={absDateStart}
                      onChange={setAbsDateStart}
                      calendarLayout="popover"
                      hideClearButton
                    />
                  </div>
                  <div>
                    <label
                      htmlFor="edit-abs-end"
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Bis
                    </label>
                    <ISODatePicker
                      id="edit-abs-end"
                      value={absDateEnd}
                      min={absDateStart || undefined}
                      onChange={setAbsDateEnd}
                      calendarLayout="popover"
                      hideClearButton
                    />
                  </div>
                </div>

                {/* Half day toggle */}
                <div className="flex items-center gap-3">
                  <BooleanField
                    ariaLabel="Halber Tag"
                    value={absHalfDay}
                    onChange={setAbsHalfDay}
                  />
                  <span className="text-sm font-medium text-gray-700">
                    Halber Tag
                  </span>
                </div>

                {/* Absence note */}
                <div>
                  <label
                    htmlFor="edit-abs-note"
                    className="mb-1 block text-sm font-medium text-gray-700"
                  >
                    Bemerkung{" "}
                    <span className="font-normal text-gray-400">
                      (optional)
                    </span>
                  </label>
                  <Textarea
                    id="edit-abs-note"
                    value={absNote}
                    onChange={(e) => setAbsNote(e.target.value)}
                    rows={2}
                    maxLength={2000}
                    placeholder="z.B. Arzttermin, Schulung ..."
                  />
                </div>

                {/* Destructive action */}
                <div className="pt-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={handleAbsenceDelete}
                    disabled={absenceDeleting}
                    className="text-moto-red hover:text-moto-red-strong px-0 text-sm hover:bg-transparent"
                  >
                    {absenceDeleting
                      ? "Abwesenheit wird gelöscht..."
                      : "Abwesenheit löschen"}
                  </Button>
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}

// ─── CreateAbsenceModal ───────────────────────────────────────────────────────

function CreateAbsenceModal({
  isOpen,
  onClose,
  onSave,
  canManageAbsenceTypes,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSave: (req: {
    absence_type: string;
    absence_type_id: string | null;
    date_start: string;
    date_end: string;
    half_day?: boolean;
    note?: string;
  }) => Promise<void>;
  /** Gates the "Name hinzufügen" affordance on time_tracking:manage (#2403). */
  readonly canManageAbsenceTypes: boolean;
}) {
  const todayStr = toISODate(new Date());
  // One option space for the standard types and the school's own names — see
  // absence-type-select for why the value is a string, not an AbsenceType.
  const [absenceType, setAbsenceType] = useState<string>("sick");
  const [dateStart, setDateStart] = useState(todayStr);
  const [dateEnd, setDateEnd] = useState(todayStr);
  const [halfDay, setHalfDay] = useState(false);
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const absenceTypeSelect = useAbsenceTypeSelect({
    value: absenceType,
    onChange: setAbsenceType,
    canManage: canManageAbsenceTypes,
  });

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      const today = toISODate(new Date());
      setAbsenceType("sick");
      setDateStart(today);
      setDateEnd(today);
      setHalfDay(false);
      setNote("");
    }
  }, [isOpen]);

  // Clamp dateEnd when dateStart moves past it
  useEffect(() => {
    if (dateStart && dateEnd && dateEnd < dateStart) {
      setDateEnd(dateStart);
    }
  }, [dateStart, dateEnd]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave({
        ...absenceRequestFor(absenceType),
        date_start: dateStart,
        date_end: dateEnd,
        half_day: halfDay || undefined,
        note: note.trim() || undefined,
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Abwesenheit melden"
      footer={
        <ModalActions
          onCancel={onClose}
          onConfirm={handleSave}
          confirmLabel={saving ? "Speichern..." : "Speichern"}
          confirmDisabled={saving || !dateStart || !dateEnd}
        />
      }
    >
      <div className="space-y-4">
        {/* Absence type */}
        <div>
          <label
            id="absence-type-label"
            htmlFor="absence-type"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Art der Abwesenheit
          </label>
          <ListboxDropdown
            {...absenceTypeSelect}
            id="absence-type"
            ariaLabelledBy="absence-type-label"
            testId="absence-type-select"
          />
        </div>

        {/* Date range */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label
              htmlFor="absence-start"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Von
            </label>
            <ISODatePicker
              id="absence-start"
              value={dateStart}
              onChange={setDateStart}
              calendarLayout="popover"
              hideClearButton
            />
          </div>
          <div>
            <label
              htmlFor="absence-end"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Bis
            </label>
            <ISODatePicker
              id="absence-end"
              value={dateEnd}
              min={dateStart || undefined}
              onChange={setDateEnd}
              calendarLayout="popover"
              hideClearButton
            />
          </div>
        </div>

        {/* Half day toggle */}
        <div className="flex items-center gap-3">
          <BooleanField
            ariaLabel="Halber Tag"
            value={halfDay}
            onChange={setHalfDay}
          />
          <span className="text-sm font-medium text-gray-700">Halber Tag</span>
        </div>

        {/* Note */}
        <div>
          <label
            htmlFor="absence-note"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Bemerkung{" "}
            <span className="font-normal text-gray-400">(optional)</span>
          </label>
          <Textarea
            id="absence-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={2}
            maxLength={2000}
            placeholder="z.B. Arzttermin, Schulung ..."
          />
        </div>
      </div>
    </Modal>
  );
}

// ─── Main Content ─────────────────────────────────────────────────────────────

function TimeTrackingContent() {
  const { data: authSession, status: authStatus } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  // Nachtragen fehlender Tage ist admin-only (#2361); mit dieser Berechtigung
  // bietet der Hinweis-Dialog den Absprung in die eigene Verwaltungs-Ansicht.
  const canManageTimeTracking = hasPermission(
    authSession,
    "time_tracking:manage",
  );

  const toast = useToast();
  const todayISO = useBerlinToday();
  // WeekChart shows trailing 10 workdays from today; no UI to navigate it
  // anymore (the table owns its own range state). Kept as a constant so the
  // chart's data window stays anchored at "now".
  const weekOffset = 0;
  const [editModal, setEditModal] = useState<{
    date: Date;
    session: WorkSessionHistory | null;
    absence: StaffAbsence | null;
  } | null>(null);
  const [currentBreaks, setCurrentBreaks] = useState<WorkSessionBreak[]>([]);
  const [absenceModalOpen, setAbsenceModalOpen] = useState(false);
  const handleCloseEditModal = useCallback(() => setEditModal(null), []);
  const handleCloseAbsenceModal = useCallback(
    () => setAbsenceModalOpen(false),
    [],
  );
  const handleClosePendingCheckIn = useCallback(
    () => setPendingCheckIn(null),
    [],
  );
  const [pendingCheckIn, setPendingCheckIn] = useState<SessionStatus | null>(
    null,
  );

  // F9 deviation-reason prompt. Set when the backend rejects a stamp with
  // DEVIATION_REASON_REQUIRED_CODE because it falls outside the tolerance
  // window around the planned shift window. The modal collects the reason
  // and retries the same stamp; the backend stores it in the audit log.
  const [pendingDeviation, setPendingDeviation] = useState<{
    action: "check_in" | "check_out";
    status?: SessionStatus;
    plannedTime?: string;
    actualTime?: string;
    deviationMinutes?: string;
  } | null>(null);
  const [deviationReason, setDeviationReason] = useState("");
  const [deviationSubmitting, setDeviationSubmitting] = useState(false);
  const handleClosePendingDeviation = useCallback(() => {
    setPendingDeviation(null);
    setDeviationReason("");
  }, []);

  // Calculate date range for data fetching
  // - Chart shows trailing 10 workdays ending at reference date
  // - WeekView shows calendar week containing reference date, so the
  //   fetch must include up to the Sunday of that week (otherwise days
  //   after `ref` in the same week are never fetched and the table
  //   silently drops them)
  const { toDate, chartFromDate, weekFromDate } = (() => {
    const ref = new Date();
    ref.setDate(ref.getDate() + weekOffset * 7);
    const days = getWeekDays(ref);
    const sunday = days[6] ?? ref;
    const fetchEnd = sunday.getTime() > ref.getTime() ? sunday : ref;

    // Chart needs ~14 days back to cover 10 workdays (worst case)
    const chartStart = new Date(ref);
    chartStart.setDate(chartStart.getDate() - 14);

    return {
      toDate: toISODate(fetchEnd), // Sunday of reference week or today
      chartFromDate: toISODate(chartStart), // 14 days before reference
      weekFromDate: days[0] ? toISODate(days[0]) : "", // Monday of reference week
    };
  })();

  // Fetch current session
  const { data: currentSession, mutate: mutateCurrentSession } =
    useSWRAuth<WorkSession | null>(
      "time-tracking-current",
      () => timeTrackingService.getCurrentSession(),
      { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
    );

  // Pattern-mutate helper — refreshes the OwnZeiterfassungSection's dedicated
  // table fetches, its Tagesprojektion and its Monatskarte after a
  // check-in/out, edit or absence change, so all three update without a manual
  // refresh. The Monatskarte belongs in here: its Ist, Gutschriften, Saldo and
  // Übertrag are server-derived from exactly the sessions and absences these
  // handlers just changed. So is the Tagesprojektion since #2443 — the
  // schedule-targets key stopped being a pure Soll map and now carries the
  // server-computed Ist, Gutschrift and Saldo per day, which means every one
  // of these mutations invalidates it. Leaving it out left the table's own
  // numbers stale while the session list and the Monatskarte above them had
  // already moved on. The weekly KPI card (usePeriodMetrics) shares the key
  // and is refreshed with it.
  const refreshTableData = useTenantMutateMatching([
    "time-tracking-table",
    "time-tracking-month-summary",
    "time-tracking-schedule-targets-",
  ]);

  // Fetch history covering 2 weeks (chart needs prev + current week)
  const { data: historyData, mutate: mutateHistory } = useSWRAuth<{
    sessions: WorkSessionHistory[];
    weeklySummaries: WeeklySummary[];
  }>(
    chartFromDate && toDate
      ? `time-tracking-history-${chartFromDate}-${toDate}`
      : null,
    () => timeTrackingService.getHistory(chartFromDate, toDate),
    { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
  );

  const history = useMemo(() => historyData?.sessions ?? [], [historyData]);

  // Fetch absences for the same date range
  const { data: absencesData, mutate: mutateAbsences } = useSWRAuth<
    StaffAbsence[]
  >(
    chartFromDate && toDate
      ? `time-tracking-absences-${chartFromDate}-${toDate}`
      : null,
    () => timeTrackingService.getAbsences(chartFromDate, toDate),
    { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
  );

  const absences = absencesData ?? [];

  // Check if today has an absence (for check-in warning)
  const todayAbsence = absences.find(
    (a) => a.dateStart <= todayISO && a.dateEnd >= todayISO,
  );

  // Today's own planned shifts (Dienstplan) — shown as a quiet
  // "Geplant: 08:00–16:00" line in the Stempeluhr card. Fetched for today
  // only, independent of the viewed chart week, so navigating to another
  // week never hides today's planned shifts.
  const { data: ownTodayShifts } = useSWRAuth<StaffShift[]>(
    `time-tracking-own-shifts-today-${todayISO}`,
    () => ownShiftService.getOwnShifts(todayISO, todayISO),
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );
  const todayShifts = useMemo(
    () =>
      (ownTodayShifts ?? [])
        // A cancelled shift does not take place (#1841): the person is absent
        // or the gap is left open, so it must not appear as a *planned* shift in
        // the Stempeluhr. A replacement is the covering person's own shift and
        // shows for them normally (tagged "Vertretung", #1844).
        .filter((s) => s.date === todayISO && !s.cancelled)
        .sort((a, b) => a.startTime.localeCompare(b.startTime)),
    [ownTodayShifts, todayISO],
  );
  // Cancelled shifts are shown separately as "entfällt" so Dienstplan-level
  // changes stay visible to the staff member instead of silently vanishing
  // (#1844, AC 2).
  const todayCancelledShifts = useMemo(
    () =>
      (ownTodayShifts ?? [])
        .filter((s) => s.date === todayISO && s.cancelled)
        .sort((a, b) => a.startTime.localeCompare(b.startTime)),
    [ownTodayShifts, todayISO],
  );

  // --- MA-Saldo-Widget (Tranche 1.5) ----------------------------------------
  //
  // Diese Woche / Dieser Monat / Stundenkonto all come from the server-computed
  // model (usePeriodMetrics, #1842), which prices every day against the
  // schedule that was valid on that day. The old path fed the CURRENT schedule
  // plus a history fetch over the whole account range into computeStaffMetrics,
  // which re-priced historical days at today's hours and contradicted the
  // Monatskarte further down this very page.
  const { data: ownStaff } = useSWRAuth(
    "time-tracking-own-staff",
    () => userContextService.getCurrentStaff(),
    { revalidateOnFocus: false },
  );
  const ownStaffId = ownStaff?.id ?? null;

  const { data: ownSchedule } = useSWRAuth(
    ownStaffId ? `time-tracking-own-schedule-${ownStaffId}` : null,
    () => staffScheduleService.getSchedule(ownStaffId as string),
    { revalidateOnFocus: false },
  );

  const ownMetrics = usePeriodMetrics();

  // Fetch breaks for current session
  const fetchBreaks = useCallback(async () => {
    if (!currentSession?.id || currentSession.checkOutTime) {
      setCurrentBreaks([]);
      return;
    }
    try {
      const breaks = await timeTrackingService.getSessionBreaks(
        currentSession.id,
      );
      setCurrentBreaks(breaks);
    } catch (err) {
      logger.error("fetch_breaks_failed", {
        error: err instanceof Error ? err.message : String(err),
        session_id: currentSession?.id,
      });
    }
  }, [currentSession?.id, currentSession?.checkOutTime]);

  useEffect(() => {
    fetchBreaks().catch(() => {
      // Error already handled in fetchBreaks
    });
  }, [fetchBreaks]);

  // Weekly and daily minutes for the stamp card. Every block is booked on the
  // Berlin calendar day its minutes actually fall on — a block from 22:00 to
  // 02:00 splits across both days — which is what the week chart, the
  // Monatskarte and the server-side balance do. Summing whole netMinutes onto
  // the row's check-in date instead would put a whole night on the day it
  // started and contradict all three.
  const closedMinutesByDate = useMemo(
    () =>
      indexWorkSessionMinutesByBerlinDate(
        history.filter((session) => session.checkOutTime !== null),
      ),
    [history],
  );
  const allMinutesByDate = useMemo(
    () => indexWorkSessionMinutesByBerlinDate(history),
    [history],
  );
  // Today counts only closed blocks: ClockInCard adds the running one live
  // (clamped to today, see calcGrossMinutes). On the earlier days of the week
  // the running block's minutes are already final, so those do count here.
  const weeklyCompletedMinutes = useMemo(() => {
    let sum = 0;
    for (const [date, minutes] of allMinutesByDate) {
      if (date < weekFromDate || date > toDate || date === todayISO) continue;
      sum += minutes.netMinutes;
    }
    if (todayISO >= weekFromDate && todayISO <= toDate) {
      sum += closedMinutesByDate.get(todayISO)?.netMinutes ?? 0;
    }
    return sum;
  }, [allMinutesByDate, closedMinutesByDate, weekFromDate, toDate, todayISO]);
  const completedToday = useMemo(() => {
    const today = closedMinutesByDate.get(todayISO);
    return {
      minutes: today?.netMinutes ?? 0,
      breaks: today?.breakMinutes ?? 0,
    };
  }, [closedMinutesByDate, todayISO]);

  const executeCheckIn = useCallback(
    async (status: SessionStatus) => {
      try {
        await timeTrackingService.checkIn(status);
        await Promise.all([
          mutateCurrentSession(),
          mutateHistory(),
          refreshTableData(),
        ]);
        toast.success("Erfolgreich eingestempelt");
      } catch (err) {
        const apiErr = err as ApiError;
        if (apiErr.code === DEVIATION_REASON_REQUIRED_CODE) {
          const details = apiErr.details;
          setDeviationReason("");
          setPendingDeviation({
            action: "check_in",
            status,
            plannedTime:
              typeof details?.planned_time === "string"
                ? details.planned_time
                : undefined,
            actualTime:
              typeof details?.actual_time === "string"
                ? details.actual_time
                : undefined,
            deviationMinutes:
              typeof details?.deviation_minutes === "string"
                ? details.deviation_minutes
                : undefined,
          });
          return;
        }
        if (apiErr.code === PLANNED_START_NOT_REACHED_CODE) {
          const plannedStart =
            typeof apiErr.details?.planned_start_time === "string"
              ? apiErr.details.planned_start_time
              : undefined;
          toast.error(
            plannedStart
              ? `Einstempeln ist erst ab ${plannedStart} Uhr möglich.`
              : "Einstempeln ist noch nicht möglich.",
          );
          return;
        }
        logger.error("check_in_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(friendlyError(err, "Fehler beim Einstempeln"));
      }
    },
    [mutateCurrentSession, mutateHistory, refreshTableData, toast],
  );

  const handleCheckIn = useCallback(
    async (status: SessionStatus) => {
      if (todayAbsence) {
        setPendingCheckIn(status);
        return;
      }
      await executeCheckIn(status);
    },
    [todayAbsence, executeCheckIn],
  );

  const handleCheckOut = useCallback(async () => {
    try {
      await timeTrackingService.checkOut();
      setCurrentBreaks([]);
      await Promise.all([
        mutateCurrentSession(),
        mutateHistory(),
        refreshTableData(),
      ]);
      toast.success("Erfolgreich ausgestempelt");
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr.code === DEVIATION_REASON_REQUIRED_CODE) {
        const details = apiErr.details;
        setDeviationReason("");
        setPendingDeviation({
          action: "check_out",
          plannedTime:
            typeof details?.planned_time === "string"
              ? details.planned_time
              : undefined,
          actualTime:
            typeof details?.actual_time === "string"
              ? details.actual_time
              : undefined,
          deviationMinutes:
            typeof details?.deviation_minutes === "string"
              ? details.deviation_minutes
              : undefined,
        });
        return;
      }
      logger.error("check_out_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(friendlyError(err, "Fehler beim Ausstempeln"));
    }
  }, [mutateCurrentSession, mutateHistory, refreshTableData, toast]);

  // Retry the rejected stamp with the collected reason. The modal stays open
  // when the retry fails so the reason isn't lost.
  const confirmDeviationReason = useCallback(async () => {
    const pending = pendingDeviation;
    if (!pending) return;
    const reason = deviationReason.trim();
    if (!reason) return;

    setDeviationSubmitting(true);
    try {
      if (pending.action === "check_in" && pending.status) {
        await timeTrackingService.checkIn(pending.status, reason);
        toast.success("Erfolgreich eingestempelt");
      } else {
        await timeTrackingService.checkOut(reason);
        setCurrentBreaks([]);
        toast.success("Erfolgreich ausgestempelt");
      }
      await Promise.all([
        mutateCurrentSession(),
        mutateHistory(),
        refreshTableData(),
      ]);
      setPendingDeviation(null);
      setDeviationReason("");
    } catch (err) {
      logger.error("deviation_reason_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
        action: pending.action,
      });
      toast.error(friendlyError(err, "Fehler beim Stempeln"));
    } finally {
      setDeviationSubmitting(false);
    }
  }, [
    pendingDeviation,
    deviationReason,
    mutateCurrentSession,
    mutateHistory,
    refreshTableData,
    toast,
  ]);

  const handleStartBreak = useCallback(
    async (durationMinutes: number) => {
      try {
        await timeTrackingService.startBreak(durationMinutes);
        await fetchBreaks();
      } catch (err) {
        logger.error("start_break_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(friendlyError(err, "Fehler beim Starten der Pause"));
      }
    },
    [fetchBreaks, toast],
  );

  const handleEndBreak = useCallback(async () => {
    try {
      await timeTrackingService.endBreak();
      await Promise.all([
        mutateCurrentSession(),
        mutateHistory(),
        refreshTableData(),
        fetchBreaks(),
      ]);
    } catch (err) {
      logger.error("end_break_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(friendlyError(err, "Fehler beim Beenden der Pause"));
    }
  }, [
    mutateCurrentSession,
    mutateHistory,
    refreshTableData,
    fetchBreaks,
    toast,
  ]);

  const handleEditSave = useCallback(
    async (
      id: string,
      updates: {
        checkInTime?: string;
        checkOutTime?: string;
        breakMinutes?: number;
        status?: SessionStatus;
        notes?: string;
        breaks?: Array<{ id: string; durationMinutes: number }>;
      },
    ) => {
      try {
        await timeTrackingService.updateSession(id, {
          checkInTime: updates.checkInTime,
          checkOutTime: updates.checkOutTime,
          breakMinutes: updates.breakMinutes,
          status: updates.status,
          notes: updates.notes,
          breaks: updates.breaks,
        });
        await Promise.all([
          mutateCurrentSession(),
          mutateHistory(),
          refreshTableData(),
        ]);
        toast.success("Eintrag gespeichert");
      } catch (err) {
        logger.error("session_edit_failed", {
          error: err instanceof Error ? err.message : String(err),
          session_id: id,
        });
        toast.error(friendlyError(err, "Fehler beim Speichern"));
      }
    },
    [mutateCurrentSession, mutateHistory, refreshTableData, toast],
  );

  const handleCreateAbsence = useCallback(
    async (req: {
      absence_type: string;
      absence_type_id: string | null;
      date_start: string;
      date_end: string;
      half_day?: boolean;
      note?: string;
    }) => {
      try {
        await timeTrackingService.createAbsence({
          absence_type: req.absence_type,
          absence_type_id: req.absence_type_id,
          date_start: req.date_start,
          date_end: req.date_end,
          half_day: req.half_day,
          note: req.note,
        });
        await Promise.all([mutateAbsences(), refreshTableData()]);
        toast.success("Abwesenheit eingetragen");
        setAbsenceModalOpen(false);
      } catch (err) {
        logger.error("create_absence_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          friendlyError(err, "Fehler beim Eintragen der Abwesenheit"),
        );
      }
    },
    [mutateAbsences, refreshTableData, toast],
  );

  const handleDeleteAbsence = useCallback(
    async (id: string) => {
      try {
        await timeTrackingService.deleteAbsence(id);
        await Promise.all([mutateAbsences(), refreshTableData()]);
        toast.success("Abwesenheit gelöscht");
      } catch (err) {
        logger.error("delete_absence_failed", {
          error: err instanceof Error ? err.message : String(err),
          absence_id: id,
        });
        toast.error(friendlyError(err, "Fehler beim Löschen der Abwesenheit"));
      }
    },
    [mutateAbsences, refreshTableData, toast],
  );

  const handleUpdateAbsence = useCallback(
    async (
      id: string,
      req: {
        absence_type?: string;
        absence_type_id?: string | null;
        date_start?: string;
        date_end?: string;
        half_day?: boolean;
        note?: string;
      },
    ) => {
      try {
        await timeTrackingService.updateAbsence(id, req);
        await Promise.all([mutateAbsences(), refreshTableData()]);
        toast.success("Abwesenheit aktualisiert");
      } catch (err) {
        logger.error("update_absence_failed", {
          error: err instanceof Error ? err.message : String(err),
          absence_id: id,
        });
        toast.error(
          friendlyError(err, "Fehler beim Aktualisieren der Abwesenheit"),
        );
      }
    },
    [mutateAbsences, refreshTableData, toast],
  );

  if (authStatus === "loading") {
    return <TimeTrackingPageSkeleton />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Page header - only visible on mobile where breadcrumbs are hidden */}
      <h1 className="mb-6 text-2xl font-bold text-gray-900 md:hidden">
        Zeiterfassung
      </h1>

      {/* Action zone — Stempeluhr (mit integrierten Stats) und Wochenübersicht
          50/50 nebeneinander. Drunter eine Placeholder-Section für den
          Urlaubs-Workflow (kommt in eigenem Chat). */}
      <div className="mb-4 grid grid-cols-1 gap-4 md:mb-6 md:grid-cols-2 md:gap-6">
        <ClockInCard
          currentSession={currentSession ?? null}
          breaks={currentBreaks}
          onCheckIn={handleCheckIn}
          onCheckOut={handleCheckOut}
          onStartBreak={handleStartBreak}
          onEndBreak={handleEndBreak}
          weeklyMinutes={weeklyCompletedMinutes}
          completedTodayMinutes={completedToday.minutes}
          completedTodayBreakMinutes={completedToday.breaks}
          onAddAbsence={() => setAbsenceModalOpen(true)}
          metrics={ownMetrics}
          plannedShifts={todayShifts}
          cancelledShifts={todayCancelledShifts}
        />
        <WeekChart history={history} weekOffset={weekOffset} />
      </div>

      {/* Heute geplante Betreuungsplan-Einsätze (Ort/Aufgabe + Vertretungen,
          #1844). Rendert nichts, wenn die Schule keinen Betreuungsplan pflegt. */}
      <div className="mb-4 md:mb-6">
        <BetreuungsplanHeuteCard />
      </div>

      <div className="mb-4 md:mb-6">
        <LeaveRequestsCard />
      </div>

      {/* Zeiterfassung — gleiche Struktur wie auf /staff/[id], damit es
          zwischen Admin-Sicht und MA-Sicht keinen Drift gibt. Die externe
          onEditDay-Logik bleibt aktiv, damit der existing EditSessionModal
          (inkl. Absence-Handling) genutzt werden kann. */}
      <OwnZeiterfassungSection
        ownStaffId={ownStaffId}
        schedule={ownSchedule ?? null}
        onEditDay={(date, session, absence) =>
          setEditModal({ date, session, absence })
        }
      />

      {/* Edit modal */}
      <EditSessionModal
        isOpen={editModal !== null}
        onClose={handleCloseEditModal}
        session={editModal?.session ?? null}
        date={editModal?.date ?? null}
        onSave={handleEditSave}
        absence={editModal?.absence ?? null}
        onUpdateAbsence={handleUpdateAbsence}
        onDeleteAbsence={handleDeleteAbsence}
        canManage={canManageTimeTracking}
        ownStaffId={ownStaffId}
      />

      {/* Create absence modal */}
      <CreateAbsenceModal
        isOpen={absenceModalOpen}
        onClose={handleCloseAbsenceModal}
        onSave={handleCreateAbsence}
        canManageAbsenceTypes={canManageTimeTracking}
      />

      {/* Check-in confirmation when absence exists */}
      <Modal
        isOpen={pendingCheckIn !== null}
        onClose={handleClosePendingCheckIn}
        title="Abwesenheit eingetragen"
        footer={
          <ModalActions
            onCancel={() => setPendingCheckIn(null)}
            onConfirm={() => {
              const status = pendingCheckIn;
              setPendingCheckIn(null);
              if (status) void executeCheckIn(status);
            }}
            confirmLabel="Trotzdem einstempeln"
          />
        }
      >
        <div className="py-4 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-[#F78C10]/10">
            <svg
              className="h-6 w-6 text-[#8A5600]"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
              />
            </svg>
          </div>
          <p className="mt-2 text-gray-600">
            Für heute ist eine Abwesenheit eingetragen
            {todayAbsence ? ` (${absenceDisplayLabel(todayAbsence)})` : ""}.
            Trotzdem einstempeln?
          </p>
        </div>
      </Modal>

      {/* F9: stamping outside the tolerance window around the planned shift
          needs a reason that lands in the audit trail. */}
      <Modal
        isOpen={pendingDeviation !== null}
        onClose={handleClosePendingDeviation}
        title="Abweichung vom Dienstplan"
        footer={
          <ModalActions
            onCancel={handleClosePendingDeviation}
            cancelDisabled={deviationSubmitting}
            onConfirm={confirmDeviationReason}
            confirmDisabled={
              deviationSubmitting || deviationReason.trim() === ""
            }
            confirmLabel={
              pendingDeviation?.action === "check_in"
                ? "Mit Begründung einstempeln"
                : "Mit Begründung ausstempeln"
            }
          />
        }
      >
        <div className="py-2">
          <p className="text-sm text-gray-600">
            {pendingDeviation?.action === "check_in" ? (
              <>
                Du stempelst
                {pendingDeviation?.deviationMinutes ? (
                  <span className="font-medium text-gray-900">
                    {" "}
                    {pendingDeviation.deviationMinutes} Minuten
                  </span>
                ) : (
                  " deutlich"
                )}{" "}
                vor deinem geplanten Dienstbeginn
                {pendingDeviation?.plannedTime
                  ? ` (${pendingDeviation.plannedTime} Uhr)`
                  : ""}{" "}
                ein.
              </>
            ) : (
              <>
                Du stempelst
                {pendingDeviation?.deviationMinutes ? (
                  <span className="font-medium text-gray-900">
                    {" "}
                    {pendingDeviation.deviationMinutes} Minuten
                  </span>
                ) : (
                  " deutlich"
                )}{" "}
                nach deinem geplanten Dienstende
                {pendingDeviation?.plannedTime
                  ? ` (${pendingDeviation.plannedTime} Uhr)`
                  : ""}{" "}
                aus.
              </>
            )}{" "}
            Bitte gib einen kurzen Grund an. Er wird im Audit-Trail der Sitzung
            gespeichert.
          </p>
          <label
            htmlFor="deviation-reason"
            className="mt-4 block text-xs font-medium text-gray-700"
          >
            Grund
          </label>
          <Textarea
            id="deviation-reason"
            value={deviationReason}
            onChange={(e) => setDeviationReason(e.target.value)}
            disabled={deviationSubmitting}
            placeholder="z. B. Elterngespräch lief länger"
            rows={3}
            className="mt-1"
          />
        </div>
      </Modal>
    </div>
  );
}

// ─── Page Export ───────────────────────────────────────────────────────────────

function TimeTrackingPageSkeleton() {
  return (
    <SkeletonRegion label="Zeiterfassung wird geladen">
      <div className="mb-4 grid grid-cols-1 gap-4 md:mb-6 md:grid-cols-2 md:gap-6">
        <CardSkeleton rows={4} />
        <CardSkeleton rows={4} />
      </div>
      <div className="mb-4 md:mb-6">
        <CardSkeleton rows={2} />
      </div>
      <div className="mb-4 md:mb-6">
        <CardSkeleton rows={2} />
      </div>
      <TableSkeleton rows={7} columns={5} />
    </SkeletonRegion>
  );
}

export default function TimeTrackingPage() {
  return (
    <Suspense fallback={<TimeTrackingPageSkeleton />}>
      <TimeTrackingContent />
    </Suspense>
  );
}
