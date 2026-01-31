"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { Loading } from "~/components/ui/loading";
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
  isLoading,
  onCheckIn,
  onCheckOut,
  onBreakUpdate,
  weeklyMinutes,
}: {
  readonly currentSession: WorkSession | null;
  readonly isLoading: boolean;
  readonly onCheckIn: (status: "present" | "home_office") => Promise<void>;
  readonly onCheckOut: () => Promise<void>;
  readonly onBreakUpdate: (minutes: number) => Promise<void>;
  readonly weeklyMinutes: number;
}) {
  const [mode, setMode] = useState<"present" | "home_office">("present");
  const [actionLoading, setActionLoading] = useState(false);
  const [elapsedMinutes, setElapsedMinutes] = useState(0);
  const [currentTime, setCurrentTime] = useState(() => new Date());
  const [breakInput, setBreakInput] = useState("");

  const isCheckedIn =
    currentSession !== null && currentSession.checkOutTime === null;
  const isCheckedOut =
    currentSession !== null && currentSession.checkOutTime !== null;

  // Sync break input when session changes
  useEffect(() => {
    if (currentSession) {
      setBreakInput(currentSession.breakMinutes.toString());
    }
  }, [currentSession]);

  // Live timer: update elapsed time every 60s
  useEffect(() => {
    if (!isCheckedIn || !currentSession) return;

    const updateElapsed = () => {
      const checkIn = new Date(currentSession.checkInTime);
      const now = new Date();
      const totalMin = Math.floor((now.getTime() - checkIn.getTime()) / 60000);
      setElapsedMinutes(Math.max(0, totalMin - currentSession.breakMinutes));
      setCurrentTime(now);
    };

    updateElapsed();
    const interval = setInterval(updateElapsed, 60000);
    return () => clearInterval(interval);
  }, [isCheckedIn, currentSession]);

  // Update current time every 60s for the button display
  useEffect(() => {
    if (isCheckedIn) return; // Already handled above
    const interval = setInterval(() => setCurrentTime(new Date()), 60000);
    return () => clearInterval(interval);
  }, [isCheckedIn]);

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

  const handleBreakBlur = async () => {
    const minutes = parseInt(breakInput, 10);
    if (
      Number.isNaN(minutes) ||
      minutes < 0 ||
      minutes === currentSession?.breakMinutes
    )
      return;
    await onBreakUpdate(minutes);
  };

  const timeStr = `${currentTime.getHours().toString().padStart(2, "0")}:${currentTime.getMinutes().toString().padStart(2, "0")}`;

  // Break compliance warning
  const breakWarning = (() => {
    if (!isCheckedIn || !currentSession) return null;
    const breakMins = parseInt(breakInput, 10) || 0;
    if (elapsedMinutes > 540 && breakMins < 45) {
      return "45 Min Pause ab 9h nötig (§4 ArbZG)";
    }
    if (elapsedMinutes > 360 && breakMins < 30) {
      return "30 Min Pause ab 6h nötig (§4 ArbZG)";
    }
    return null;
  })();

  if (isLoading) {
    return (
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-8 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <div className="animate-pulse space-y-4">
          <div className="h-6 w-48 rounded bg-gray-200" />
          <div className="h-16 w-full rounded-2xl bg-gray-200" />
          <div className="h-4 w-32 rounded bg-gray-200" />
        </div>
      </div>
    );
  }

  return (
    <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <div className="relative p-6 sm:p-8">
        {/* Status line */}
        <div className="mb-6 flex items-center gap-3">
          <div
            className={`h-3 w-3 rounded-full ${
              isCheckedIn
                ? currentSession?.status === "home_office"
                  ? "bg-amber-500"
                  : "bg-green-500"
                : "bg-gray-400"
            }`}
          />
          <span className="text-sm font-medium text-gray-600">
            {isCheckedIn
              ? `Eingestempelt seit ${formatTime(currentSession?.checkInTime)}`
              : isCheckedOut
                ? `Ausgestempelt um ${formatTime(currentSession?.checkOutTime)}`
                : "Nicht eingestempelt"}
          </span>
          {isCheckedIn && currentSession && (
            <span
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                currentSession.status === "home_office"
                  ? "bg-amber-100 text-amber-700"
                  : "bg-green-100 text-green-700"
              }`}
            >
              {currentSession.status === "home_office"
                ? "Homeoffice"
                : "Anwesend"}
            </span>
          )}
        </div>

        {/* Mode selection (only before check-in) */}
        {!isCheckedIn && !isCheckedOut && (
          <div className="mb-6 flex gap-3">
            <button
              onClick={() => setMode("present")}
              className={`flex-1 rounded-xl px-4 py-3 text-sm font-medium transition-all ${
                mode === "present"
                  ? "bg-green-100 text-green-800 ring-2 ring-green-400"
                  : "bg-gray-100 text-gray-600 hover:bg-gray-200"
              }`}
            >
              Anwesend
            </button>
            <button
              onClick={() => setMode("home_office")}
              className={`flex-1 rounded-xl px-4 py-3 text-sm font-medium transition-all ${
                mode === "home_office"
                  ? "bg-amber-100 text-amber-800 ring-2 ring-amber-400"
                  : "bg-gray-100 text-gray-600 hover:bg-gray-200"
              }`}
            >
              Homeoffice
            </button>
          </div>
        )}

        {/* Main action button */}
        {!isCheckedOut && (
          <button
            onClick={isCheckedIn ? handleCheckOut : handleCheckIn}
            disabled={actionLoading}
            className={`w-full rounded-2xl px-6 py-5 text-lg font-bold text-white transition-all active:scale-[0.98] disabled:opacity-60 ${
              isCheckedIn
                ? "bg-gradient-to-r from-red-500 to-red-600 hover:from-red-600 hover:to-red-700"
                : mode === "home_office"
                  ? "bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-600 hover:to-amber-700"
                  : "bg-gradient-to-r from-green-500 to-green-600 hover:from-green-600 hover:to-green-700"
            }`}
          >
            <div className="flex flex-col items-center gap-1">
              <span>
                {actionLoading
                  ? "..."
                  : isCheckedIn
                    ? "AUSSTEMPELN"
                    : "EINSTEMPELN"}
              </span>
              <span className="text-sm font-normal opacity-80">
                {isCheckedIn ? formatDuration(elapsedMinutes) : timeStr}
              </span>
            </div>
          </button>
        )}

        {/* Checked out summary */}
        {isCheckedOut && currentSession && (
          <div className="rounded-2xl bg-gray-50 p-5">
            <div className="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
              <div>
                <span className="text-gray-500">Start</span>
                <p className="font-medium text-gray-900">
                  {formatTime(currentSession.checkInTime)}
                </p>
              </div>
              <div>
                <span className="text-gray-500">Ende</span>
                <p className="font-medium text-gray-900">
                  {formatTime(currentSession.checkOutTime)}
                </p>
              </div>
              <div>
                <span className="text-gray-500">Pause</span>
                <p className="font-medium text-gray-900">
                  {currentSession.breakMinutes} Min
                </p>
              </div>
              <div>
                <span className="text-gray-500">Netto</span>
                <p className="font-medium text-gray-900">
                  {formatDuration(
                    calculateNetMinutes(
                      currentSession.checkInTime,
                      currentSession.checkOutTime,
                      currentSession.breakMinutes,
                    ),
                  )}
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Break input (only when checked in) */}
        {isCheckedIn && (
          <div className="mt-5">
            <div className="flex items-center gap-3">
              <label
                htmlFor="break-minutes"
                className="text-sm font-medium text-gray-600"
              >
                Pause:
              </label>
              <input
                id="break-minutes"
                type="number"
                min={0}
                max={480}
                value={breakInput}
                onChange={(e) => setBreakInput(e.target.value)}
                onBlur={handleBreakBlur}
                className="w-20 rounded-lg border border-gray-200 px-3 py-1.5 text-center text-sm focus:border-blue-400 focus:ring-1 focus:ring-blue-400 focus:outline-none"
              />
              <span className="text-sm text-gray-500">Min</span>
            </div>
            {breakWarning && (
              <p className="mt-2 text-xs font-medium text-amber-600">
                ⚠ {breakWarning}
              </p>
            )}
          </div>
        )}

        {/* Today/Week summary */}
        <div className="mt-5 flex flex-wrap gap-x-6 gap-y-1 text-sm text-gray-500">
          <span>
            Heute:{" "}
            <span className="font-medium text-gray-700">
              {isCheckedIn
                ? `${formatDuration(elapsedMinutes)} (läuft)`
                : isCheckedOut && currentSession
                  ? formatDuration(
                      calculateNetMinutes(
                        currentSession.checkInTime,
                        currentSession.checkOutTime,
                        currentSession.breakMinutes,
                      ),
                    )
                  : "--"}
            </span>
          </span>
          <span>
            Diese Woche:{" "}
            <span className="font-medium text-gray-700">
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
  const {
    data: currentSession,
    isLoading: currentLoading,
    mutate: mutateCurrentSession,
  } = useSWRAuth<WorkSession | null>(
    "time-tracking-current",
    () => timeTrackingService.getCurrentSession(),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Fetch week history
  const {
    data: historyData,
    isLoading: historyLoading,
    mutate: mutateHistory,
  } = useSWRAuth<WorkSessionHistory[]>(
    fromDate && toDate ? `time-tracking-history-${fromDate}-${toDate}` : null,
    () => timeTrackingService.getHistory(fromDate, toDate),
    { keepPreviousData: true },
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
        const message =
          err instanceof Error ? err.message : "Fehler beim Einstempeln";
        toast.error(message);
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
      const message =
        err instanceof Error ? err.message : "Fehler beim Ausstempeln";
      toast.error(message);
    }
  }, [mutateCurrentSession, mutateHistory, toast]);

  const handleBreakUpdate = useCallback(
    async (minutes: number) => {
      try {
        await timeTrackingService.updateBreak(minutes);
        await mutateCurrentSession();
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : "Fehler beim Aktualisieren der Pause";
        toast.error(message);
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
        const message =
          err instanceof Error ? err.message : "Fehler beim Speichern";
        toast.error(message);
      }
    },
    [mutateCurrentSession, mutateHistory, toast],
  );

  if (authStatus === "loading") {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Page header */}
      <div className="mb-6 flex items-center gap-4">
        <h1 className="text-2xl font-bold text-gray-900">Zeiterfassung</h1>
      </div>

      {/* Clock-in card */}
      <div className="mb-6">
        <ClockInCard
          currentSession={currentSession ?? null}
          isLoading={currentLoading}
          onCheckIn={handleCheckIn}
          onCheckOut={handleCheckOut}
          onBreakUpdate={handleBreakUpdate}
          weeklyMinutes={weeklyCompletedMinutes}
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
