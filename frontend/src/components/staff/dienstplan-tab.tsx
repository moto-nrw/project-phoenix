"use client";

import { useState, useMemo, useEffect, useRef } from "react";
import { staffScheduleService, staffHistoryService } from "~/lib/staff-api";
import type { StaffHistorySession } from "~/lib/staff-api";
import { formatDuration } from "~/lib/time-tracking-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useToast } from "~/contexts/ToastContext";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { createLogger } from "~/lib/logger";
import {
  computeStaffMetrics,
  getDeltaStatus,
  buildMonthGrid,
  formatMonthHeader,
  groupSessionsByDay,
  startOfMonth,
  endOfMonth,
  startOfYear,
  startOfWeek,
  endOfWeek,
  toDateKey,
  isSameDay,
  toIsoDayOfWeek,
} from "~/lib/staff-metrics-helpers";

const logger = createLogger({ component: "DienstplanTab" });

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
const weekdayShort = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
const WORK_DAYS: readonly number[] = [0, 1, 2, 3, 4];

// ─── KPI Cards ───────────────────────────────────────────────────────────────

function KpiCard({
  label,
  primary,
  secondary,
  progressPct,
  color,
}: {
  readonly label: string;
  readonly primary: string;
  readonly secondary?: string;
  readonly progressPct?: number;
  readonly color?: "green" | "amber" | "gray" | "red";
}) {
  const primaryColor = {
    green: "text-green-600",
    amber: "text-amber-600",
    gray: "text-gray-700",
    red: "text-red-600",
  }[color ?? "gray"];
  const barColor = {
    green: "bg-green-500",
    amber: "bg-amber-500",
    gray: "bg-gray-400",
    red: "bg-red-500",
  }[color ?? "gray"];
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <p className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
        {label}
      </p>
      <p className={`mt-2 text-2xl font-bold ${primaryColor}`}>{primary}</p>
      {secondary && <p className="mt-1 text-xs text-gray-500">{secondary}</p>}
      {progressPct !== undefined && (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-100">
          <div
            className={`h-full rounded-full ${barColor} transition-all`}
            style={{ width: `${Math.min(100, Math.max(0, progressPct))}%` }}
          />
        </div>
      )}
    </div>
  );
}

function formatSignedDuration(minutes: number): string {
  const sign = minutes > 0 ? "+" : minutes < 0 ? "−" : "";
  return `${sign}${formatDuration(Math.abs(minutes))}`;
}

function KpiCards({
  metrics,
}: {
  readonly metrics: ReturnType<typeof computeStaffMetrics>;
}) {
  const weekPct =
    metrics.weekSoll > 0 ? (metrics.weekIst / metrics.weekSoll) * 100 : 0;
  const monthPct =
    metrics.monthSoll > 0 ? (metrics.monthIst / metrics.monthSoll) * 100 : 0;

  const monthDeltaColor = getDeltaStatus(metrics.monthDelta, metrics.monthSoll);
  const ytdColor: "green" | "amber" | "gray" =
    metrics.ytdBalance > 0
      ? "amber"
      : metrics.ytdBalance < -60
        ? "gray"
        : "green";

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <KpiCard
        label="Diese Woche"
        primary={formatDuration(metrics.weekIst)}
        secondary={`von ${formatDuration(metrics.weekSoll)} Soll`}
        progressPct={weekPct}
        color={getDeltaStatus(metrics.weekDelta, metrics.weekSoll)}
      />
      <KpiCard
        label="Dieser Monat"
        primary={formatDuration(metrics.monthIst)}
        secondary={`von ${formatDuration(metrics.monthSoll)} Soll`}
        progressPct={monthPct}
        color={monthDeltaColor}
      />
      <KpiCard
        label="Überstunden Monat"
        primary={formatSignedDuration(metrics.monthDelta)}
        secondary={
          metrics.monthDelta === 0
            ? "ausgeglichen"
            : metrics.monthDelta > 0
              ? "Überstunden"
              : "Minusstunden"
        }
        color={monthDeltaColor}
      />
      <KpiCard
        label="Saldo seit 1. Januar"
        primary={formatSignedDuration(metrics.ytdBalance)}
        secondary="Arbeitszeitkonto"
        color={ytdColor}
      />
    </div>
  );
}

// ─── View Toggle ─────────────────────────────────────────────────────────────

