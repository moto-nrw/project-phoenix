"use client";

import React, {
  useState,
  useEffect,
  useCallback,
  useMemo,
  Suspense,
  useRef,
} from "react";
import { createPortal } from "react-dom";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChevronLeft, ChevronRight, Download, SquarePen } from "lucide-react";
import { Loading } from "~/components/ui/loading";
import {
  type ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useSWRAuth } from "~/lib/swr";
import { timeTrackingService } from "~/lib/time-tracking-api";
import {
  type AbsenceType,
  type StaffAbsence,
  type WorkSession,
  type WorkSessionBreak,
  type WorkSessionEdit,
  type WorkSessionHistory,
  absenceTypeLabels,
  absenceTypeColors,
  formatDuration,
  formatTime,
  getWeekDays,
  getWeekNumber,
  getComplianceWarnings,
  calculateNetMinutes,
} from "~/lib/time-tracking-helpers";

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDateGerman(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  const year = date.getFullYear();
  return `${day}.${month}.${year}`;
}

function formatDateShort(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}`;
}

function formatDateTime(date: Date): string {
  const d = date.getDate().toString().padStart(2, "0");
  const m = (date.getMonth() + 1).toString().padStart(2, "0");
  const y = date.getFullYear();
  const h = date.getHours().toString().padStart(2, "0");
  const min = date.getMinutes().toString().padStart(2, "0");
  return `${d}.${m}.${y}, ${h}:${min}`;
}

function toISODate(date: Date): string {
  const y = date.getFullYear();
  const m = (date.getMonth() + 1).toString().padStart(2, "0");
  const d = date.getDate().toString().padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function isBeforeDay(a: Date, b: Date): boolean {
  const aDate = new Date(a.getFullYear(), a.getMonth(), a.getDate());
  const bDate = new Date(b.getFullYear(), b.getMonth(), b.getDate());
  return aDate.getTime() < bDate.getTime();
}

// Extracts error message string from unknown error types
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === "string") return err;
  return "";
}

// Maps backend error messages to user-friendly German messages
function friendlyError(err: unknown, fallback: string): string {
  const msg = getErrorMessage(err);

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

  return fallback;
}

const DAY_NAMES = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
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

function formatHMM(minutes: number): string {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return `${h}:${m.toString().padStart(2, "0")}`;
}

function formatTimeFromDate(date: Date): string {
  return `${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
}

const BREAK_OPTIONS = [15, 30, 45, 60] as const;

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

// Returns status badge styling and label for current session state
function getSessionStatusBadge(
  isOnBreak: boolean,
  status: string | undefined,
): { className: string; label: string } {
  if (isOnBreak) {
    return { className: "bg-amber-100 text-amber-700", label: "Pause" };
  }
  if (status === "home_office") {
    return { className: "bg-sky-100 text-sky-700", label: "Homeoffice" };
  }
  return { className: "bg-[#83CD2D]/10 text-[#70b525]", label: "In der OGS" };
}

// Returns className for mode toggle button
function getModeToggleClassName(
  buttonMode: "present" | "home_office" | "absent",
  currentMode: "present" | "home_office" | "absent",
): string {
  const base =
    "rounded-full px-3 py-1.5 text-xs font-medium transition-all sm:px-4";
  const inactive = "bg-gray-100 text-gray-500 hover:bg-gray-200";
  if (buttonMode !== currentMode) return `${base} ${inactive}`;
  if (buttonMode === "present")
    return `${base} bg-[#83CD2D]/10 text-[#70b525] ring-1 ring-[#83CD2D]/40`;
  if (buttonMode === "home_office")
    return `${base} bg-sky-100 text-sky-700 ring-1 ring-sky-300`;
  return `${base} bg-red-100 text-red-700 ring-1 ring-red-300`;
}

// Returns className for check-in button based on mode
function getCheckInButtonClassName(
  mode: "present" | "home_office" | "absent",
): string {
  const base =
    "flex h-16 w-16 items-center justify-center rounded-full border-2 transition-all active:scale-95 disabled:opacity-50";
  if (mode === "home_office")
    return `${base} border-sky-500 text-sky-500 hover:bg-sky-50`;
  return `${base} border-[#83CD2D] text-[#83CD2D] hover:bg-[#83CD2D]/5`;
}

