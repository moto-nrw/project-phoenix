"use client";

import { useState, useEffect, useCallback, useMemo, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
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
import type {
  WorkSession,
  WorkSessionBreak,
  WorkSessionHistory,
} from "~/lib/time-tracking-helpers";
import {
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

// Maps backend error messages to user-friendly German messages
function friendlyError(err: unknown, fallback: string): string {
  const msg =
    err instanceof Error ? err.message : typeof err === "string" ? err : "";

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
  };

  return map[code] ?? fallback;
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

function ClockInCard({
  currentSession,
  breaks,
  onCheckIn,
  onCheckOut,
  onStartBreak,
  onEndBreak,
  weeklyMinutes,
}: {
  readonly currentSession: WorkSession | null;
  readonly breaks: WorkSessionBreak[];
  readonly onCheckIn: (status: "present" | "home_office") => Promise<void>;
  readonly onCheckOut: () => Promise<void>;
  readonly onStartBreak: () => Promise<void>;
  readonly onEndBreak: () => Promise<void>;
  readonly weeklyMinutes: number;
}) {
  const [mode, setMode] = useState<"present" | "home_office">("present");
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

  // Use `tick` to make `now` reactive
  const now = new Date();
  void tick; // suppress unused warning — tick triggers re-render

  const breakMins = currentSession?.breakMinutes ?? 0;

  // Total break time including live active break
  const liveBreakMins = (() => {
    if (!activeBreak) return breakMins;
    const activeBreakMins = Math.max(
      0,
      Math.floor(
        (now.getTime() - new Date(activeBreak.startedAt).getTime()) / 60000,
      ),
    );
    return breakMins + activeBreakMins;
  })();

  // Gross elapsed since check-in
  const grossMinutes =
    isCheckedIn && currentSession
      ? Math.max(
          0,
          Math.floor(
            (now.getTime() - new Date(currentSession.checkInTime).getTime()) /
              60000,
          ),
        )
      : 0;
  const netMinutes = Math.max(0, grossMinutes - liveBreakMins);

  // Timer display: net working time (freeze during break by subtracting active break time)
  const displayMinutes = isCheckedIn ? netMinutes : 0;

  // Active break: elapsed seconds for countdown precision
  const activeBreakElapsedSecs = activeBreak
    ? Math.max(
        0,
        Math.floor(
          (now.getTime() - new Date(activeBreak.startedAt).getTime()) / 1000,
        ),
      )
    : 0;

  // Countdown remaining in seconds (null if no planned duration)
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

  // Break compliance warning
  const breakWarning = (() => {
    if (!isCheckedIn || !currentSession) return null;
    if (netMinutes > 540 && liveBreakMins < 45)
      return "45 Min Pause ab 9h nötig (§4 ArbZG)";
    if (netMinutes > 360 && liveBreakMins < 30)
      return "30 Min Pause ab 6h nötig (§4 ArbZG)";
    return null;
  })();

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

  // Format countdown as MM:SS
  const formatCountdown = (totalSecs: number): string => {
    const m = Math.floor(totalSecs / 60);
    const s = totalSecs % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  };

  return (
    <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="relative p-6 sm:p-8">
        {/* Title + status badge */}
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-lg font-bold text-gray-900">Stempeluhr</h2>
          {isCheckedIn && currentSession && (
            <span
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                isOnBreak
                  ? "bg-amber-100 text-amber-700"
                  : currentSession.status === "home_office"
                    ? "bg-amber-100 text-amber-700"
                    : "bg-[#83CD2D]/10 text-[#70b525]"
              }`}
            >
              {isOnBreak
                ? "Pause"
                : currentSession.status === "home_office"
                  ? "Homeoffice"
                  : "In der OGS"}
            </span>
          )}
        </div>

        {/* ── Not checked in: start controls ── */}
        {!isCheckedIn && !isCheckedOut && (
          <div className="flex flex-col items-center gap-5">
            {/* Mode toggle */}
            <div className="flex gap-2">
              <button
                onClick={() => setMode("present")}
                className={`rounded-full px-4 py-1.5 text-xs font-medium transition-all ${
                  mode === "present"
                    ? "bg-[#83CD2D]/10 text-[#70b525] ring-1 ring-[#83CD2D]/40"
                    : "bg-gray-100 text-gray-500 hover:bg-gray-200"
                }`}
              >
                In der OGS
              </button>
              <button
                onClick={() => setMode("home_office")}
                className={`rounded-full px-4 py-1.5 text-xs font-medium transition-all ${
                  mode === "home_office"
                    ? "bg-amber-100 text-amber-800 ring-1 ring-amber-300"
                    : "bg-gray-100 text-gray-500 hover:bg-gray-200"
                }`}
              >
                Homeoffice
              </button>
            </div>

            {/* Play button */}
            <button
              onClick={handleCheckIn}
              disabled={actionLoading}
              className={`flex h-16 w-16 items-center justify-center rounded-full border-2 transition-all active:scale-95 disabled:opacity-50 ${
                mode === "home_office"
                  ? "border-amber-500 text-amber-500 hover:bg-amber-50"
                  : "border-[#83CD2D] text-[#83CD2D] hover:bg-[#83CD2D]/5"
              }`}
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

            <span className="text-sm text-gray-400">Einstempeln</span>
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
                    className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-amber-400 text-amber-500 transition-all active:scale-95 disabled:opacity-60"
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
                    className={`flex h-12 w-12 items-center justify-center rounded-full border-2 transition-all active:scale-95 disabled:opacity-60 ${
                      breakMins > 0
                        ? "border-amber-400 text-amber-500 hover:bg-amber-50"
                        : "border-gray-200 text-gray-400 hover:border-gray-300 hover:text-gray-500"
                    }`}
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
                    <div
                      className="fixed inset-0 z-10"
                      onClick={() => setBreakMenuOpen(false)}
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
                {isOnBreak && countdownRemainingSecs !== null ? (
                  <>
                    <span className="text-4xl font-light text-amber-500 tabular-nums">
                      {formatCountdown(countdownRemainingSecs)}
                    </span>
                    <span className="mt-0.5 text-xs font-medium text-amber-500">
                      Pause ({plannedBreakMinutes} Min)
                    </span>
                  </>
                ) : isOnBreak ? (
                  <>
                    <span className="text-4xl font-light text-amber-500 tabular-nums">
                      {formatCountdown(activeBreakElapsedSecs)}
                    </span>
                    <span className="mt-0.5 text-xs font-medium text-amber-500">
                      Pause läuft
                    </span>
                  </>
                ) : (
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
                )}
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
                {checkedOutNet !== null ? formatDuration(checkedOutNet) : "--"}
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
        <div className="mt-4 flex flex-wrap gap-x-6 gap-y-1 text-xs text-gray-400">
          <span>
            Heute:{" "}
            <span className="font-medium text-gray-600">
              {isCheckedIn
                ? formatDuration(netMinutes)
                : isCheckedOut && checkedOutNet !== null
                  ? formatDuration(checkedOutNet)
                  : "--"}
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
    const breakDuration = breakEnd
      ? brk.durationMinutes
      : isActiveBreak && plannedBreakMinutes
        ? plannedBreakMinutes
        : Math.max(
            0,
            Math.floor((now.getTime() - breakStart.getTime()) / 60000),
          );
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
  const lastBreak = breaks[breaks.length - 1];
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

  const renderRow = (
    seg: (typeof segments)[number],
    index: number,
    showBorder: boolean,
  ) => (
    <div
      key={`${seg.type}-${index}`}
      className={`flex items-center justify-between py-2.5 text-sm ${
        showBorder ? "border-t border-gray-50" : ""
      } ${seg.type === "break" && seg.isActive ? "text-amber-600" : seg.type === "break" ? "text-gray-500" : ""}`}
    >
      <span
        className={`w-14 shrink-0 font-medium ${
          seg.type === "work" && seg.isActive
            ? "text-[#83CD2D]"
            : seg.type === "work"
              ? "text-gray-600"
              : ""
        }`}
      >
        {seg.type === "work" ? "Arbeit" : "Pause"}
      </span>
      <span
        className={`w-12 shrink-0 text-center tabular-nums ${
          seg.type === "work" && seg.isActive
            ? "text-[#70b525]"
            : seg.type === "work"
              ? "text-gray-600"
              : ""
        }`}
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

  return (
    <div className="relative flex h-full flex-col overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="relative flex min-h-0 flex-1 flex-col p-6 sm:p-8">
        <h2 className="mb-4 text-lg font-bold text-gray-900">
          Wochenübersicht
        </h2>
        <ChartContainer config={weekChartConfig} className="min-h-0 flex-1">
          <BarChart
            accessibilityLayer
            data={chartData}
            margin={{ top: 4, right: 4, bottom: 0, left: -20 }}
          >
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="day"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              fontSize={11}
              interval={0}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={4}
              fontSize={12}
              tickFormatter={(v: number) => `${Math.round(v / 60)}h`}
              domain={[0, "auto"]}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelFormatter={(_value, payload) => {
                    const item = payload[0]?.payload as
                      | { label?: string }
                      | undefined;
                    return item?.label ?? "";
                  }}
                  formatter={(value, name) => {
                    const totalMins = value as number;
                    const hours = Math.floor(totalMins / 60);
                    const mins = totalMins % 60;
                    return (
                      <span>
                        {name === "netMinutes" ? "Arbeitszeit" : "Pause"}:{" "}
                        {hours}h {mins}min
                      </span>
                    );
                  }}
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

// ─── WeekTable ────────────────────────────────────────────────────────────────

function WeekTable({
  weekOffset,
  onWeekChange,
  history,
  isLoading,
  onEditDay,
  currentSession,
  currentBreaks,
}: {
  readonly weekOffset: number;
  readonly onWeekChange: (offset: number) => void;
  readonly history: WorkSessionHistory[];
  readonly isLoading: boolean;
  readonly onEditDay: (date: Date, session: WorkSessionHistory) => void;
  readonly currentSession: WorkSession | null;
  readonly currentBreaks: WorkSessionBreak[];
}) {
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

  // Calculate weekly total
  const weeklyNetMinutes = history.reduce((sum, s) => {
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
      <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4">
        <button
          onClick={() => onWeekChange(weekOffset - 1)}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
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
        <span className="text-lg font-bold text-gray-900">
          KW {weekNum}: {mondayDate ? formatDateGerman(mondayDate) : ""} –{" "}
          {sundayDate ? formatDateGerman(sundayDate) : ""}
        </span>
        <button
          onClick={() => onWeekChange(weekOffset + 1)}
          disabled={weekOffset >= 0}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-gray-700 disabled:opacity-30 disabled:hover:bg-transparent"
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
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-100 text-left text-xs font-medium tracking-wide text-gray-400 uppercase">
              <th className="px-6 py-3">Tag</th>
              <th className="px-4 py-3">Start</th>
              <th className="px-4 py-3">Ende</th>
              <th className="px-4 py-3">Pause</th>
              <th className="px-4 py-3">Netto</th>
              <th className="px-4 py-3 text-center">Ort</th>
            </tr>
          </thead>
          <tbody>
            {weekDays.map((day, index) => {
              if (!day) return null;
              const dateKey = toISODate(day);
              const session = sessionMap.get(dateKey);
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

              // Calculate net for active session live (with real break time)
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
                  netDisplay = live !== null ? formatDuration(live) : "--";
                }
              }

              const warnings = session ? getComplianceWarnings(session) : [];

              return (
                <tr
                  key={dateKey}
                  onClick={canEdit ? () => onEditDay(day, session) : undefined}
                  className={`border-b border-gray-50 transition-colors ${
                    isToday ? "bg-blue-50/50" : ""
                  } ${canEdit ? "cursor-pointer hover:bg-gray-50" : ""} ${
                    isFuture ? "opacity-40" : ""
                  }`}
                >
                  <td className="px-6 py-3 font-medium text-gray-700">
                    {dayName} {formatDateShort(day)}
                  </td>
                  <td className="px-4 py-3 text-gray-600">
                    {session ? formatTime(session.checkInTime) : "--:--"}
                  </td>
                  <td className="px-4 py-3 text-gray-600">
                    {session
                      ? session.checkOutTime
                        ? formatTime(session.checkOutTime)
                        : isActive
                          ? "···"
                          : "--:--"
                      : "--:--"}
                  </td>
                  <td className="px-4 py-3 text-gray-600">
                    {session && session.breakMinutes > 0
                      ? `${session.breakMinutes}`
                      : "--"}
                  </td>
                  <td className="px-4 py-3">
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
                    {session ? (
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                          isActive
                            ? "bg-green-100 text-green-700"
                            : session.status === "home_office"
                              ? "bg-amber-100 text-amber-700"
                              : "bg-gray-100 text-gray-600"
                        }`}
                      >
                        {isActive
                          ? "aktiv"
                          : session.status === "home_office"
                            ? "Homeoffice"
                            : "In der OGS"}
                      </span>
                    ) : isFuture ? null : (
                      <span className="text-gray-300">—</span>
                    )}
                  </td>
                </tr>
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
              <td className="px-4 py-3 text-sm font-bold text-gray-700">
                {isLoading ? "..." : formatDuration(weeklyNetMinutes)}
              </td>
              <td />
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  );
}

