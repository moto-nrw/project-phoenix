"use client";

// Shared sub-components used across the staff Übersicht and Zeiterfassung
// tabs. All views derive their numbers from staff-metrics-helpers and the
// time-tracking-helpers formatters, so the visual treatment stays consistent
// regardless of where the cards or calendar are rendered.

import { useMemo } from "react";

import { formatDuration } from "~/lib/time-tracking-helpers";
import {
  buildMonthGrid,
  computeStaffMetrics,
  endOfWeek,
  formatMonthHeader,
  getDeltaStatus,
  isSameDay,
  startOfWeek,
  toDateKey,
  toIsoDayOfWeek,
} from "~/lib/staff-metrics-helpers";

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

export function formatSignedDuration(minutes: number): string {
  const sign = minutes > 0 ? "+" : minutes < 0 ? "−" : "";
  return `${sign}${formatDuration(Math.abs(minutes))}`;
}

// ─── KPI Cards ───────────────────────────────────────────────────────────────

export function KpiCard({
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

export function KpiCards({
  metrics,
}: {
  readonly metrics: ReturnType<typeof computeStaffMetrics>;
}) {
  const weekPct =
    metrics.weekSoll > 0 ? (metrics.weekIst / metrics.weekSoll) * 100 : 0;
  const monthPct =
    metrics.monthSoll > 0 ? (metrics.monthIst / metrics.monthSoll) * 100 : 0;

  const monthDeltaColor = getDeltaStatus(metrics.monthDelta, metrics.monthSoll);
  const accountColor: "green" | "amber" | "gray" =
    metrics.accountBalance > 0
      ? "amber"
      : metrics.accountBalance < -60
        ? "gray"
        : "green";

  // Localized "seit 13. Mai 2026" hint under the Stundenkonto card so users
  // see *which* anchor the cumulative balance is based on. Without it the
  // card silently shifts meaning as schedules get updated, which is exactly
  // the source-of-truth problem we want to surface.
  const accountStartLabel = metrics.accountStart.toLocaleDateString("de-DE", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });

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
        label="Stundenkonto"
        primary={formatSignedDuration(metrics.accountBalance)}
        secondary={`seit ${accountStartLabel}`}
        color={accountColor}
      />
    </div>
  );
}

// ─── View Toggle ─────────────────────────────────────────────────────────────

export type ViewMode = "month" | "week";

export function ViewToggle({
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

const ChevronLeft = () => (
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
);
const ChevronRight = () => (
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
);

export function MonthCalendar({
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
      <div className="mb-4 grid grid-cols-3 items-center">
        <div />
        <div className="flex items-center justify-center gap-2">
          <button
            type="button"
            onClick={onPrev}
            aria-label="Vorheriger Monat"
            className="rounded-full p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900"
          >
            <ChevronLeft />
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
            <ChevronRight />
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

      <div className="grid grid-cols-7 gap-1 border-b border-gray-100 pb-2 text-center text-xs font-semibold tracking-wider text-gray-400 uppercase">
        {dayLabels.map((label) => (
          <div key={label}>{label}</div>
        ))}
      </div>
      <div className="mt-2 grid grid-cols-7 gap-1">
        {grid.flat().map((cell, idx) => {
          const day = cell.date;
          const inMonth = cell.inMonth;
          const key = toDateKey(day);
          const dow = toIsoDayOfWeek(day);
          const actual = sessionMinutesByDay.get(key);
          const target = targetByDow.get(dow) ?? 0;
          const isToday = isSameDay(day, today);
          const isFuture = day > today;
          const delta = actual !== undefined ? actual - target : 0;
          const status =
            actual !== undefined ? getDeltaStatus(delta, target) : "gray";
          const cellTextColor = inMonth ? "text-gray-700" : "text-gray-300";
          const ringColor = isToday ? "ring-2 ring-gray-900/10" : "";
          const minutesColor =
            actual === undefined
              ? "text-gray-300"
              : status === "green"
                ? "text-green-600"
                : status === "amber"
                  ? "text-amber-600"
                  : "text-gray-500";

          return (
            <div
              key={`${key}-${idx}`}
              className={`min-h-[68px] rounded-xl border border-gray-100 bg-white p-2 text-xs ${cellTextColor} ${ringColor} ${
                isFuture ? "opacity-50" : ""
              }`}
            >
              <div
                className={`flex items-center justify-between ${
                  isToday ? "font-bold" : "font-medium"
                }`}
              >
                <span>{day.getDate()}</span>
                {target > 0 && (
                  <span className="text-[10px] text-gray-400 tabular-nums">
                    {formatDuration(target)}
                  </span>
                )}
              </div>
              {actual !== undefined && (
                <div
                  className={`mt-1 text-right text-[11px] tabular-nums ${minutesColor}`}
                >
                  {formatDuration(actual)}
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

export function WeekView({
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
            <ChevronLeft />
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
            <ChevronRight />
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
