"use client";

import { useState, useEffect, useCallback, useMemo, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Loading } from "~/components/ui/loading";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useSWRAuth } from "~/lib/swr";
import { timeTrackingService } from "~/lib/time-tracking-api";
import type {
  WorkSession,
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
    "already checked out today": "Du hast heute bereits gearbeitet.", // kept for backwards compat
    "no active session found": "Kein aktiver Eintrag vorhanden.",
    "no session found for today": "Kein Eintrag für heute vorhanden.",
    "session not found": "Eintrag nicht gefunden.",
    "can only update own sessions": "Du kannst nur eigene Einträge bearbeiten.",
    "break minutes cannot be negative":
      "Pausenminuten dürfen nicht negativ sein.",
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

function ClockInCard({
  currentSession,
  onCheckIn,
  onCheckOut,
  onBreakUpdate,
  weeklyMinutes,
}: {
  readonly currentSession: WorkSession | null;
  readonly onCheckIn: (status: "present" | "home_office") => Promise<void>;
  readonly onCheckOut: () => Promise<void>;
  readonly onBreakUpdate: (minutes: number) => Promise<void>;
  readonly weeklyMinutes: number;
}) {
  const [mode, setMode] = useState<"present" | "home_office">("present");
  const [actionLoading, setActionLoading] = useState(false);
  const [elapsedMinutes, setElapsedMinutes] = useState(0);
  const [showBreakInput, setShowBreakInput] = useState(false);

  const isCheckedIn =
    currentSession !== null && currentSession.checkOutTime === null;
  const isCheckedOut =
    currentSession !== null && currentSession.checkOutTime !== null;

  // Live timer: update elapsed time every 60s
  useEffect(() => {
    if (!isCheckedIn || !currentSession) return;

    const updateElapsed = () => {
      const checkIn = new Date(currentSession.checkInTime);
      const now = new Date();
      const totalMin = Math.floor((now.getTime() - checkIn.getTime()) / 60000);
      setElapsedMinutes(Math.max(0, totalMin - currentSession.breakMinutes));
    };

    updateElapsed();
    const interval = setInterval(updateElapsed, 60000);
    return () => clearInterval(interval);
  }, [isCheckedIn, currentSession]);

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

  // Format elapsed as H:MM
  const timerDisplay = (() => {
    const h = Math.floor(elapsedMinutes / 60);
    const m = elapsedMinutes % 60;
    return `${h}:${m.toString().padStart(2, "0")}`;
  })();

  // Break compliance warning
  const breakWarning = (() => {
    if (!isCheckedIn || !currentSession) return null;
    const breakMins = currentSession?.breakMinutes ?? 0;
    if (elapsedMinutes > 540 && breakMins < 45) {
      return "45 Min Pause ab 9h nötig (§4 ArbZG)";
    }
    if (elapsedMinutes > 360 && breakMins < 30) {
      return "30 Min Pause ab 6h nötig (§4 ArbZG)";
    }
    return null;
  })();

  // Computed net minutes for checked-out state
  const checkedOutNet =
    isCheckedOut && currentSession
      ? calculateNetMinutes(
          currentSession.checkInTime,
          currentSession.checkOutTime,
          currentSession.breakMinutes,
        )
      : null;

  return (
    <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="relative p-6 sm:p-8">
        {/* Title + status badge */}
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-lg font-bold text-gray-900">Zeiterfassung</h2>
          {isCheckedIn && currentSession && (
            <span
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                currentSession.status === "home_office"
                  ? "bg-amber-100 text-amber-700"
                  : "bg-[#83CD2D]/10 text-[#70b525]"
              }`}
            >
              {currentSession.status === "home_office"
                ? "Homeoffice"
                : "Anwesend"}
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
                Anwesend
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
              {/* Pause button */}
              <div className="relative">
                <button
                  onClick={() => setShowBreakInput((v) => !v)}
                  className={`flex h-12 w-12 items-center justify-center rounded-full border-2 transition-all active:scale-95 ${
                    currentSession && currentSession.breakMinutes > 0
                      ? "border-amber-400 text-amber-500 hover:bg-amber-50"
                      : "border-gray-200 text-gray-400 hover:border-gray-300 hover:text-gray-500"
                  }`}
                  aria-label="Pause bearbeiten"
                  title={`Pause: ${currentSession?.breakMinutes ?? 0} Min`}
                >
                  <svg
                    className="h-5 w-5"
                    fill="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
                  </svg>
                </button>

                {/* Break quick-select popover */}
                {showBreakInput && (
                  <div className="absolute top-14 left-0 z-10 w-44 rounded-xl border border-gray-200 bg-white p-2 shadow-lg">
                    <div className="grid grid-cols-2 gap-1.5">
                      {[15, 30, 45, 60].map((mins) => (
                        <button
                          key={mins}
                          onClick={() => {
                            void onBreakUpdate(mins);
                            setShowBreakInput(false);
                          }}
                          className={`rounded-lg px-3 py-2 text-center text-sm font-medium whitespace-nowrap transition-all ${
                            currentSession?.breakMinutes === mins
                              ? "bg-amber-100 text-amber-700"
                              : "bg-gray-50 text-gray-600 hover:bg-gray-100"
                          }`}
                        >
                          {mins} Min
                        </button>
                      ))}
                    </div>
                    {breakWarning && (
                      <p className="mt-1.5 px-1 text-xs font-medium text-amber-600">
                        {breakWarning}
                      </p>
                    )}
                  </div>
                )}
              </div>

              {/* Timer display */}
              <span className="text-4xl font-light text-gray-900 tabular-nums">
                {timerDisplay}
              </span>

              {/* Stop / check-out button */}
              <button
                onClick={handleCheckOut}
                disabled={actionLoading}
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

            {/* Session detail rows */}
            <div className="border-t border-gray-100">
              {/* Work row (always visible) */}
              <div className="flex items-center justify-between py-3 text-sm">
                <span className="w-16 font-medium text-[#83CD2D]">Arbeit</span>
                <span className="text-[#70b525] tabular-nums">
                  {formatTime(currentSession?.checkInTime)}
                </span>
                <span className="text-gray-300">&rarr;</span>
                <span className="text-gray-400 tabular-nums">···</span>
                <span className="w-20 text-right font-medium text-gray-700 tabular-nums">
                  {formatDuration(elapsedMinutes)}
                </span>
              </div>

              {/* Break row (only when break > 0) */}
              {currentSession && currentSession.breakMinutes > 0 && (
                <div className="flex items-center justify-between border-t border-gray-50 py-3 text-sm">
                  <span className="w-16 font-medium text-gray-500">Pause</span>
                  <span className="text-gray-500 tabular-nums" />
                  <span />
                  <span />
                  <span className="w-20 text-right text-gray-500 tabular-nums">
                    {formatDuration(currentSession.breakMinutes)}
                  </span>
                </div>
              )}
            </div>
          </>
        )}

        {/* ── Checked out: summary rows ── */}
        {isCheckedOut && currentSession && (
          <div className="space-y-0 border-t border-gray-100">
            {/* Work row */}
            <div className="flex items-center justify-between border-b border-gray-50 py-3 text-sm">
              <span className="w-16 font-medium text-gray-700">Arbeit</span>
              <span className="text-gray-600 tabular-nums">
                {formatTime(currentSession.checkInTime)}
              </span>
              <span className="text-gray-300">&rarr;</span>
              <span className="text-gray-600 tabular-nums">
                {formatTime(currentSession.checkOutTime)}
              </span>
              <span className="w-20 text-right font-medium text-gray-700 tabular-nums">
                {checkedOutNet !== null ? formatDuration(checkedOutNet) : "--"}
              </span>
            </div>

            {/* Break row */}
            {currentSession.breakMinutes > 0 && (
              <div className="flex items-center justify-between py-3 text-sm">
                <span className="w-16 font-medium text-gray-500">Pause</span>
                <span className="text-gray-500 tabular-nums" />
                <span className="text-gray-300" />
                <span className="text-gray-500 tabular-nums" />
                <span className="w-20 text-right text-gray-500 tabular-nums">
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
                ? `${formatDuration(elapsedMinutes)}`
                : isCheckedOut && checkedOutNet !== null
                  ? formatDuration(checkedOutNet)
                  : "--"}
            </span>
          </span>
          <span>
            Woche:{" "}
            <span className="font-medium text-gray-600">
              {formatDuration(
                weeklyMinutes + (isCheckedIn ? elapsedMinutes : 0),
              )}
            </span>
          </span>
        </div>
      </div>
    </div>
  );
}

// ─── WeekChart ───────────────────────────────────────────────────────────────

const weekChartConfig = {
  arbeitszeit: { label: "Arbeitszeit", color: "#0ea5e9" }, // sky-500 — matches sidebar icon
  pause: { label: "Pause", color: "#94a3b8" }, // slate-400 — muted secondary
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
    const referenceDate = new Date(today);
    referenceDate.setDate(referenceDate.getDate() + weekOffset * 7);
    const weekDays = getWeekDays(referenceDate);

    const sessionMap = new Map<string, WorkSessionHistory>();
    for (const session of history) {
      sessionMap.set(session.date, session);
    }

    return weekDays.map((day, i) => {
      if (!day) return { day: DAY_NAMES[i] ?? "", arbeitszeit: 0, pause: 0 };

      const dateKey = toISODate(day);
      const session = sessionMap.get(dateKey);

      let netHours = 0;
      let breakHours = 0;

      if (session) {
        if (session.checkOutTime) {
          netHours = session.netMinutes / 60;
        } else if (
          isSameDay(day, today) &&
          currentSession &&
          !currentSession.checkOutTime
        ) {
          const elapsed = Math.floor(
            (Date.now() - new Date(session.checkInTime).getTime()) / 60000,
          );
          netHours = Math.max(0, elapsed - session.breakMinutes) / 60;
        }
        breakHours = session.breakMinutes / 60;
      }

      return {
        day: DAY_NAMES[i] ?? "",
        arbeitszeit: +netHours.toFixed(1),
        pause: +breakHours.toFixed(1),
      };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [history, currentSession, weekOffset]);

  return (
    <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="relative p-6 sm:p-8">
        <h3 className="mb-4 text-sm font-semibold text-gray-700">
          Wochenübersicht
        </h3>
        <ChartContainer config={weekChartConfig} className="h-[180px] w-full">
          <AreaChart
            data={chartData}
            margin={{ top: 4, right: 4, bottom: 0, left: -20 }}
          >
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis
              dataKey="day"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              fontSize={12}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={4}
              fontSize={12}
              tickFormatter={(v: number) => `${v}h`}
              domain={[0, "auto"]}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value, name) => {
                    const hours = Math.floor(value as number);
                    const mins = Math.round(((value as number) - hours) * 60);
                    return (
                      <span>
                        {name === "arbeitszeit" ? "Arbeitszeit" : "Pause"}:{" "}
                        {hours}h {mins}min
                      </span>
                    );
                  }}
                />
              }
            />
            <Area
              type="monotone"
              dataKey="pause"
              stroke="var(--color-pause)"
              fill="var(--color-pause)"
              fillOpacity={0.1}
              strokeWidth={1.5}
            />
            <Area
              type="monotone"
              dataKey="arbeitszeit"
              stroke="var(--color-arbeitszeit)"
              fill="var(--color-arbeitszeit)"
              fillOpacity={0.15}
              strokeWidth={2}
            />
          </AreaChart>
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
}: {
  readonly weekOffset: number;
  readonly onWeekChange: (offset: number) => void;
  readonly history: WorkSessionHistory[];
  readonly isLoading: boolean;
  readonly onEditDay: (date: Date, session: WorkSessionHistory) => void;
  readonly currentSession: WorkSession | null;
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

  // Calculate weekly total
  const weeklyNetMinutes = history.reduce((sum, s) => {
    if (s.checkOutTime) return sum + s.netMinutes;
    // For active session, calculate live
    if (currentSession && !currentSession.checkOutTime) {
      const live = calculateNetMinutes(
        s.checkInTime,
        new Date().toISOString(),
        s.breakMinutes,
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
        <span className="text-sm font-semibold text-gray-700">
          KW {weekNum}: {mondayDate ? formatDateShort(mondayDate) : ""} –{" "}
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
              <th className="px-4 py-3">Status</th>
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

              // Calculate net for active session live
              let netDisplay = "--";
              if (session) {
                if (session.checkOutTime) {
                  netDisplay = formatDuration(session.netMinutes);
                } else if (isActive) {
                  const live = calculateNetMinutes(
                    session.checkInTime,
                    new Date().toISOString(),
                    session.breakMinutes,
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
                  <td className="px-4 py-3">
                    {session ? (
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                          isActive
                            ? "bg-green-100 text-green-700"
                            : session.autoCheckedOut
                              ? "bg-orange-100 text-orange-700"
                              : session.status === "home_office"
                                ? "bg-amber-100 text-amber-700"
                                : "bg-gray-100 text-gray-600"
                        }`}
                      >
                        {isActive
                          ? "aktiv"
                          : session.autoCheckedOut
                            ? "Auto"
                            : session.status === "home_office"
                              ? "HO"
                              : "Schule"}
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
              <option value="present">Anwesend</option>
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

  // Calculate week date range for history fetch (stable across renders)
  const { fromDate, toDate } = (() => {
    const ref = new Date();
    ref.setDate(ref.getDate() + weekOffset * 7);
    const days = getWeekDays(ref);
    return {
      fromDate: days[0] ? toISODate(days[0]) : "",
      toDate: days[6] ? toISODate(days[6]) : "",
    };
  })();

  // Fetch current session
  const { data: currentSession, mutate: mutateCurrentSession } =
    useSWRAuth<WorkSession | null>(
      "time-tracking-current",
      () => timeTrackingService.getCurrentSession(),
      { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
    );

  // Fetch week history
  const {
    data: historyData,
    isLoading: historyLoading,
    mutate: mutateHistory,
  } = useSWRAuth<WorkSessionHistory[]>(
    fromDate && toDate ? `time-tracking-history-${fromDate}-${toDate}` : null,
    () => timeTrackingService.getHistory(fromDate, toDate),
    { keepPreviousData: true, revalidateOnFocus: false, errorRetryCount: 1 },
  );

  const history = historyData ?? [];

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
      await Promise.all([mutateCurrentSession(), mutateHistory()]);
      toast.success("Erfolgreich ausgestempelt");
    } catch (err) {
      toast.error(friendlyError(err, "Fehler beim Ausstempeln"));
    }
  }, [mutateCurrentSession, mutateHistory, toast]);

  const handleBreakUpdate = useCallback(
    async (minutes: number) => {
      try {
        await timeTrackingService.updateBreak(minutes);
        await mutateCurrentSession();
      } catch (err) {
        toast.error(friendlyError(err, "Fehler beim Aktualisieren der Pause"));
      }
    },
    [mutateCurrentSession, toast],
  );

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
          onCheckIn={handleCheckIn}
          onCheckOut={handleCheckOut}
          onBreakUpdate={handleBreakUpdate}
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
