"use client";

// Shared sub-components used across the staff Übersicht and Zeiterfassung
// tabs. All views derive their numbers from staff-metrics-helpers and the
// time-tracking-helpers formatters, so the visual treatment stays consistent
// regardless of where the cards or calendar are rendered.

import { OriginChip } from "~/components/ui/origin-chip";
import { formatDuration } from "~/lib/time-tracking-helpers";
import type { PeriodMetrics } from "~/lib/hooks/use-period-metrics";
import { getDeltaStatus } from "~/lib/staff-metrics-helpers";

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
  compactPrimary,
}: {
  readonly label: string;
  readonly primary: string;
  readonly secondary?: string;
  readonly progressPct?: number;
  readonly color?: "green" | "amber" | "gray" | "red";
  /** Schulweite Summen sind vierstellig ("1521h 11min") und brechen sonst
   *  mitten im Wert um. Eine Stufe kleiner und ohne Umbruch. */
  readonly compactPrimary?: boolean;
}) {
  const primaryColor = {
    green: "text-moto-green-hover",
    amber: "text-moto-amber-strong",
    gray: "text-gray-700",
    red: "text-moto-red-strong",
  }[color ?? "gray"];
  const barColor = {
    green: "bg-moto-green",
    amber: "bg-moto-amber",
    gray: "bg-gray-400",
    red: "bg-moto-red",
  }[color ?? "gray"];
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
      <p className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
        {label}
      </p>
      <p
        className={`mt-2 font-bold ${primaryColor} ${
          compactPrimary ? "text-xl whitespace-nowrap" : "text-2xl"
        }`}
      >
        {primary}
      </p>
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
  // Every figure is date-valid, straight from the backend month model
  // (usePeriodMetrics, #1842). Never computeStaffMetrics: that one prices
  // historical days at the CURRENT schedule and contradicts the Monatskarte
  // and the daily rows below it after a contract change. null = loading.
  readonly metrics: PeriodMetrics;
}) {
  const { week, month, accountBalanceMinutes } = metrics;

  const weekPct = week && week.soll > 0 ? (week.ist / week.soll) * 100 : 0;
  const monthPct = month && month.soll > 0 ? (month.ist / month.soll) * 100 : 0;

  const monthDeltaColor = month === null ? "gray" : getDeltaStatus(month.delta);
  const accountColor =
    accountBalanceMinutes === null
      ? "gray"
      : getDeltaStatus(accountBalanceMinutes);

  // Localized "seit 13. Mai 2026" hint under the Stundenkonto card so users
  // see *which* anchor the cumulative balance is based on. Without it the
  // card silently shifts meaning as schedules get updated, which is exactly
  // the source-of-truth problem we want to surface.
  const accountStartLabel = metrics.accountStart.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  const dash = "–";

  return (
    <div className="space-y-2">
      {/* Herkunfts-Chip am Soll-Wert (Planung-Redesign, docs/04 6.2): genau
          einer pro Oberfläche, solange die Soll-Quellen-Frage offen ist. */}
      <div className="flex justify-end">
        <OriginChip
          label="Soll aus Arbeitszeitmodell"
          title="Wochensaldo und Stundenkonto rechnen gegen das im Arbeitszeitmodell hinterlegte Soll."
        />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard
          label="Diese Woche"
          primary={week === null ? dash : formatDuration(week.ist)}
          secondary={
            week === null ? undefined : `von ${formatDuration(week.soll)} Soll`
          }
          progressPct={week === null ? undefined : weekPct}
          color={week === null ? "gray" : getDeltaStatus(week.delta)}
        />
        <KpiCard
          label="Dieser Monat"
          primary={month === null ? dash : formatDuration(month.ist)}
          secondary={
            month === null
              ? undefined
              : `von ${formatDuration(month.soll)} Soll`
          }
          progressPct={month === null ? undefined : monthPct}
          color={monthDeltaColor}
        />
        <KpiCard
          label="Überstunden Monat"
          primary={month === null ? dash : formatSignedDuration(month.delta)}
          secondary={
            month === null
              ? undefined
              : month.delta === 0
                ? "ausgeglichen"
                : month.delta > 0
                  ? "Überstunden"
                  : "Minusstunden"
          }
          color={monthDeltaColor}
        />
        <KpiCard
          label="Stundenkonto"
          primary={
            accountBalanceMinutes === null
              ? dash
              : formatSignedDuration(accountBalanceMinutes)
          }
          secondary={
            accountBalanceMinutes === null
              ? `seit ${accountStartLabel}`
              : accountBalanceMinutes === 0
                ? `Soll und Ist ausgeglichen seit ${accountStartLabel}`
                : `seit ${accountStartLabel}`
          }
          color={accountColor}
        />
      </div>
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
