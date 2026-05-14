"use client";

// Shared sub-components used across the staff Übersicht and Zeiterfassung
// tabs. All views derive their numbers from staff-metrics-helpers and the
// time-tracking-helpers formatters, so the visual treatment stays consistent
// regardless of where the cards or calendar are rendered.

import { formatDuration } from "~/lib/time-tracking-helpers";
import {
  computeStaffMetrics,
  getDeltaStatus,
} from "~/lib/staff-metrics-helpers";

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
    green: "text-[#70b525]",
    amber: "text-amber-600",
    gray: "text-gray-700",
    red: "text-red-600",
  }[color ?? "gray"];
  const barColor = {
    green: "bg-[#83CD2D]",
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

  const monthDeltaColor = getDeltaStatus(metrics.monthDelta);
  const accountColor = getDeltaStatus(metrics.accountBalance);

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
        color={getDeltaStatus(metrics.weekDelta)}
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