type ViewMode = "month" | "week";

function ViewToggle({
  value,
  onChange,
}: {
  readonly value: ViewMode;
  readonly onChange: (v: ViewMode) => void;
}) {
  const buttonClass = (active: boolean) =>
    `px-3 py-1.5 text-xs font-medium transition-colors ${
      active ? "bg-gray-900 text-white" : "text-gray-500 hover:text-gray-700"
    }`;
  return (
    <div className="inline-flex items-center overflow-hidden rounded-full border border-gray-200 bg-white">
      <button
        type="button"
        onClick={() => onChange("month")}
        className={buttonClass(value === "month")}
      >
        Monat
      </button>
      <button
        type="button"
        onClick={() => onChange("week")}
        className={buttonClass(value === "week")}
      >
        Woche
      </button>
    </div>
  );
}

// ─── Month Calendar ──────────────────────────────────────────────────────────

function MonthCalendar({
  monthAnchor,
  sessionMinutesByDay,
  targetByDow,
  onPrev,
  onNext,
  onGoToday,
  today,
}: {
  readonly monthAnchor: Date;
  readonly sessionMinutesByDay: Map<string, number>;
  readonly targetByDow: Map<number, number>;
  readonly onPrev: () => void;
  readonly onNext: () => void;
  readonly onGoToday: () => void;
  readonly today: Date;
}) {
  const grid = useMemo(() => buildMonthGrid(monthAnchor), [monthAnchor]);

  return (
    <div>
      {/* Navigation header — 3-column grid: spacer | centered nav | Heute */}
      <div className="mb-4 grid grid-cols-3 items-center">
        <div />
        <div className="flex items-center justify-center gap-2">
          <button
            type="button"
            onClick={onPrev}
            aria-label="Vorheriger Monat"
            className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
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
          </button>
          <h3 className="min-w-[10rem] text-center text-sm font-semibold text-gray-800">
            {formatMonthHeader(monthAnchor)}
          </h3>
          <button
            type="button"
            onClick={onNext}
            aria-label="Nächster Monat"
            className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
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
          </button>
        </div>
        <div className="flex justify-end">
          <button
            type="button"
            onClick={onGoToday}
            className="rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50"
          >
            Heute
          </button>
        </div>
      </div>

      {/* Weekday headers */}
      <div className="mb-2 grid grid-cols-7 gap-1">
        {weekdayShort.map((d) => (
          <div
            key={d}
            className="px-2 py-1 text-xs font-semibold tracking-wider text-gray-400 uppercase"
          >
            {d}
          </div>
        ))}
      </div>

      {/* Grid */}
      <div className="grid grid-cols-7 gap-1">
        {grid.flat().map((cell, idx) => {
          const key = toDateKey(cell.date);
          const actual = sessionMinutesByDay.get(key);
          const dow = toIsoDayOfWeek(cell.date);
          const target = targetByDow.get(dow) ?? 0;
          const isToday = isSameDay(cell.date, today);
          const isFuture = cell.date > today;
          const isWeekend = dow === 5 || dow === 6;

          let pillColor = "";
          let pillText: string | null = null;
          if (actual !== undefined) {
            const delta = actual - target;
            const status = getDeltaStatus(delta, target);
            pillColor =
              status === "green"
                ? "bg-green-50 text-green-700"
                : status === "amber"
                  ? "bg-amber-50 text-amber-700"
                  : "bg-gray-50 text-gray-600";
            pillText = formatDuration(actual);
          }

          return (
            <div
              key={`${key}-${idx}`}
              className={`min-h-[4.5rem] rounded-xl border p-2 transition-colors ${
                cell.inMonth
                  ? "border-gray-100 bg-white hover:bg-gray-50"
                  : "border-transparent bg-gray-50/50"
              } ${isWeekend && cell.inMonth ? "bg-gray-50/30" : ""}`}
            >
              <div className="flex items-start justify-between">
                <span
                  className={`inline-flex h-6 w-6 items-center justify-center text-xs ${
                    isToday
                      ? "rounded-full bg-gray-900 font-semibold text-white"
                      : cell.inMonth
                        ? isFuture
                          ? "text-gray-400"
                          : "font-medium text-gray-700"
                        : "text-gray-300"
                  }`}
                >
                  {cell.date.getDate()}
                </span>
              </div>
              {pillText && cell.inMonth && (
                <div className="mt-1.5">
                  <span
                    className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium tabular-nums ${pillColor}`}
                  >
                    {pillText}
                  </span>
                </div>
              )}
              {target > 0 && !pillText && cell.inMonth && !isFuture && (
                <div className="mt-1.5">
                  <span className="text-[10px] text-gray-300 tabular-nums">
                    Soll {formatDuration(target)}
                  </span>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Week View ───────────────────────────────────────────────────────────────

function WeekView({
  weekAnchor,
  sessionMinutesByDay,
  targetByDow,
  onPrev,
  onNext,
  onGoToday,
  today,
}: {
  readonly weekAnchor: Date;
  readonly sessionMinutesByDay: Map<string, number>;
  readonly targetByDow: Map<number, number>;
  readonly onPrev: () => void;
  readonly onNext: () => void;
  readonly onGoToday: () => void;
  readonly today: Date;
}) {
  const monday = useMemo(() => startOfWeek(weekAnchor), [weekAnchor]);
  const days = useMemo(() => {
    const result: Date[] = [];
    for (let i = 0; i < 7; i++) {
      const d = new Date(monday);
      d.setDate(d.getDate() + i);
      result.push(d);
    }
    return result;
  }, [monday]);

  const weekEnd = useMemo(() => endOfWeek(weekAnchor), [weekAnchor]);

  const formatRange = () => {
    const startDay = monday.getDate();
    const endDay = weekEnd.getDate();
    const startMonth = monday.toLocaleString("de-DE", { month: "short" });
    const endMonth = weekEnd.toLocaleString("de-DE", { month: "short" });
    if (monday.getMonth() === weekEnd.getMonth()) {
      return `${startDay}. – ${endDay}. ${endMonth} ${weekEnd.getFullYear()}`;
    }
    return `${startDay}. ${startMonth} – ${endDay}. ${endMonth} ${weekEnd.getFullYear()}`;
  };

  return (
    <div>
      <div className="mb-4 grid grid-cols-3 items-center">
        <div />
        <div className="flex items-center justify-center gap-2">
          <button
            type="button"
            onClick={onPrev}
            aria-label="Vorherige Woche"
            className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
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
          </button>
          <h3 className="min-w-[14rem] text-center text-sm font-semibold text-gray-800">
            {formatRange()}
          </h3>
          <button
            type="button"
            onClick={onNext}
            aria-label="Nächste Woche"
            className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
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
          </button>
        </div>
        <div className="flex justify-end">
          <button
            type="button"
            onClick={onGoToday}
            className="rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50"
          >
            Diese Woche
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-7">
        {days.map((day, idx) => {
          const key = toDateKey(day);
          const dow = toIsoDayOfWeek(day);
          const actual = sessionMinutesByDay.get(key);
          const target = targetByDow.get(dow) ?? 0;
          const isToday = isSameDay(day, today);
          const isFuture = day > today;
          const delta = actual !== undefined ? actual - target : 0;
          const status =
            actual !== undefined ? getDeltaStatus(delta, target) : "gray";
          const borderColor =
            actual === undefined
              ? "border-gray-100"
              : status === "green"
                ? "border-green-200"
                : status === "amber"
                  ? "border-amber-200"
                  : "border-gray-200";

          return (
            <div
              key={`${key}-${idx}`}
              className={`rounded-2xl border ${borderColor} bg-white p-4 shadow-sm ${
                isToday ? "ring-2 ring-gray-900/10" : ""
              } ${isFuture ? "opacity-60" : ""}`}
            >
              <div className="flex items-baseline justify-between">
                <span className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
                  {dayLabels[dow]}
                </span>
                <span
                  className={`text-sm ${
                    isToday
                      ? "font-bold text-gray-900"
                      : "font-medium text-gray-600"
                  }`}
                >
                  {day.getDate()}.{day.getMonth() + 1}.
                </span>
              </div>
              <div className="mt-3 space-y-1.5 text-xs">
                <div className="flex items-center justify-between">
                  <span className="text-gray-400">Soll</span>
                  <span className="font-medium text-gray-700 tabular-nums">
                    {target > 0 ? formatDuration(target) : "–"}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-gray-400">Ist</span>
                  <span
                    className={`font-medium tabular-nums ${
                      actual === undefined
                        ? "text-gray-300"
                        : status === "green"
                          ? "text-green-600"
                          : status === "amber"
                            ? "text-amber-600"
                            : "text-gray-500"
                    }`}
                  >
                    {actual !== undefined ? formatDuration(actual) : "–"}
                  </span>
                </div>
                {actual !== undefined && target > 0 && (
                  <div className="flex items-center justify-between border-t border-gray-100 pt-1.5">
                    <span className="text-gray-400">Δ</span>
                    <span
                      className={`font-semibold tabular-nums ${
                        status === "green"
                          ? "text-green-600"
                          : status === "amber"
                            ? "text-amber-600"
                            : "text-gray-500"
                      }`}
                    >
                      {formatSignedDuration(delta)}
                    </span>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Schedule Edit Modal ─────────────────────────────────────────────────────

function ScheduleEditModal({
  isOpen,
  onClose,
  staffId,
  initialEntries,
  onSaved,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly staffId: string;
  readonly initialEntries: Array<{ dayOfWeek: number; targetMinutes: number }>;
  readonly onSaved: () => void;
}) {
  const toast = useToast();
  const [localEntries, setLocalEntries] = useState(initialEntries);
  const [saving, setSaving] = useState(false);

  // Reset local state whenever the modal is opened or input data changes.
  useEffect(() => {
    if (isOpen) setLocalEntries(initialEntries);
  }, [isOpen, initialEntries]);

  const updateDay = (dayOfWeek: number, minutes: number) => {
    const clamped = Math.max(0, Math.min(720, minutes));
    setLocalEntries((prev) =>
      prev.map((e) =>
        e.dayOfWeek === dayOfWeek ? { ...e, targetMinutes: clamped } : e,
      ),
    );
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const nonZero = localEntries.filter((e) => e.targetMinutes > 0);
      await staffScheduleService.updateSchedule(staffId, nonZero);
      toast.success("Dienstplan gespeichert");
      onSaved();
      onClose();
    } catch (error) {
      logger.error("schedule_save_failed", {
        error: error instanceof Error ? error.message : String(error),
        staff_id: staffId,
      });
      toast.error("Fehler beim Speichern");
    } finally {
      setSaving(false);
    }
  };

  const weeklyTotal = localEntries.reduce((sum, e) => sum + e.targetMinutes, 0);

  const footer = (
    <div className="flex items-center justify-between gap-3">
      <span className="text-xs text-gray-500">
        Wochensoll:{" "}
        <span className="font-bold text-gray-700">
          {formatDuration(weeklyTotal)}
        </span>
      </span>
      <div className="flex gap-2">
        <button
          type="button"
          onClick={onClose}
          disabled={saving}
          className="rounded-full border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-40"
        >
          Abbrechen
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving}
          className="rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {saving ? "Speichern..." : "Speichern"}
        </button>
      </div>
    </div>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Wochensoll bearbeiten"
      footer={footer}
    >
      <div className="space-y-3">
        <p className="text-xs text-gray-500">
          Die vertraglich vereinbarten Sollstunden pro Wochentag. Wochenenden
          sind bewusst ausgeklammert.
        </p>
        {localEntries.map((entry) => {
          const hours = Math.floor(entry.targetMinutes / 60);
          const mins = entry.targetMinutes % 60;
          return (
            <div
              key={entry.dayOfWeek}
              className="flex items-center gap-4 rounded-lg py-1.5"
            >
              <span className="w-10 shrink-0 text-sm font-medium text-gray-600">
                {dayLabels[entry.dayOfWeek]}
              </span>
              <input
                type="number"
                min={0}
                max={12}
                value={hours}
                onChange={(e) => {
                  const h = Math.max(
                    0,
                    Math.min(12, parseInt(e.target.value) || 0),
                  );
                  updateDay(entry.dayOfWeek, h * 60 + mins);
                }}
                className="w-14 rounded-lg border border-gray-200 px-2 py-1.5 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none"
              />
              <span className="text-xs text-gray-400">h</span>
              <select
                value={mins}
                onChange={(e) => {
                  const m = parseInt(e.target.value) || 0;
                  updateDay(entry.dayOfWeek, hours * 60 + m);
                }}
                className="w-16 rounded-lg border border-gray-200 px-2 py-1.5 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none"
              >
                <option value={0}>00</option>
                <option value={15}>15</option>
                <option value={30}>30</option>
                <option value={45}>45</option>
              </select>
              <span className="text-xs text-gray-400">min</span>
              <span className="ml-auto text-xs text-gray-500 tabular-nums">
                {formatDuration(entry.targetMinutes)}
              </span>
            </div>
          );
        })}
      </div>
    </Modal>
  );
}

// ─── Global Overflow Menu for the calendar ───────────────────────────────────

interface MenuItem {
  readonly label: string;
  readonly onClick?: () => void;
  readonly disabled?: boolean;
  readonly destructive?: boolean;
}

function CalendarKebabMenu({
  items,
}: {
  readonly items: ReadonlyArray<MenuItem | "divider">;
}) {
  const [open, setOpen] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open]);

  return (
    <div className="relative">
      <button
        ref={buttonRef}
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-label="Dienstplan-Aktionen"
        aria-haspopup="menu"
        aria-expanded={open}
        className="flex h-9 w-9 items-center justify-center rounded-full text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700"
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
            d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z"
          />
        </svg>
      </button>
      {open && (
        <>
          <button
            type="button"
            className="fixed inset-0 z-40 cursor-default"
            onClick={() => setOpen(false)}
            aria-label="Menue schliessen"
          />
          <div
            role="menu"
            className="absolute top-full right-0 z-50 mt-2 w-64 rounded-2xl border border-gray-200/50 bg-white/95 p-2 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md"
          >
            {items.map((item, idx) => {
              if (item === "divider") {
                return (
                  <div
                    key={`divider-${idx}`}
                    className="my-1 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent"
                  />
                );
              }
              const stateClasses = item.disabled
                ? "cursor-not-allowed text-gray-300"
                : item.destructive
                  ? "text-red-600 hover:bg-red-50 hover:text-red-700"
                  : "text-gray-700 hover:bg-gray-100 hover:text-gray-900";
              return (
                <button
                  key={item.label}
                  type="button"
                  disabled={item.disabled}
                  onClick={() => {
                    if (item.disabled || !item.onClick) return;
                    item.onClick();
                    setOpen(false);
                  }}
                  className={`flex w-full items-center rounded-xl px-3 py-2.5 text-left text-sm font-medium transition-all duration-150 ease-out ${stateClasses}`}
                >
                  {item.label}
                </button>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}

// ─── Main Tab ────────────────────────────────────────────────────────────────

export function DienstplanTab({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const today = useMemo(() => new Date(), []);

  const [viewMode, setViewMode] = useState<ViewMode>("month");
  const [monthAnchor, setMonthAnchor] = useState(() =>
    startOfMonth(new Date()),
  );
  const [weekAnchor, setWeekAnchor] = useState(() => startOfWeek(new Date()));
  const [scheduleModalOpen, setScheduleModalOpen] = useState(false);

  // Schedule fetch
  const {
    data: schedule,
    isLoading: scheduleLoading,
    mutate: mutateSchedule,
  } = useSWRAuth(`staff-schedule-${staffId}`, () =>
    staffScheduleService.getSchedule(staffId),
  );

  // Fetch YTD history for metrics (Jan 1 .. today)
  const ytdFrom = toDateKey(startOfYear(today));
  const ytdTo = toDateKey(today);
  const { data: ytdSessions, isLoading: ytdLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-ytd-${staffId}-${ytdFrom}-${ytdTo}`, () =>
    staffHistoryService.getHistory(staffId, ytdFrom, ytdTo),
  );

  // Fetch the visible range based on view mode. Month view covers full grid
  // (including the out-of-month days), week view covers the 7 days.
  const visibleFrom = useMemo(() => {
    if (viewMode === "month") {
      // Grid starts on the Monday of the week containing day 1
      const grid = buildMonthGrid(monthAnchor);
      const firstRow = grid[0];
      return firstRow?.[0]?.date ?? startOfMonth(monthAnchor);
    }
    return startOfWeek(weekAnchor);
  }, [viewMode, monthAnchor, weekAnchor]);

  const visibleTo = useMemo(() => {
    if (viewMode === "month") {
      const grid = buildMonthGrid(monthAnchor);
      const lastRow = grid[grid.length - 1];
      return lastRow?.[6]?.date ?? endOfMonth(monthAnchor);
    }
    return endOfWeek(weekAnchor);
  }, [viewMode, monthAnchor, weekAnchor]);

  const visibleFromKey = toDateKey(visibleFrom);
  const visibleToKey = toDateKey(visibleTo);
  const { data: visibleSessions, isLoading: visibleLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-visible-${staffId}-${visibleFromKey}-${visibleToKey}`, () =>
    staffHistoryService.getHistory(staffId, visibleFromKey, visibleToKey),
  );

  // Memoise the schedule entries array so downstream hooks don't
  // re-evaluate on every render (schedule?.entries returns a new reference
  // when `schedule` is defined but we want referential stability).
  const scheduleEntries = useMemo(() => schedule?.entries ?? [], [schedule]);

  const targetByDow = useMemo(() => {
    const map = new Map<number, number>();
    for (const entry of scheduleEntries) {
      map.set(entry.dayOfWeek, entry.targetMinutes);
    }
    return map;
  }, [scheduleEntries]);

  const sessionMinutesByDay = useMemo(
    () => groupSessionsByDay(visibleSessions ?? []),
    [visibleSessions],
  );

  const metrics = useMemo(
    () => computeStaffMetrics(scheduleEntries, ytdSessions ?? [], today),
    [scheduleEntries, ytdSessions, today],
  );

  const initialModalEntries = useMemo(
    () =>
      WORK_DAYS.map((d) => ({
        dayOfWeek: d,
        targetMinutes:
          scheduleEntries.find((e) => e.dayOfWeek === d)?.targetMinutes ?? 0,
      })),
    [scheduleEntries],
  );

  if (scheduleLoading || ytdLoading) {
    return <Loading fullPage={false} />;
  }

  const menuItems: ReadonlyArray<MenuItem | "divider"> = [
    {
      label: "Wochensoll bearbeiten",
      disabled: !canEdit,
      onClick: () => setScheduleModalOpen(true),
    },
    "divider",
    { label: "Session hinzufügen", disabled: true },
    { label: "Abwesenheit eintragen", disabled: true },
    { label: "Saldo korrigieren", disabled: true },
  ];

  const handlePrevMonth = () => {
    setMonthAnchor(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() - 1, 1),
    );
  };
  const handleNextMonth = () => {
    setMonthAnchor(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() + 1, 1),
    );
  };
  const handleGoToday = () => {
    if (viewMode === "month") {
      setMonthAnchor(startOfMonth(new Date()));
    } else {
      setWeekAnchor(startOfWeek(new Date()));
    }
  };
  const handlePrevWeek = () => {
    setWeekAnchor((prev) => {
      const next = new Date(prev);
      next.setDate(next.getDate() - 7);
      return next;
    });
  };
  const handleNextWeek = () => {
    setWeekAnchor((prev) => {
      const next = new Date(prev);
      next.setDate(next.getDate() + 7);
      return next;
    });
  };

  return (
    <div className="space-y-5">
      {/* KPI Cards */}
      <KpiCards metrics={metrics} />

      {/* Calendar Card */}
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
            Wochenplan
          </h3>
          <div className="flex items-center gap-2">
            <ViewToggle value={viewMode} onChange={setViewMode} />
            <CalendarKebabMenu items={menuItems} />
          </div>
        </div>

        {visibleLoading ? (
          <Loading fullPage={false} />
        ) : viewMode === "month" ? (
          <MonthCalendar
            monthAnchor={monthAnchor}
            sessionMinutesByDay={sessionMinutesByDay}
            targetByDow={targetByDow}
            onPrev={handlePrevMonth}
            onNext={handleNextMonth}
            onGoToday={handleGoToday}
            today={today}
          />
        ) : (
          <WeekView
            weekAnchor={weekAnchor}
            sessionMinutesByDay={sessionMinutesByDay}
            targetByDow={targetByDow}
            onPrev={handlePrevWeek}
            onNext={handleNextWeek}
            onGoToday={handleGoToday}
            today={today}
          />
        )}
      </div>

      {/* Schedule Edit Modal */}
      <ScheduleEditModal
        isOpen={scheduleModalOpen}
        onClose={() => setScheduleModalOpen(false)}
        staffId={staffId}
        initialEntries={initialModalEntries}
        onSaved={() => void mutateSchedule()}
      />
    </div>
  );
}