// Returns className for break/pause button
function getBreakButtonClassName(
  isOnBreak: boolean,
  breakMins: number,
): string {
  const base =
    "flex h-12 w-12 items-center justify-center rounded-full border-2 transition-all active:scale-95 disabled:opacity-60";
  if (isOnBreak) return `${base} border-amber-400 text-amber-500`;
  if (breakMins > 0)
    return `${base} border-amber-400 text-amber-500 hover:bg-amber-50`;
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

function ClockInCard({
  currentSession,
  breaks,
  onCheckIn,
  onCheckOut,
  onStartBreak,
  onEndBreak,
  weeklyMinutes,
  onAddAbsence,
}: {
  readonly currentSession: WorkSession | null;
  readonly breaks: WorkSessionBreak[];
  readonly onCheckIn: (status: "present" | "home_office") => Promise<void>;
  readonly onCheckOut: () => Promise<void>;
  readonly onStartBreak: () => Promise<void>;
  readonly onEndBreak: () => Promise<void>;
  readonly weeklyMinutes: number;
  readonly onAddAbsence: () => void;
}) {
  const [mode, setMode] = useState<"present" | "home_office" | "absent">(
    "present",
  );
  const [actionLoading, setActionLoading] = useState(false);
  const [tick, setTick] = useState(0); // forces re-render for live times
  const [breakMenuOpen, setBreakMenuOpen] = useState(false);
  const [plannedBreakMinutes, setPlannedBreakMinutes] = useState<number | null>(
    null,
  );

  const isCheckedIn =
    currentSession !== null && currentSession.checkOutTime === null;
  const isCheckedOut =
    currentSession !== null && currentSession.checkOutTime !== null;

  // Derive active break from breaks array
  const activeBreak = breaks.find((b) => !b.endedAt) ?? null;
  const isOnBreak = activeBreak !== null;

  // Tick every second during break (for countdown), every 30s otherwise
  useEffect(() => {
    if (!isCheckedIn) return;
    const interval = setInterval(
      () => setTick((t) => t + 1),
      isOnBreak ? 1000 : 30000,
    );
    return () => clearInterval(interval);
  }, [isCheckedIn, isOnBreak]);

  // Use `tick` to make `now` reactive (tick triggers re-render)
  // eslint-disable-next-line @typescript-eslint/no-unused-expressions
  tick;
  const now = new Date();

  const breakMins = currentSession?.breakMinutes ?? 0;

  // Calculate derived time values using extracted helpers
  const liveBreakMins = calcLiveBreakMins(breakMins, activeBreak, now);
  const grossMinutes =
    isCheckedIn && currentSession
      ? calcElapsedMinutes(currentSession.checkInTime, now)
      : 0;
  const netMinutes = Math.max(0, grossMinutes - liveBreakMins);
  const displayMinutes = isCheckedIn ? netMinutes : 0;
  const activeBreakElapsedSecs = activeBreak
    ? calcElapsedSeconds(activeBreak.startedAt, now)
    : 0;
  const countdownRemainingSecs =
    isOnBreak && plannedBreakMinutes !== null
      ? Math.max(0, plannedBreakMinutes * 60 - activeBreakElapsedSecs)
      : null;

  // Auto-end break when countdown reaches 0
  useEffect(() => {
    if (
      countdownRemainingSecs !== null &&
      countdownRemainingSecs <= 0 &&
      isOnBreak &&
      !actionLoading
    ) {
      void (async () => {
        setActionLoading(true);
        try {
          await onEndBreak();
          setPlannedBreakMinutes(null);
        } finally {
          setActionLoading(false);
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

  // Break compliance warning (only shown when checked in)
  const breakWarning =
    isCheckedIn && currentSession
      ? getBreakWarning(netMinutes, liveBreakMins)
      : null;

  // Checked-out net
  const checkedOutNet =
    isCheckedOut && currentSession
      ? calculateNetMinutes(
          currentSession.checkInTime,
          currentSession.checkOutTime,
          currentSession.breakMinutes,
        )
      : null;

  const handleCheckIn = async () => {
    if (mode === "absent") return;
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
      await onStartBreak();
    } finally {
      setActionLoading(false);
    }
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

  return (
    <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="relative p-5 sm:p-6 md:p-8">
        {/* Title + status badge */}
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-base font-bold text-gray-900 sm:text-lg">
            Stempeluhr
          </h2>
          {isCheckedIn &&
            currentSession &&
            (() => {
              const badge = getSessionStatusBadge(
                isOnBreak,
                currentSession.status,
              );
              return (
                <span
                  className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${badge.className}`}
                >
                  {badge.label}
                </span>
              );
            })()}
        </div>

        {/* ── Not checked in: start controls ── */}
        {!isCheckedIn && !isCheckedOut && (
          <div className="flex flex-col items-center gap-5">
            {/* Mode toggle */}
            <div className="flex gap-2">
              <button
                onClick={() => setMode("present")}
                className={getModeToggleClassName("present", mode)}
              >
                In der OGS
              </button>
              <button
                onClick={() => setMode("home_office")}
                className={getModeToggleClassName("home_office", mode)}
              >
                Homeoffice
              </button>
              <button
                onClick={() => setMode("absent")}
                className={getModeToggleClassName("absent", mode)}
              >
                Abwesend
              </button>
            </div>

            {/* Action button: play (check-in) or calendar (absence) */}
            {mode === "absent" ? (
              <button
                onClick={onAddAbsence}
                className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-red-400 text-red-500 transition-all hover:bg-red-50 active:scale-95"
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
              </button>
            ) : (
              <button
                onClick={handleCheckIn}
                disabled={actionLoading}
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
              </button>
            )}

            <span className="text-xs text-gray-400 sm:text-sm">
              {mode === "absent" ? "Abwesenheit melden" : "Einstempeln"}
            </span>
          </div>
        )}

        {/* ── Checked in: timer controls ── */}
        {isCheckedIn && (
          <>
            {/* Control row: pause | timer | stop */}
            <div className="mb-5 flex items-center justify-between">
              {/* Pause button (opens menu) or End Break button */}
              <div className="relative">
                {isOnBreak ? (
                  <button
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
                  </button>
                ) : (
                  <button
                    onClick={() => setBreakMenuOpen(!breakMenuOpen)}
                    disabled={actionLoading}
                    className={getBreakButtonClassName(false, breakMins)}
                    aria-label="Pause starten"
                  >
                    <svg
                      className="h-5 w-5"
                      fill="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
                    </svg>
                  </button>
                )}

                {/* Break duration menu */}
                {breakMenuOpen && !isOnBreak && (
                  <>
                    {/* Backdrop to close menu */}
                    <button
                      type="button"
                      aria-label="Menü schließen"
                      className="fixed inset-0 z-10 cursor-default bg-transparent"
                      onClick={() => setBreakMenuOpen(false)}
                      onKeyDown={(e) => {
                        if (e.key === "Escape" || e.key === "Enter")
                          setBreakMenuOpen(false);
                      }}
                    />
                    <div className="absolute top-full left-0 z-20 mt-2 flex gap-1.5 rounded-xl border border-gray-200 bg-white p-2 shadow-lg">
                      {BREAK_OPTIONS.map((mins) => (
                        <button
                          key={mins}
                          onClick={() => handleSelectBreakDuration(mins)}
                          disabled={actionLoading}
                          className="rounded-lg bg-amber-50 px-3 py-1.5 text-sm font-medium text-amber-700 transition-all hover:bg-amber-100 active:scale-95 disabled:opacity-50"
                        >
                          {mins}m
                        </button>
                      ))}
                    </div>
                  </>
                )}
              </div>

              {/* Timer display */}
              <div className="flex flex-col items-center">
                {(() => {
                  // Break with countdown timer
                  if (isOnBreak && countdownRemainingSecs !== null) {
                    return (
                      <>
                        <span className="text-4xl font-light text-amber-500 tabular-nums">
                          {formatCountdown(countdownRemainingSecs)}
                        </span>
                        <span className="mt-0.5 text-xs font-medium text-amber-500">
                          Pause ({plannedBreakMinutes} Min)
                        </span>
                      </>
                    );
                  }
                  // Break without countdown (manual)
                  if (isOnBreak) {
                    return (
                      <>
                        <span className="text-4xl font-light text-amber-500 tabular-nums">
                          {formatCountdown(activeBreakElapsedSecs)}
                        </span>
                        <span className="mt-0.5 text-xs font-medium text-amber-500">
                          Pause läuft
                        </span>
                      </>
                    );
                  }
                  // Working time display
                  return (
                    <>
                      <span className="text-4xl font-light text-gray-900 tabular-nums">
                        {formatHMM(displayMinutes)}
                      </span>
                      {breakWarning && (
                        <span className="mt-0.5 text-xs font-medium text-amber-600">
                          {breakWarning}
                        </span>
                      )}
                    </>
                  );
                })()}
              </div>

              {/* Stop / check-out button */}
              <button
                onClick={handleCheckOut}
                disabled={actionLoading || isOnBreak}
                className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-gray-300 text-gray-500 transition-all hover:border-red-400 hover:text-red-500 active:scale-95 disabled:opacity-50"
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
              </button>
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

        {/* Today/Week footer */}
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
      </div>
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
    if (seg.type === "break" && seg.isActive) return "text-amber-600";
    if (seg.type === "break") return "text-gray-500";
    return "";
  };

  // Returns label text color class based on segment type and state
  const getSegmentLabelColor = (seg: {
    type: "work" | "break";
    isActive: boolean;
  }) => {
    if (seg.type === "work" && seg.isActive) return "text-[#83CD2D]";
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
      key={`${seg.type}-${index}`}
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
        className={`shrink-0 ${seg.type === "break" && seg.isActive ? "text-amber-300" : "text-gray-300"}`}
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
        <button
          onClick={() => setExpanded(!expanded)}
          className="w-full border-t border-gray-50 py-1.5 text-center text-xs font-medium text-gray-400 transition-colors hover:text-gray-600"
        >
          {expanded
            ? "Weniger anzeigen"
            : `${hiddenSegments.length} weitere anzeigen`}
        </button>
      )}
    </>
  );
}

// ─── WeekChart ───────────────────────────────────────────────────────────────

const weekChartConfig = {
  netMinutes: { label: "Arbeitszeit", color: "#0ea5e9" }, // sky-500 — matches sidebar icon
  breakMinutes: { label: "Pause", color: "#94a3b8" }, // slate-400 — muted secondary
} satisfies ChartConfig;

function WeekChart({
  history,
  currentSession,
  weekOffset,
}: {
  readonly history: WorkSessionHistory[];
  readonly currentSession: WorkSession | null;
  readonly weekOffset: number;
}) {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 768);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  const today = new Date();

  const chartData = useMemo(() => {
    // Build 2 weeks of workdays (Mon-Fri): previous week + current week
    const referenceDate = new Date(today);
    referenceDate.setDate(referenceDate.getDate() + weekOffset * 7);
    const currentWeek = getWeekDays(referenceDate);
    const prevRef = new Date(referenceDate);
    prevRef.setDate(prevRef.getDate() - 7);
    const prevWeek = getWeekDays(prevRef);

    // Combine both weeks, filter weekends (Sat=5, Sun=6)
    const allDays = [...prevWeek, ...currentWeek].filter(
      (d) => d && d.getDay() !== 0 && d.getDay() !== 6,
    );

    const sessionMap = new Map<string, WorkSessionHistory>();
    for (const session of history) {
      sessionMap.set(session.date, session);
    }

    return allDays.map((day) => {
      if (!day) return { day: "", label: "", netMinutes: 0, breakMinutes: 0 };

      const dateKey = toISODate(day);
      const session = sessionMap.get(dateKey);
      const dayIndex = (day.getDay() + 6) % 7; // Mon=0..Sun=6

      let netMins = 0;
      let breakMins = 0;

      if (session) {
        if (session.checkOutTime) {
          netMins = session.netMinutes;
        } else if (
          isSameDay(day, today) &&
          currentSession &&
          !currentSession.checkOutTime
        ) {
          const elapsed = Math.floor(
            (Date.now() - new Date(session.checkInTime).getTime()) / 60000,
          );
          netMins = Math.max(0, elapsed - session.breakMinutes);
        }
        breakMins = session.breakMinutes;
      }

      return {
        day: DAY_NAMES[dayIndex] ?? "",
        label: `${DAY_NAMES[dayIndex] ?? ""} ${formatDateShort(day)}`,
        netMinutes: netMins,
        breakMinutes: breakMins,
      };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [history, currentSession, weekOffset]);

  const tooltipLabelFormatter = useCallback(
    (_value: unknown, payload: Array<{ payload?: { label?: string } }>) => {
      const item = payload[0]?.payload;
      return item?.label ?? "";
    },
    [],
  );

  const tooltipValueFormatter = useCallback(
    (
      value: number | string | Array<number | string>,
      name: string | number,
    ) => {
      const totalMins = value as number;
      const hours = Math.floor(totalMins / 60);
      const mins = totalMins % 60;
      const label = name === "netMinutes" ? "Arbeitszeit" : "Pause";
      return `${label}: ${hours}h ${mins}min`;
    },
    [],
  );

  return (
    <div className="relative flex h-full flex-col overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="flex min-h-0 flex-1 flex-col p-4 sm:p-6 md:p-8">
        <div className="mb-3 flex items-baseline justify-between sm:mb-4">
          <h2 className="text-base font-bold text-gray-900 sm:text-lg">
            Wochenübersicht
          </h2>
          {chartData.length > 0 && (
            <span className="text-[10px] text-gray-400 sm:text-xs">
              {chartData[0]?.label?.split(" ")[1] ?? ""} –{" "}
              {chartData[chartData.length - 1]?.label?.split(" ")[1] ?? ""}
            </span>
          )}
        </div>
        <ChartContainer
          config={weekChartConfig}
          className="!aspect-auto min-h-0 flex-1"
        >
          <BarChart
            accessibilityLayer
            data={chartData}
            margin={{
              top: 4,
              right: 4,
              bottom: 0,
              left: isMobile ? -24 : -20,
            }}
            barCategoryGap={isMobile ? 2 : 4}
          >
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="day"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              fontSize={isMobile ? 10 : 11}
              interval={0}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={4}
              fontSize={isMobile ? 10 : 12}
              tickFormatter={(v: number) => `${Math.round(v / 60)}h`}
              domain={[0, "auto"]}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelFormatter={tooltipLabelFormatter}
                  formatter={tooltipValueFormatter}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent />} />
            <Bar
              dataKey="breakMinutes"
              stackId="a"
              fill="var(--color-breakMinutes)"
              radius={[0, 0, 4, 4]}
            />
            <Bar
              dataKey="netMinutes"
              stackId="a"
              fill="var(--color-netMinutes)"
              radius={[4, 4, 0, 0]}
            />
          </BarChart>
        </ChartContainer>
      </div>
    </div>
  );
}

// ─── ExportDropdown ───────────────────────────────────────────────────────────

function ExportDropdown({ weekDays }: { readonly weekDays: (Date | null)[] }) {
  const monday = weekDays[0];
  const sunday = weekDays[6];
  const [rangeFrom, setRangeFrom] = useState<Date | null>(null);
  const [rangeTo, setRangeTo] = useState<Date | null>(null);
  const [open, setOpen] = useState(false);
  const [viewMonth, setViewMonth] = useState(() => new Date());
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  // Pre-fill with current week when dropdown opens
  useEffect(() => {
    if (open && monday && sunday) {
      setRangeFrom(monday);
      setRangeTo(sunday);
      setViewMonth(new Date(monday));
    }
  }, [open, monday, sunday]);

  // Close on outside click or scroll
  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      const target = e.target as Node;
      if (
        triggerRef.current?.contains(target) ||
        panelRef.current?.contains(target)
      )
        return;
      setOpen(false);
    }
    function handleScroll() {
      setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    window.addEventListener("scroll", handleScroll, true);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      window.removeEventListener("scroll", handleScroll, true);
    };
  }, [open]);

  const handleExport = (format: "csv" | "xlsx") => {
    if (!rangeFrom || !rangeTo) return;
    const from = toISODate(rangeFrom);
    const to = toISODate(rangeTo);
    globalThis.location.href = `/api/time-tracking/export?from=${from}&to=${to}&format=${format}`;
    setOpen(false);
  };

  const handleDayClick = (day: Date) => {
    if (!rangeFrom || (rangeFrom && rangeTo)) {
      // Start a new range
      setRangeFrom(day);
      setRangeTo(null);
    } else if (isBeforeDay(day, rangeFrom)) {
      // Complete the range (selected day is before start)
      setRangeTo(rangeFrom);
      setRangeFrom(day);
    } else {
      // Complete the range (selected day is after start)
      setRangeTo(day);
    }
  };

  const hasRange = rangeFrom && rangeTo;

  // Position the portal panel below the trigger button
  const [pos, setPos] = useState({ top: 0, right: 0 });
  useEffect(() => {
    if (!open || !triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    setPos({
      top: rect.bottom + window.scrollY + 8,
      right: window.innerWidth - rect.right - window.scrollX,
    });
  }, [open]);

  return (
    <>
      <button
        ref={triggerRef}
        onClick={() => setOpen((v) => !v)}
        className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
        aria-label="Export"
      >
        <Download className="h-5 w-5" />
      </button>
      {open &&
        createPortal(
          <div
            ref={panelRef}
            style={{ top: pos.top, right: pos.right }}
            className="fixed z-50 w-[320px] rounded-xl border border-gray-200 bg-white shadow-lg"
          >
            <div className="p-4 pb-2">
              <p className="text-sm font-medium text-gray-700">
                Zeitraum exportieren
              </p>
              {(() => {
                if (hasRange) {
                  return (
                    <p className="mt-1 text-xs text-gray-500">
                      {formatDateGerman(rangeFrom)} –{" "}
                      {formatDateGerman(rangeTo)}
                    </p>
                  );
                }
                if (rangeFrom) {
                  return (
                    <p className="mt-1 text-xs text-gray-500">
                      {formatDateGerman(rangeFrom)} – …
                    </p>
                  );
                }
                return null;
              })()}
            </div>
            <MiniCalendar
              viewMonth={viewMonth}
              onViewMonthChange={setViewMonth}
              rangeFrom={rangeFrom}
              rangeTo={rangeTo}
              onDayClick={handleDayClick}
            />
            <div className="flex gap-2 border-t border-gray-100 p-4 pt-3">
              <button
                onClick={() => handleExport("csv")}
                disabled={!hasRange}
                className="flex-1 rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
              >
                CSV
              </button>
              <button
                onClick={() => handleExport("xlsx")}
                disabled={!hasRange}
                className="flex-1 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
              >
                Excel
              </button>
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}

// ─── MiniCalendar ─────────────────────────────────────────────────────────────

const WEEKDAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
const MONTH_NAMES = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
];

function getDaysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate();
}

/** Monday-based day-of-week (0=Mon … 6=Sun) */
function mondayIndex(date: Date): number {
  return (date.getDay() + 6) % 7;
}

function MiniCalendar({
  viewMonth,
  onViewMonthChange,
  rangeFrom,
  rangeTo,
  onDayClick,
}: {
  readonly viewMonth: Date;
  readonly onViewMonthChange: (d: Date) => void;
  readonly rangeFrom: Date | null;
  readonly rangeTo: Date | null;
  readonly onDayClick: (d: Date) => void;
}) {
  const today = new Date();
  const year = viewMonth.getFullYear();
  const month = viewMonth.getMonth();
  const daysInMonth = getDaysInMonth(year, month);
  const firstDayOffset = mondayIndex(new Date(year, month, 1));

  const prevMonth = () => onViewMonthChange(new Date(year, month - 1, 1));
  const nextMonth = () => onViewMonthChange(new Date(year, month + 1, 1));

  // Build grid cells: leading blanks + day numbers
  const cells: (number | null)[] = [];
  for (let i = 0; i < firstDayOffset; i++) cells.push(null);
  for (let d = 1; d <= daysInMonth; d++) cells.push(d);

  const isInRange = (day: Date) => {
    if (!rangeFrom) return false;
    if (!rangeTo) return isSameDay(day, rangeFrom);
    return (
      (isSameDay(day, rangeFrom) || isBeforeDay(rangeFrom, day)) &&
      (isSameDay(day, rangeTo) || isBeforeDay(day, rangeTo))
    );
  };

  const isRangeStart = (day: Date) => rangeFrom && isSameDay(day, rangeFrom);
  const isRangeEnd = (day: Date) => rangeTo && isSameDay(day, rangeTo);
  const isFuture = (day: Date) => isBeforeDay(today, day);

  return (
    <div className="px-4 pb-2">
      {/* Month navigation */}
      <div className="mb-2 flex items-center justify-between">
        <button
          onClick={prevMonth}
          className="rounded-md p-1 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
          aria-label="Vorheriger Monat"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>
        <span className="text-sm font-medium text-gray-800">
          {MONTH_NAMES[month]} {year}
        </span>
        <button
          onClick={nextMonth}
          className="rounded-md p-1 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
          aria-label="Nächster Monat"
        >
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>

      {/* Weekday header */}
      <div className="grid grid-cols-7">
        {WEEKDAY_LABELS.map((label) => (
          <div
            key={label}
            className="flex h-9 items-center justify-center text-xs font-medium text-gray-400"
          >
            {label}
          </div>
        ))}

        {/* Day cells */}
        {cells.map((dayNum, idx) => {
          if (dayNum === null) {
            return <div key={`blank-${String(idx)}`} className="h-9" />;
          }

          const date = new Date(year, month, dayNum);
          const disabled = isFuture(date);
          const inRange = isInRange(date);
          const isStart = isRangeStart(date);
          const isEnd = isRangeEnd(date);
          const isToday = isSameDay(date, today);

          let cellBg = "";
          if (isStart || isEnd) {
            cellBg = "bg-gray-900 text-white";
          } else if (inRange) {
            cellBg = "bg-gray-100 text-gray-800";
          }

          // Rounding for range edges
          let rounding = "rounded-md";
          if (isStart && !isEnd) rounding = "rounded-l-md";
          else if (isEnd && !isStart) rounding = "rounded-r-md";
          else if (inRange && !isStart && !isEnd) rounding = "rounded-none";

          // Determine text/hover styles
          let interactionClass = "";
          if (disabled) {
            interactionClass = "cursor-not-allowed text-gray-300";
          } else if (!inRange) {
            interactionClass = "text-gray-700 hover:bg-gray-100";
          }

          return (
            <button
              key={dayNum}
              type="button"
              disabled={disabled}
              onClick={() => onDayClick(date)}
              className={`flex h-9 items-center justify-center text-sm transition-colors ${rounding} ${cellBg} ${interactionClass} ${isToday && !isStart && !isEnd ? "font-bold" : ""}`}
            >
              {dayNum}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ─── WeekTable ────────────────────────────────────────────────────────────────

function WeekTable({
  weekOffset,
  onWeekChange,
  history,
  absences,
  isLoading,
  onEditDay,
  currentSession,
  currentBreaks,
  expandedSessionId,
  onToggleExpand,
  expandedEdits,
  editsLoading,
}: {
  readonly weekOffset: number;
  readonly onWeekChange: (offset: number) => void;
  readonly history: WorkSessionHistory[];
  readonly absences: StaffAbsence[];
  readonly isLoading: boolean;
  readonly onEditDay: (
    date: Date,
    session: WorkSessionHistory | null,
    absence: StaffAbsence | null,
  ) => void;
  readonly currentSession: WorkSession | null;
  readonly currentBreaks: WorkSessionBreak[];
  readonly expandedSessionId: string | null;
  readonly onToggleExpand: (sessionId: string) => void;
  readonly expandedEdits: WorkSessionEdit[];
  readonly editsLoading: boolean;
}) {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 768);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  const today = new Date();
  const referenceDate = new Date(today);
  referenceDate.setDate(referenceDate.getDate() + weekOffset * 7);
  const weekDays = getWeekDays(referenceDate);
  const weekNum = getWeekNumber(referenceDate);
  const mondayDate = weekDays[0];
  const sundayDate = weekDays[6];

  // Build session map by date string
  const sessionMap = new Map<string, WorkSessionHistory>();
  for (const session of history) {
    sessionMap.set(session.date, session);
  }

  // Build absence map: for each date in the week, check if an absence covers it
  const absenceMap = new Map<string, StaffAbsence>();
  for (const day of weekDays) {
    if (!day) continue;
    const dateKey = toISODate(day);
    const absence = absences.find(
      (a) => a.dateStart <= dateKey && a.dateEnd >= dateKey,
    );
    if (absence) {
      absenceMap.set(dateKey, absence);
    }
  }

  // Live break minutes for active session (cached + active break elapsed)
  const activeBreak = currentBreaks.find((b) => !b.endedAt);
  const liveBreakMins = (() => {
    if (!currentSession || currentSession.checkOutTime) return 0;
    const cached = currentSession.breakMinutes;
    if (!activeBreak) return cached;
    const elapsed = Math.max(
      0,
      Math.floor(
        (Date.now() - new Date(activeBreak.startedAt).getTime()) / 60000,
      ),
    );
    return cached + elapsed;
  })();

  // Calculate weekly total (only sessions in the current week)
  const weekDateKeys = new Set(
    weekDays.filter(Boolean).map((d) => toISODate(d)),
  );
  const weeklyNetMinutes = history
    .filter((s) => weekDateKeys.has(s.date))
    .reduce((sum, s) => {
      if (s.checkOutTime) return sum + s.netMinutes;
      // For active session, calculate live with real break time
      if (currentSession && !currentSession.checkOutTime) {
        const live = calculateNetMinutes(
          s.checkInTime,
          new Date().toISOString(),
          liveBreakMins,
        );
        return sum + (live ?? 0);
      }
      return sum;
    }, 0);

  return (
    <div className="overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      {/* Week navigation header */}
      <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3 sm:px-6 sm:py-4">
        <button
          onClick={() => onWeekChange(weekOffset - 1)}
          className="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 sm:p-2"
          aria-label="Vorherige Woche"
        >
          <svg
            className="h-5 w-5"
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
        </button>
        <span className="text-sm font-bold text-gray-900 sm:text-base md:text-lg">
          KW {weekNum}: {mondayDate ? formatDateGerman(mondayDate) : ""} –{" "}
          {sundayDate ? formatDateGerman(sundayDate) : ""}
        </span>
        <div className="flex items-center gap-1">
          <button
            onClick={() => onWeekChange(weekOffset + 1)}
            disabled={weekOffset >= 0}
            className="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-30 disabled:hover:bg-transparent sm:p-2"
            aria-label="Nächste Woche"
          >
            <svg
              className="h-5 w-5"
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
          </button>
          <ExportDropdown weekDays={weekDays} />
        </div>
      </div>

      {/* Mobile card layout */}
      {isMobile ? (
        <div className="divide-y divide-gray-100 px-4 py-2">
          {weekDays.map((day, index) => {
            if (!day) return null;
            if (day.getDay() === 0 || day.getDay() === 6) return null;
            const dateKey = toISODate(day);
            const session = sessionMap.get(dateKey);
            const absence = absenceMap.get(dateKey);
            const isToday = isSameDay(day, today);
            const isPast = isBeforeDay(day, today);
            const isFuture = !isToday && !isPast;
            const dayName = DAY_NAMES[index];
            const isActive =
              isToday &&
              currentSession !== null &&
              currentSession.checkOutTime === null;
            const canEdit =
              session !== undefined && (isPast || (isToday && !isActive));
            const hasEdits = (session?.editCount ?? 0) > 0;

            let netDisplay = "--";
            if (session) {
              if (session.checkOutTime) {
                netDisplay = formatDuration(session.netMinutes);
              } else if (isActive) {
                const live = calculateNetMinutes(
                  session.checkInTime,
                  new Date().toISOString(),
                  liveBreakMins,
                );
                netDisplay = live == null ? "--" : formatDuration(live);
              }
            }

            const warnings = session ? getComplianceWarnings(session) : [];

            // Absence-only card — clickable to open edit modal
            if (absence && session == null) {
              const colorClass =
                absenceTypeColors[absence.absenceType] ??
                "bg-gray-100 text-gray-600";
              return (
                <button
                  key={dateKey}
                  type="button"
                  onClick={() => onEditDay(day, null, absence)}
                  className={`w-full cursor-pointer py-3 text-left transition-colors hover:bg-gray-50 ${isToday ? "rounded-lg bg-blue-50/50" : ""}`}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-700">
                      {dayName} {formatDateShort(day)}
                    </span>
                    <div className="flex items-center gap-2">
                      <span
                        className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${colorClass}`}
                      >
                        {absenceTypeLabels[absence.absenceType]}
                      </span>
                      <SquarePen className="h-3.5 w-3.5 text-gray-300" />
                    </div>
                  </div>
                </button>
              );
            }

            // Regular day card
            return (
              <div
                key={dateKey}
                className={`py-3 ${isToday ? "rounded-lg bg-blue-50/50 px-2" : ""} ${isFuture ? "opacity-40" : ""}`}
              >
                {/* Header: day name + status badge */}
                <div className="mb-2 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-700">
                      {dayName} {formatDateShort(day)}
                    </span>
                    {absence && (
                      <span
                        className={`rounded-full px-1.5 py-0.5 text-[10px] font-medium ${absenceTypeColors[absence.absenceType] ?? "bg-gray-100 text-gray-600"}`}
                      >
                        {absenceTypeLabels[absence.absenceType]}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {session &&
                      (() => {
                        let badgeClass = "bg-gray-100 text-gray-600";
                        let badgeLabel = "OGS";
                        if (isActive) {
                          badgeClass = "bg-green-100 text-green-700";
                          badgeLabel = "aktiv";
                        } else if (session.status === "home_office") {
                          badgeClass = "bg-sky-100 text-sky-700";
                          badgeLabel = "HO";
                        }
                        return (
                          <span
                            className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${badgeClass}`}
                          >
                            {badgeLabel}
                          </span>
                        );
                      })()}
                    {canEdit && session && (
                      <button
                        type="button"
                        onClick={() => onEditDay(day, session, absence ?? null)}
                        aria-label="Eintrag bearbeiten"
                      >
                        <SquarePen className="h-3.5 w-3.5 text-gray-300" />
                      </button>
                    )}
                  </div>
                </div>

                {/* 2x2 data grid */}
                {session ? (
                  <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                    <div>
                      <span className="text-gray-400">Start</span>
                      <p className="font-medium text-gray-600 tabular-nums">
                        {formatTime(session.checkInTime)}
                      </p>
                    </div>
                    <div>
                      <span className="text-gray-400">Ende</span>
                      <p className="font-medium text-gray-600 tabular-nums">
                        {session.checkOutTime
                          ? formatTime(session.checkOutTime)
                          : isActive
                            ? "···"
                            : "--:--"}
                      </p>
                    </div>
                    <div>
                      <span className="text-gray-400">Pause</span>
                      <p className="font-medium text-gray-600 tabular-nums">
                        {session.breakMinutes > 0
                          ? `${session.breakMinutes} Min`
                          : "--"}
                      </p>
                    </div>
                    <div>
                      <span className="text-gray-400">Netto</span>
                      <p className="flex items-center gap-1 font-medium text-gray-700 tabular-nums">
                        {netDisplay}
                        {warnings.length > 0 && (
                          <span
                            className="cursor-help text-amber-500"
                            title={warnings.join("\n")}
                          >
                            ⚠
                          </span>
                        )}
                      </p>
                    </div>
                  </div>
                ) : (
                  !isFuture && (
                    <p className="text-xs text-gray-300">Kein Eintrag</p>
                  )
                )}

                {/* Edit history footer */}
                {hasEdits && session && (
                  <button
                    type="button"
                    onClick={() => onToggleExpand(session.id)}
                    className="mt-2 flex w-full items-center gap-1 border-t border-gray-50 pt-2 text-[10px] text-gray-400"
                  >
                    <ChevronRight
                      className={`h-3 w-3 transition-transform ${expandedSessionId === session.id ? "rotate-90" : ""}`}
                    />
                    Geändert {formatDateTime(new Date(session.updatedAt))}
                  </button>
                )}
                {expandedSessionId === session?.id && session && (
                  <div className="mt-2 rounded-lg bg-gray-50 p-3">
                    <EditHistoryAccordion
                      edits={expandedEdits}
                      isLoading={editsLoading}
                      onEdit={
                        canEdit
                          ? () => onEditDay(day, session, absence ?? null)
                          : undefined
                      }
                    />
                  </div>
                )}
              </div>
            );
          })}

          {/* Weekly total card */}
          <div className="flex items-center justify-between py-3">
            <span className="text-sm font-medium text-gray-500">
              Woche gesamt
            </span>
            <span className="text-sm font-bold text-gray-700">
              {isLoading ? "..." : formatDuration(weeklyNetMinutes)}
            </span>
          </div>
        </div>
      ) : (
        /* Desktop table */
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 text-left text-xs font-medium tracking-wide text-gray-400 uppercase">
                <th className="py-3 pr-6 pl-[46px]">Tag</th>
                <th className="px-4 py-3 text-center">Start</th>
                <th className="px-4 py-3 text-center">Ende</th>
                <th className="px-4 py-3 text-center">Pause</th>
                <th className="px-4 py-3 text-center">Netto</th>
                <th className="px-4 py-3 text-center">Ort</th>
                <th className="px-4 py-3 text-center">Änderung</th>
              </tr>
            </thead>
            <tbody>
              {weekDays.map((day, index) => {
                if (!day) return null;
                if (day.getDay() === 0 || day.getDay() === 6) return null;
                const dateKey = toISODate(day);
                const session = sessionMap.get(dateKey);
                const absence = absenceMap.get(dateKey);
                const isToday = isSameDay(day, today);
                const isPast = isBeforeDay(day, today);
                const isFuture = !isToday && !isPast;
                const dayName = DAY_NAMES[index];
                const isActive =
                  isToday &&
                  currentSession !== null &&
                  currentSession.checkOutTime === null;
                const canEdit =
                  session !== undefined && (isPast || (isToday && !isActive));
                const hasEdits = (session?.editCount ?? 0) > 0;
                const isExpanded = expandedSessionId === session?.id;

                const handleRowClick = () => {
                  if (!canEdit && !hasEdits) return;
                  if (hasEdits && session) {
                    onToggleExpand(session.id);
                  } else if (canEdit && session) {
                    onEditDay(day, session, absence ?? null);
                  }
                };

                let netDisplay = "--";
                if (session) {
                  if (session.checkOutTime) {
                    netDisplay = formatDuration(session.netMinutes);
                  } else if (isActive) {
                    const live = calculateNetMinutes(
                      session.checkInTime,
                      new Date().toISOString(),
                      liveBreakMins,
                    );
                    netDisplay = live == null ? "--" : formatDuration(live);
                  }
                }

                const warnings = session ? getComplianceWarnings(session) : [];

                if (absence && session == null) {
                  const colorClass =
                    absenceTypeColors[absence.absenceType] ??
                    "bg-gray-100 text-gray-600";
                  return (
                    <tr
                      key={dateKey}
                      onClick={() => onEditDay(day, null, absence)}
                      className={`group/row cursor-pointer border-b border-gray-50 transition-colors hover:bg-gray-50 ${isToday ? "bg-blue-50/50" : ""}`}
                    >
                      <td className="px-6 py-3 font-medium text-gray-700">
                        <div className="flex items-center gap-1.5">
                          <span className="inline-block h-4 w-4 shrink-0" />
                          {dayName} {formatDateShort(day)}
                        </div>
                      </td>
                      <td colSpan={5} className="px-4 py-3 text-center">
                        <span
                          className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium ${colorClass}`}
                        >
                          {absenceTypeLabels[absence.absenceType]}
                          {absence.halfDay && " (halber Tag)"}
                          {absence.note && (
                            <span
                              className="cursor-help opacity-60"
                              title={absence.note}
                            >
                              ℹ
                            </span>
                          )}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-center">
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            onEditDay(day, null, absence);
                          }}
                          aria-label={`${absenceTypeLabels[absence.absenceType]} am ${dayName} ${formatDateShort(day)} bearbeiten`}
                          className="inline-flex opacity-0 transition-opacity group-hover/row:opacity-100 focus:opacity-100"
                        >
                          <SquarePen className="h-3.5 w-3.5 text-gray-300" />
                        </button>
                      </td>
                    </tr>
                  );
                }

                return (
                  <React.Fragment key={dateKey}>
                    <tr
                      onClick={canEdit || hasEdits ? handleRowClick : undefined}
                      className={`group/row border-b border-gray-50 transition-colors ${(() => {
                        if (isExpanded) return "bg-gray-50";
                        if (isToday) return "bg-blue-50/50";
                        return "";
                      })()} ${canEdit || hasEdits ? "cursor-pointer hover:bg-gray-50" : ""} ${
                        isFuture ? "opacity-40" : ""
                      }`}
                    >
                      <td className="px-6 py-3 font-medium text-gray-700">
                        <div className="flex items-center gap-1.5">
                          {hasEdits ? (
                            <ChevronRight
                              className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${isExpanded ? "rotate-90" : ""}`}
                            />
                          ) : (
                            <span className="inline-block h-4 w-4 shrink-0" />
                          )}
                          {dayName} {formatDateShort(day)}
                          {absence && (
                            <span
                              className={`ml-1 inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium ${absenceTypeColors[absence.absenceType] ?? "bg-gray-100 text-gray-600"}`}
                            >
                              {absenceTypeLabels[absence.absenceType]}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-center text-gray-600">
                        {session ? formatTime(session.checkInTime) : "--:--"}
                      </td>
                      <td className="px-4 py-3 text-center text-gray-600">
                        {(() => {
                          if (!session) return "--:--";
                          if (session.checkOutTime)
                            return formatTime(session.checkOutTime);
                          if (isActive) return "···";
                          return "--:--";
                        })()}
                      </td>
                      <td className="px-4 py-3 text-center text-gray-600">
                        {session && session.breakMinutes > 0
                          ? `${session.breakMinutes}`
                          : "--"}
                      </td>
                      <td className="px-4 py-3 text-center">
                        <span className="font-medium text-gray-700">
                          {netDisplay}
                        </span>
                        {warnings.length > 0 && (
                          <span
                            className="ml-1 cursor-help text-amber-500"
                            title={warnings.join("\n")}
                          >
                            ⚠
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-center">
                        {(() => {
                          if (session) {
                            let badgeClass = "bg-gray-100 text-gray-600";
                            let badgeLabel = "In der OGS";
                            if (isActive) {
                              badgeClass = "bg-green-100 text-green-700";
                              badgeLabel = "aktiv";
                            } else if (session.status === "home_office") {
                              badgeClass = "bg-sky-100 text-sky-700";
                              badgeLabel = "Homeoffice";
                            }
                            return (
                              <span
                                className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${badgeClass}`}
                              >
                                {badgeLabel}
                              </span>
                            );
                          }
                          if (isFuture) return null;
                          return <span className="text-gray-300">—</span>;
                        })()}
                      </td>
                      <td className="px-4 py-3 text-center">
                        {(() => {
                          if (hasEdits && session) {
                            return (
                              <span className="text-xs text-gray-500">
                                Zuletzt geändert{" "}
                                {formatDateTime(new Date(session.updatedAt))}
                              </span>
                            );
                          }
                          if (canEdit && session) {
                            return (
                              <div className="flex items-center justify-center">
                                <span className="text-xs text-gray-300 group-hover/row:hidden">
                                  –
                                </span>
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    onEditDay(day, session, absence ?? null);
                                  }}
                                  className="hidden group-hover/row:block"
                                  aria-label="Eintrag bearbeiten"
                                >
                                  <SquarePen className="h-3.5 w-3.5 text-gray-300" />
                                </button>
                              </div>
                            );
                          }
                          return (
                            <div className="flex items-center justify-center">
                              <span className="text-xs text-gray-300">–</span>
                            </div>
                          );
                        })()}
                      </td>
                    </tr>
                    {isExpanded && session && (
                      <tr className="border-b border-gray-50 bg-gray-50/50">
                        <td colSpan={7} className="py-3 pr-6 pl-[46px]">
                          <EditHistoryAccordion
                            edits={expandedEdits}
                            isLoading={editsLoading}
                            onEdit={
                              canEdit
                                ? () => onEditDay(day, session, absence ?? null)
                                : undefined
                            }
                          />
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                );
              })}
            </tbody>
            <tfoot>
              <tr className="border-t border-gray-200 bg-gray-50/50">
                <td
                  colSpan={4}
                  className="px-6 py-3 text-right text-sm font-medium text-gray-500"
                >
                  Woche gesamt
                </td>
                <td className="px-4 py-3 text-center text-sm font-bold text-gray-700">
                  {isLoading ? "..." : formatDuration(weeklyNetMinutes)}
                </td>
                <td colSpan={3} />
              </tr>
            </tfoot>
          </table>
        </div>
      )}
    </div>
  );
}

// ─── EditHistoryAccordion ──────────────────────────────────────────────────────

const FIELD_LABELS: Record<string, string> = {
  check_in_time: "Start",
  check_out_time: "Ende",
  break_minutes: "Pause",
  break_duration: "Pausendauer",
  status: "Ort",
  notes: "Notiz",
};

function formatEditValue(fieldName: string, value: string | null): string {
  if (value === null || value === "") return "–";
  if (fieldName === "check_in_time" || fieldName === "check_out_time") {
    return formatTime(value);
  }
  if (fieldName === "break_minutes" || fieldName === "break_duration") {
    return `${value} min`;
  }
  if (fieldName === "status") {
    return value === "home_office" ? "Homeoffice" : "In der OGS";
  }
  return value;
}

function EditHistoryAccordion({
  edits,
  isLoading,
  onEdit,
}: {
  readonly edits: WorkSessionEdit[];
  readonly isLoading: boolean;
  readonly onEdit?: () => void;
}) {
  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-3">
        <div className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-transparent" />
        <span className="ml-2 text-xs text-gray-400">Laden...</span>
      </div>
    );
  }

  if (edits.length === 0) {
    return (
      <p className="py-2 text-xs text-gray-400">Keine Änderungen vorhanden.</p>
    );
  }

  // Group edits by createdAt timestamp (edits from same save action share the same timestamp)
  const grouped = new Map<string, WorkSessionEdit[]>();
  for (const edit of edits) {
    const key = edit.createdAt;
    const existing = grouped.get(key);
    if (existing) {
      existing.push(edit);
    } else {
      grouped.set(key, [edit]);
    }
  }

  // Flatten into rows: one row per edit group (same timestamp)
  const rows = Array.from(grouped.entries()).map(([timestamp, group]) => {
    const date = new Date(timestamp);
    const dateStr = `${date.getDate().toString().padStart(2, "0")}.${(date.getMonth() + 1).toString().padStart(2, "0")}.${date.getFullYear()}, ${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
    const fieldEdits = group.filter((e) => e.fieldName !== "notes");
    const notes = group[0]?.notes;
    return { timestamp, dateStr, fieldEdits, notes };
  });

  return (
    <div>
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-gray-100 text-left text-[10px] font-medium tracking-wide text-gray-400 uppercase">
            <th className="pr-4 pb-2">Datum</th>
            <th className="pr-4 pb-2">Feld</th>
            <th className="pr-4 pb-2">Vorher</th>
            <th className="pr-4 pb-2" />
            <th className="pr-4 pb-2">Nachher</th>
            <th className="pb-2">Grund</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ dateStr, fieldEdits, notes }) =>
            fieldEdits.map((edit, idx) => (
              <tr
                key={edit.id}
                className="border-b border-gray-50 last:border-b-0"
              >
                <td className="py-1.5 pr-4 whitespace-nowrap text-gray-500">
                  {idx === 0 ? dateStr : ""}
                </td>
                <td className="py-1.5 pr-4 whitespace-nowrap text-gray-600">
                  {FIELD_LABELS[edit.fieldName] ?? edit.fieldName}
                </td>
                <td className="py-1.5 pr-4 whitespace-nowrap text-red-400 line-through">
                  {formatEditValue(edit.fieldName, edit.oldValue)}
                </td>
                <td className="py-1.5 pr-4 text-gray-300">&rarr;</td>
                <td className="py-1.5 pr-4 font-medium whitespace-nowrap text-[#83cd2d]">
                  {formatEditValue(edit.fieldName, edit.newValue)}
                </td>
                <td className="py-1.5 text-gray-400 italic">
                  {idx === 0 && notes ? (
                    <span>&ldquo;{notes}&rdquo;</span>
                  ) : null}
                </td>
              </tr>
            )),
          )}
        </tbody>
      </table>
      {onEdit && (
        <button
          type="button"
          onClick={onEdit}
          className="mt-2 flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-gray-300 py-1.5 text-xs text-gray-400 transition-colors hover:border-gray-400 hover:bg-gray-50 hover:text-gray-600"
        >
          <SquarePen className="h-3.5 w-3.5" />
          Weitere Änderung vornehmen
        </button>
      )}
    </div>
  );
}

// ─── EditModal ────────────────────────────────────────────────────────────────

const BREAK_DURATION_OPTIONS = [0, 15, 30, 45, 60] as const;

function EditSessionModal({
  isOpen,
  onClose,
  session,
  date,
  onSave,
  absence,
  onUpdateAbsence,
  onDeleteAbsence,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly session: WorkSessionHistory | null;
  readonly date: Date | null;
  readonly onSave: (
    id: string,
    updates: {
      checkInTime?: string;
      checkOutTime?: string;
      breakMinutes?: number;
      status?: "present" | "home_office";
      notes?: string;
      breaks?: Array<{ id: string; durationMinutes: number }>;
    },
  ) => Promise<void>;
  readonly absence: StaffAbsence | null;
  readonly onUpdateAbsence: (
    id: string,
    req: {
      absence_type?: string;
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
  const [status, setStatus] = useState<"present" | "home_office">("present");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);

  // Absence state
  const [absType, setAbsType] = useState<AbsenceType>("sick");
  const [absDateStart, setAbsDateStart] = useState("");
  const [absDateEnd, setAbsDateEnd] = useState("");
  const [absHalfDay, setAbsHalfDay] = useState(false);
  const [absNote, setAbsNote] = useState("");
  const [absenceSaving, setAbsenceSaving] = useState(false);
  const [absenceDeleting, setAbsenceDeleting] = useState(false);

  const [activeTab, setActiveTab] = useState<"session" | "absence">("session");

  const hasIndividualBreaks = (session?.breaks.length ?? 0) > 0;
  const hasSession = session !== null;
  const hasAbsence = absence !== null;
  const hasBoth = hasSession && hasAbsence;

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
      setAbsType(absence.absenceType);
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

  if (!date || (!hasSession && !hasAbsence)) return null;

  const dayIndex = (date.getDay() + 6) % 7;
  const dayName = DAY_NAMES_LONG[dayIndex] ?? "";

  // Calculate total break from individual breaks or fallback dropdown
  const editedBreak = hasIndividualBreaks
    ? Array.from(breakDurations.values()).reduce((sum, d) => sum + d, 0)
    : Number.parseInt(breakMins, 10) || 0;

  // Compliance check for the edited values
  const warnings: string[] = [];
  if (startTime && endTime) {
    const [sh, sm] = startTime.split(":").map(Number);
    const [eh, em] = endTime.split(":").map(Number);
    if (
      sh !== undefined &&
      sm !== undefined &&
      eh !== undefined &&
      em !== undefined
    ) {
      const totalMins = eh * 60 + em - (sh * 60 + sm);
      const netMins = totalMins - editedBreak;
      if (netMins > 600) {
        warnings.push("Arbeitszeit > 10h (§3 ArbZG)");
      }
      if (netMins > 540 && editedBreak < 45) {
        warnings.push("Pausenzeit < 45 Min bei > 9h Arbeitszeit (§4 ArbZG)");
      } else if (netMins > 360 && editedBreak < 30) {
        warnings.push("Pausenzeit < 30 Min bei > 6h Arbeitszeit (§4 ArbZG)");
      }
    }
  }

  const handleBreakDurationChange = (breakId: string, minutes: number) => {
    setBreakDurations((prev) => {
      const next = new Map(prev);
      next.set(breakId, minutes);
      return next;
    });
  };

  const handleSave = async () => {
    if (!session) return;
    setSaving(true);
    try {
      const dateStr = toISODate(date);

      // Build ISO timestamps from time inputs + date (with local timezone)
      const tzOffset = -new Date().getTimezoneOffset();
      const tzSign = tzOffset >= 0 ? "+" : "-";
      const tzH = String(Math.floor(Math.abs(tzOffset) / 60)).padStart(2, "0");
      const tzM = String(Math.abs(tzOffset) % 60).padStart(2, "0");
      const tz = `${tzSign}${tzH}:${tzM}`;

      const buildTimestamp = (time: string) => {
        if (!time) return undefined;
        return `${dateStr}T${time}:00${tz}`;
      };

      if (hasIndividualBreaks) {
        // Build breaks array with only changed durations
        const changedBreaks: Array<{ id: string; durationMinutes: number }> =
          [];
        for (const brk of session.breaks) {
          const newDuration = breakDurations.get(brk.id);
          if (
            newDuration !== undefined &&
            newDuration !== brk.durationMinutes
          ) {
            changedBreaks.push({ id: brk.id, durationMinutes: newDuration });
          }
        }

        await onSave(session.id, {
          checkInTime: buildTimestamp(startTime),
          checkOutTime: buildTimestamp(endTime) ?? undefined,
          status,
          notes: notes || undefined,
          breaks: changedBreaks.length > 0 ? changedBreaks : undefined,
        });
      } else {
        await onSave(session.id, {
          checkInTime: buildTimestamp(startTime),
          checkOutTime: buildTimestamp(endTime) ?? undefined,
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
        absence_type: absType,
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

  // Dynamic title based on what's present
  const modalTitle = hasBoth
    ? "Tag bearbeiten"
    : hasAbsence
      ? "Abwesenheit bearbeiten"
      : "Eintrag bearbeiten";

  // Footer: tab-aware for dual-section, standard for single-section
  const sessionFooter = (
    <div className="flex w-full gap-3">
      <button
        onClick={onClose}
        className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        onClick={handleSave}
        disabled={saving || !startTime || !notes.trim()}
        className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {saving ? "Speichern..." : "Speichern"}
      </button>
    </div>
  );

  const absenceFooter = (
    <div className="flex w-full gap-3">
      <button
        onClick={onClose}
        className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        onClick={handleAbsenceSave}
        disabled={absenceSaving || !absDateStart || !absDateEnd}
        className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {absenceSaving ? "Speichern..." : "Speichern"}
      </button>
    </div>
  );

  const footer = (() => {
    if (hasBoth) {
      return activeTab === "session" ? sessionFooter : absenceFooter;
    }
    return hasAbsence ? absenceFooter : sessionFooter;
  })();

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={modalTitle} footer={footer}>
      <div className="space-y-4">
        <p className="text-sm font-medium text-gray-500">
          {dayName}, {formatDateGerman(date)}
        </p>

        {/* ── Tabs (only when both session + absence exist) ────────────── */}
        {hasBoth && (
          <div className="flex gap-1 rounded-lg bg-gray-100 p-1">
            <button
              type="button"
              onClick={() => setActiveTab("session")}
              className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-all ${
                activeTab === "session"
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              Arbeitszeit
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("absence")}
              className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-all ${
                activeTab === "absence"
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              Abwesenheit
            </button>
          </div>
        )}

        {/* ── Session section ──────────────────────────────────────────── */}
        {hasSession && (!hasBoth || activeTab === "session") && (
          <>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label
                  htmlFor="edit-start"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Start
                </label>
                <input
                  id="edit-start"
                  type="time"
                  value={startTime}
                  onChange={(e) => setStartTime(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                />
              </div>
              <div>
                <label
                  htmlFor="edit-end"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Ende
                </label>
                <input
                  id="edit-end"
                  type="time"
                  value={endTime}
                  onChange={(e) => setEndTime(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
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
                          <div className="relative flex-1">
                            <select
                              value={(
                                breakDurations.get(brk.id) ??
                                brk.durationMinutes
                              ).toString()}
                              onChange={(e) =>
                                handleBreakDurationChange(
                                  brk.id,
                                  Number.parseInt(e.target.value, 10),
                                )
                              }
                              className="w-full appearance-none rounded-lg border border-gray-300 px-3 py-1.5 pr-8 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                            >
                              {BREAK_DURATION_OPTIONS.map((m) => (
                                <option key={m} value={m.toString()}>
                                  {m} min
                                </option>
                              ))}
                            </select>
                            <svg
                              className="pointer-events-none absolute top-1/2 right-2.5 h-4 w-4 -translate-y-1/2 text-gray-400"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M19 9l-7 7-7-7"
                              />
                            </svg>
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
                      htmlFor="edit-break"
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Pause (Min)
                    </label>
                    <div className="relative">
                      <select
                        id="edit-break"
                        value={breakMins}
                        onChange={(e) => setBreakMins(e.target.value)}
                        className="w-full appearance-none rounded-lg border border-gray-300 px-3 py-2 pr-8 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                      >
                        {[0, 15, 30, 45, 60].map((m) => (
                          <option key={m} value={m.toString()}>
                            {m} min
                          </option>
                        ))}
                      </select>
                      <svg
                        className="pointer-events-none absolute top-1/2 right-2.5 h-4 w-4 -translate-y-1/2 text-gray-400"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M19 9l-7 7-7-7"
                        />
                      </svg>
                    </div>
                  </div>
                )}
              </div>
              <div>
                <label
                  htmlFor="edit-status"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Ort
                </label>
                <div className="relative">
                  <select
                    id="edit-status"
                    value={status}
                    onChange={(e) =>
                      setStatus(e.target.value as "present" | "home_office")
                    }
                    className="w-full appearance-none rounded-lg border border-gray-300 px-3 py-2 pr-8 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                  >
                    <option value="present">In der OGS</option>
                    <option value="home_office">Homeoffice</option>
                  </select>
                  <svg
                    className="pointer-events-none absolute top-1/2 right-2.5 h-4 w-4 -translate-y-1/2 text-gray-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </div>
              </div>
            </div>

            <div>
              <label
                htmlFor="edit-notes"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Grund der Änderung <span className="text-red-500">*</span>
              </label>
              <div className="mb-2 flex flex-wrap gap-1.5">
                {[
                  "Vergessen auszustempeln",
                  "Vergessen einzustempeln",
                  "Zeitkorrektur",
                  "Krankheit",
                  "Ort-Änderung",
                ].map((reason) => (
                  <button
                    key={reason}
                    type="button"
                    onClick={() => setNotes(reason)}
                    className={`rounded-full px-3 py-1 text-xs font-medium transition-all ${
                      notes === reason
                        ? "bg-gray-900 text-white"
                        : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                    }`}
                  >
                    {reason}
                  </button>
                ))}
              </div>
              <textarea
                id="edit-notes"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={2}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                placeholder="Oder eigenen Grund eingeben..."
              />
            </div>

            {warnings.length > 0 && (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3">
                {warnings.map((w) => (
                  <p key={w} className="text-xs font-medium text-amber-700">
                    ⚠ {w}
                  </p>
                ))}
              </div>
            )}
          </>
        )}

        {/* ── Absence section ──────────────────────────────────────────── */}
        {hasAbsence && (!hasBoth || activeTab === "absence") && (
          <>
            {/* Absence type */}
            <div>
              <label
                htmlFor="edit-abs-type"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Art der Abwesenheit
              </label>
              <div className="relative">
                <select
                  id="edit-abs-type"
                  value={absType}
                  onChange={(e) => setAbsType(e.target.value as AbsenceType)}
                  className="w-full appearance-none rounded-lg border border-gray-300 px-3 py-2 pr-8 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                >
                  {ABSENCE_TYPE_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                <svg
                  className="pointer-events-none absolute top-1/2 right-2.5 h-4 w-4 -translate-y-1/2 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </div>
            </div>

            {/* Date range */}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label
                  htmlFor="edit-abs-start"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Von
                </label>
                <input
                  id="edit-abs-start"
                  type="date"
                  value={absDateStart}
                  onChange={(e) => setAbsDateStart(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                  required
                />
              </div>
              <div>
                <label
                  htmlFor="edit-abs-end"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Bis
                </label>
                <input
                  id="edit-abs-end"
                  type="date"
                  value={absDateEnd}
                  min={absDateStart}
                  onChange={(e) => setAbsDateEnd(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                  required
                />
              </div>
            </div>

            {/* Half day toggle */}
            <div className="flex items-center gap-3">
              <button
                type="button"
                role="switch"
                aria-checked={absHalfDay}
                onClick={() => setAbsHalfDay(!absHalfDay)}
                className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full transition-colors ${absHalfDay ? "bg-gray-900" : "bg-gray-200"}`}
              >
                <span
                  className={`pointer-events-none inline-block h-4 w-4 translate-y-0.5 rounded-full bg-white transition-transform ${absHalfDay ? "translate-x-4.5" : "translate-x-0.5"}`}
                />
              </button>
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
                <span className="font-normal text-gray-400">(optional)</span>
              </label>
              <textarea
                id="edit-abs-note"
                value={absNote}
                onChange={(e) => setAbsNote(e.target.value)}
                rows={2}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
                placeholder="z.B. Arzttermin, Schulung ..."
              />
            </div>

            {/* Destructive action */}
            <div className="pt-1">
              <button
                type="button"
                onClick={handleAbsenceDelete}
                disabled={absenceDeleting}
                className="text-sm font-medium text-red-500 transition-colors hover:text-red-700 disabled:opacity-50"
              >
                {absenceDeleting
                  ? "Abwesenheit wird gelöscht..."
                  : "Abwesenheit löschen"}
              </button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}