// ─── EditModal ────────────────────────────────────────────────────────────────

function EditSessionModal({
  isOpen,
  onClose,
  session,
  date,
  onSave,
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
    },
  ) => Promise<void>;
}) {
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [breakMins, setBreakMins] = useState("0");
  const [status, setStatus] = useState<"present" | "home_office">("present");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (session && isOpen) {
      setStartTime(extractTimeFromISO(session.checkInTime));
      setEndTime(
        session.checkOutTime ? extractTimeFromISO(session.checkOutTime) : "",
      );
      setBreakMins(session.breakMinutes.toString());
      setStatus(session.status);
      setNotes(session.notes ?? "");
    }
  }, [session, isOpen]);

  if (!session || !date) return null;

  const dayIndex = (date.getDay() + 6) % 7;
  const dayName = DAY_NAMES_LONG[dayIndex] ?? "";

  // Compliance check for the edited values
  const editedBreak = parseInt(breakMins, 10) || 0;
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

  const handleSave = async () => {
    if (!session) return;
    setSaving(true);
    try {
      const dateStr = toISODate(date);

      // Build ISO timestamps from time inputs + date
      const buildTimestamp = (time: string) => {
        if (!time) return undefined;
        return `${dateStr}T${time}:00`;
      };

      await onSave(session.id, {
        checkInTime: buildTimestamp(startTime),
        checkOutTime: buildTimestamp(endTime) ?? undefined,
        breakMinutes: editedBreak,
        status,
        notes: notes || undefined,
      });
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Eintrag bearbeiten — ${dayName}, ${formatDateGerman(date)}`}
      footer={
        <div className="flex justify-end gap-3">
          <button
            onClick={onClose}
            className="rounded-xl px-4 py-2 text-sm font-medium text-gray-600 hover:bg-gray-100"
          >
            Abbrechen
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !startTime}
            className="rounded-xl bg-blue-600 px-5 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? "Speichern..." : "Speichern"}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
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
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-400 focus:ring-1 focus:ring-blue-400 focus:outline-none"
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
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-400 focus:ring-1 focus:ring-blue-400 focus:outline-none"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="edit-break"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Pause (Min)
            </label>
            <input
              id="edit-break"
              type="number"
              min={0}
              max={480}
              value={breakMins}
              onChange={(e) => setBreakMins(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-400 focus:ring-1 focus:ring-blue-400 focus:outline-none"
            />
          </div>
          <div>
            <label
              htmlFor="edit-status"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Status
            </label>
            <select
              id="edit-status"
              value={status}
              onChange={(e) =>
                setStatus(e.target.value as "present" | "home_office")
              }
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-400 focus:ring-1 focus:ring-blue-400 focus:outline-none"
            >
              <option value="present">In der OGS</option>
              <option value="home_office">Homeoffice</option>
            </select>
          </div>
        </div>

        <div>
          <label
            htmlFor="edit-notes"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Notiz
          </label>
          <textarea
            id="edit-notes"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={2}
            className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-400 focus:ring-1 focus:ring-blue-400 focus:outline-none"
            placeholder="Optional..."
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
    session: WorkSessionHistory;
  } | null>(null);
  const [currentBreaks, setCurrentBreaks] = useState<WorkSessionBreak[]>([]);

  // Calculate date range: current week + previous week (for chart and table)
  const { toDate, chartFromDate } = (() => {
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
    void fetchBreaks();
  }, [fetchBreaks]);

  // Calculate weekly minutes from history (for completed sessions only)
  const weeklyCompletedMinutes = history.reduce((sum, s) => {
    if (s.checkOutTime) return sum + s.netMinutes;
    return sum;
  }, 0);

  const handleCheckIn = useCallback(
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
      },
    ) => {
      try {
        await timeTrackingService.updateSession(id, {
          checkInTime: updates.checkInTime,
          checkOutTime: updates.checkOutTime,
          breakMinutes: updates.breakMinutes,
          status: updates.status,
          notes: updates.notes,
        });
        await Promise.all([mutateCurrentSession(), mutateHistory()]);
        toast.success("Eintrag gespeichert");
      } catch (err) {
        toast.error(friendlyError(err, "Fehler beim Speichern"));
      }
    },
    [mutateCurrentSession, mutateHistory, toast],
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
      <div className="mb-6 grid grid-cols-1 gap-6 md:grid-cols-2">
        <ClockInCard
          currentSession={currentSession ?? null}
          breaks={currentBreaks}
          onCheckIn={handleCheckIn}
          onCheckOut={handleCheckOut}
          onStartBreak={handleStartBreak}
          onEndBreak={handleEndBreak}
          weeklyMinutes={weeklyCompletedMinutes}
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
        isLoading={historyLoading}
        onEditDay={(date, session) => setEditModal({ date, session })}
        currentSession={currentSession ?? null}
        currentBreaks={currentBreaks}
      />

      {/* Edit modal */}
      <EditSessionModal
        isOpen={editModal !== null}
        onClose={() => setEditModal(null)}
        session={editModal?.session ?? null}
        date={editModal?.date ?? null}
        onSave={handleEditSave}
      />
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