// ─── CreateAbsenceModal ───────────────────────────────────────────────────────

const ABSENCE_TYPE_OPTIONS: { value: AbsenceType; label: string }[] = [
  { value: "sick", label: "Krank" },
  { value: "vacation", label: "Urlaub" },
  { value: "training", label: "Fortbildung" },
  { value: "other", label: "Sonstige" },
];

function CreateAbsenceModal({
  isOpen,
  onClose,
  onSave,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSave: (req: {
    absence_type: string;
    date_start: string;
    date_end: string;
    half_day?: boolean;
    note?: string;
  }) => Promise<void>;
}) {
  const todayStr = toISODate(new Date());
  const [absenceType, setAbsenceType] = useState<AbsenceType>("sick");
  const [dateStart, setDateStart] = useState(todayStr);
  const [dateEnd, setDateEnd] = useState(todayStr);
  const [halfDay, setHalfDay] = useState(false);
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);

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
        absence_type: absenceType,
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
        <div className="flex w-full gap-3">
          <button
            onClick={onClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !dateStart || !dateEnd}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving ? "Speichern..." : "Speichern"}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        {/* Absence type */}
        <div>
          <label
            htmlFor="absence-type"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Art der Abwesenheit
          </label>
          <div className="relative">
            <select
              id="absence-type"
              value={absenceType}
              onChange={(e) => setAbsenceType(e.target.value as AbsenceType)}
              className="w-full appearance-none rounded-lg border border-gray-300 px-3 py-2 pr-8 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
            >
              {ABSENCE_TYPE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            <svg
              className="pointer-events-none absolute top-1/2 right-2.5 h-4 w-4 -translate-y-1/2 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 9l-7 7-7-7"
              />
            </svg>
          </div>
        </div>

        {/* Date range */}
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="absence-start"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Von
            </label>
            <input
              id="absence-start"
              type="date"
              value={dateStart}
              onChange={(e) => setDateStart(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
              required
            />
          </div>
          <div>
            <label
              htmlFor="absence-end"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Bis
            </label>
            <input
              id="absence-end"
              type="date"
              value={dateEnd}
              min={dateStart}
              onChange={(e) => setDateEnd(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
              required
            />
          </div>
        </div>

        {/* Half day toggle */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            role="switch"
            aria-checked={halfDay}
            onClick={() => setHalfDay(!halfDay)}
            className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full transition-colors ${halfDay ? "bg-gray-900" : "bg-gray-200"}`}
          >
            <span
              className={`pointer-events-none inline-block h-4 w-4 translate-y-0.5 rounded-full bg-white transition-transform ${halfDay ? "translate-x-4.5" : "translate-x-0.5"}`}
            />
          </button>
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
          <textarea
            id="absence-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={2}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
            placeholder="z.B. Arzttermin, Schulung ..."
          />
        </div>
      </div>
    </Modal>
  );
}

// ─── Main Content ─────────────────────────────────────────────────────────────

function TimeTrackingContent() {
  const { status: authStatus } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const toast = useToast();
  const [weekOffset, setWeekOffset] = useState(0);
  const [editModal, setEditModal] = useState<{
    date: Date;
    session: WorkSessionHistory | null;
    absence: StaffAbsence | null;
  } | null>(null);
  const [currentBreaks, setCurrentBreaks] = useState<WorkSessionBreak[]>([]);
  const [expandedSessionId, setExpandedSessionId] = useState<string | null>(
    null,
  );
  const [expandedEdits, setExpandedEdits] = useState<WorkSessionEdit[]>([]);
  const [editsLoading, setEditsLoading] = useState(false);
  const [absenceModalOpen, setAbsenceModalOpen] = useState(false);
  const [pendingCheckIn, setPendingCheckIn] = useState<
    "present" | "home_office" | null
  >(null);

  // Calculate date range: current week + previous week (for chart and table)
  const { toDate, chartFromDate, weekFromDate } = (() => {
    const ref = new Date();
    ref.setDate(ref.getDate() + weekOffset * 7);
    const days = getWeekDays(ref);
    // Chart needs 2 weeks: go back 1 extra week
    const prevRef = new Date(ref);
    prevRef.setDate(prevRef.getDate() - 7);
    const prevDays = getWeekDays(prevRef);
    return {
      toDate: days[6] ? toISODate(days[6]) : "",
      chartFromDate: prevDays[0] ? toISODate(prevDays[0]) : "",
      weekFromDate: days[0] ? toISODate(days[0]) : "",
    };
  })();

  // Fetch current session
  const { data: currentSession, mutate: mutateCurrentSession } =
    useSWRAuth<WorkSession | null>(
      "time-tracking-current",
      () => timeTrackingService.getCurrentSession(),
      { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
    );

  // Fetch history covering 2 weeks (chart needs prev + current week)
  const {
    data: historyData,
    isLoading: historyLoading,
    mutate: mutateHistory,
  } = useSWRAuth<WorkSessionHistory[]>(
    chartFromDate && toDate
      ? `time-tracking-history-${chartFromDate}-${toDate}`
      : null,
    () => timeTrackingService.getHistory(chartFromDate, toDate),
    { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
  );

  const history = historyData ?? [];

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
  const todayISO = toISODate(new Date());
  const todayAbsence = absences.find(
    (a) => a.dateStart <= todayISO && a.dateEnd >= todayISO,
  );

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
    } catch {
      // Silently handle — breaks will show empty
    }
  }, [currentSession?.id, currentSession?.checkOutTime]);

  useEffect(() => {
    fetchBreaks().catch(() => {
      // Error already handled in fetchBreaks
    });
  }, [fetchBreaks]);

  // Calculate weekly minutes from history (current week only, completed sessions)
  const weeklyCompletedMinutes = history
    .filter((s) => s.date >= weekFromDate && s.date <= toDate)
    .reduce((sum, s) => {
      if (s.checkOutTime) return sum + s.netMinutes;
      return sum;
    }, 0);

  const executeCheckIn = useCallback(
    async (status: "present" | "home_office") => {
      try {
        await timeTrackingService.checkIn(status);
        await Promise.all([mutateCurrentSession(), mutateHistory()]);
        toast.success("Erfolgreich eingestempelt");
      } catch (err) {
        toast.error(friendlyError(err, "Fehler beim Einstempeln"));
      }
    },
    [mutateCurrentSession, mutateHistory, toast],
  );

  const handleCheckIn = useCallback(
    async (status: "present" | "home_office") => {
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
      await Promise.all([mutateCurrentSession(), mutateHistory()]);
      toast.success("Erfolgreich ausgestempelt");
    } catch (err) {
      toast.error(friendlyError(err, "Fehler beim Ausstempeln"));
    }
  }, [mutateCurrentSession, mutateHistory, toast]);

  const handleStartBreak = useCallback(async () => {
    try {
      await timeTrackingService.startBreak();
      await fetchBreaks();
    } catch (err) {
      toast.error(friendlyError(err, "Fehler beim Starten der Pause"));
    }
  }, [fetchBreaks, toast]);

  const handleEndBreak = useCallback(async () => {
    try {
      await timeTrackingService.endBreak();
      await Promise.all([mutateCurrentSession(), fetchBreaks()]);
    } catch (err) {
      toast.error(friendlyError(err, "Fehler beim Beenden der Pause"));
    }
  }, [mutateCurrentSession, fetchBreaks, toast]);

  const handleEditSave = useCallback(
    async (
      id: string,
      updates: {
        checkInTime?: string;
        checkOutTime?: string;
        breakMinutes?: number;
        status?: "present" | "home_office";
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
        await Promise.all([mutateCurrentSession(), mutateHistory()]);
        // Refresh accordion edits if this session is currently expanded
        if (expandedSessionId === id) {
          try {
            const edits = await timeTrackingService.getSessionEdits(id);
            setExpandedEdits(edits);
          } catch {
            // keep stale edits on refresh failure
          }
        } else {
          // Auto-expand to show the new edit
          setExpandedSessionId(id);
          setEditsLoading(true);
          try {
            const edits = await timeTrackingService.getSessionEdits(id);
            setExpandedEdits(edits);
          } catch {
            setExpandedEdits([]);
          } finally {
            setEditsLoading(false);
          }
        }
        toast.success("Eintrag gespeichert");
      } catch (err) {
        toast.error(friendlyError(err, "Fehler beim Speichern"));
      }
    },
    [expandedSessionId, mutateCurrentSession, mutateHistory, toast],
  );

  const handleToggleExpand = useCallback(
    async (sessionId: string) => {
      if (expandedSessionId === sessionId) {
        setExpandedSessionId(null);
        setExpandedEdits([]);
        return;
      }
      setExpandedSessionId(sessionId);
      setExpandedEdits([]);
      setEditsLoading(true);
      try {
        const edits = await timeTrackingService.getSessionEdits(sessionId);
        setExpandedEdits(edits);
      } catch {
        setExpandedEdits([]);
      } finally {
        setEditsLoading(false);
      }
    },
    [expandedSessionId],
  );

  const handleCreateAbsence = useCallback(
    async (req: {
      absence_type: string;
      date_start: string;
      date_end: string;
      half_day?: boolean;
      note?: string;
    }) => {
      try {
        await timeTrackingService.createAbsence({
          absence_type: req.absence_type,
          date_start: req.date_start,
          date_end: req.date_end,
          half_day: req.half_day,
          note: req.note,
        });
        await mutateAbsences();
        toast.success("Abwesenheit eingetragen");
        setAbsenceModalOpen(false);
      } catch (err) {
        toast.error(
          friendlyError(err, "Fehler beim Eintragen der Abwesenheit"),
        );
      }
    },
    [mutateAbsences, toast],
  );

  const handleDeleteAbsence = useCallback(
    async (id: string) => {
      try {
        await timeTrackingService.deleteAbsence(id);
        await mutateAbsences();
        toast.success("Abwesenheit gelöscht");
      } catch (err) {
        toast.error(friendlyError(err, "Fehler beim Löschen der Abwesenheit"));
      }
    },
    [mutateAbsences, toast],
  );

  const handleUpdateAbsence = useCallback(
    async (
      id: string,
      req: {
        absence_type?: string;
        date_start?: string;
        date_end?: string;
        half_day?: boolean;
        note?: string;
      },
    ) => {
      try {
        await timeTrackingService.updateAbsence(id, req);
        await mutateAbsences();
        toast.success("Abwesenheit aktualisiert");
      } catch (err) {
        toast.error(
          friendlyError(err, "Fehler beim Aktualisieren der Abwesenheit"),
        );
      }
    },
    [mutateAbsences, toast],
  );

  if (authStatus === "loading") {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Page header - only visible on mobile where breadcrumbs are hidden */}
      <h1 className="mb-6 text-2xl font-bold text-gray-900 md:hidden">
        Zeiterfassung
      </h1>

      {/* Clock-in card + Week chart */}
      <div className="mb-4 grid grid-cols-1 gap-4 md:mb-6 md:grid-cols-2 md:gap-6">
        <ClockInCard
          currentSession={currentSession ?? null}
          breaks={currentBreaks}
          onCheckIn={handleCheckIn}
          onCheckOut={handleCheckOut}
          onStartBreak={handleStartBreak}
          onEndBreak={handleEndBreak}
          weeklyMinutes={weeklyCompletedMinutes}
          onAddAbsence={() => setAbsenceModalOpen(true)}
        />
        <WeekChart
          history={history}
          currentSession={currentSession ?? null}
          weekOffset={weekOffset}
        />
      </div>

      {/* Week table */}
      <WeekTable
        weekOffset={weekOffset}
        onWeekChange={setWeekOffset}
        history={history}
        absences={absences}
        isLoading={historyLoading}
        onEditDay={(date, session, absence) =>
          setEditModal({ date, session, absence })
        }
        currentSession={currentSession ?? null}
        currentBreaks={currentBreaks}
        expandedSessionId={expandedSessionId}
        onToggleExpand={handleToggleExpand}
        expandedEdits={expandedEdits}
        editsLoading={editsLoading}
      />

      {/* Edit modal */}
      <EditSessionModal
        isOpen={editModal !== null}
        onClose={() => setEditModal(null)}
        session={editModal?.session ?? null}
        date={editModal?.date ?? null}
        onSave={handleEditSave}
        absence={editModal?.absence ?? null}
        onUpdateAbsence={handleUpdateAbsence}
        onDeleteAbsence={handleDeleteAbsence}
      />

      {/* Create absence modal */}
      <CreateAbsenceModal
        isOpen={absenceModalOpen}
        onClose={() => setAbsenceModalOpen(false)}
        onSave={handleCreateAbsence}
      />

      {/* Check-in confirmation when absence exists */}
      <Modal
        isOpen={pendingCheckIn !== null}
        onClose={() => setPendingCheckIn(null)}
        title=""
        footer={
          <div className="flex w-full gap-3">
            <button
              onClick={() => setPendingCheckIn(null)}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              onClick={async () => {
                const status = pendingCheckIn;
                setPendingCheckIn(null);
                if (status) await executeCheckIn(status);
              }}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700"
            >
              Trotzdem einstempeln
            </button>
          </div>
        }
      >
        <div className="py-4 text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-amber-100">
            <svg
              className="h-6 w-6 text-amber-600"
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
          <h2 className="text-xl font-bold text-gray-800">
            Abwesenheit eingetragen
          </h2>
          <p className="mt-2 text-gray-600">
            Für heute ist eine Abwesenheit eingetragen
            {todayAbsence
              ? ` (${absenceTypeLabels[todayAbsence.absenceType]})`
              : ""}
            . Trotzdem einstempeln?
          </p>
        </div>
      </Modal>
    </div>
  );
}

// ─── Page Export ───────────────────────────────────────────────────────────────

export default function TimeTrackingPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <TimeTrackingContent />
    </Suspense>
  );
}
