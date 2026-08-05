"use client";

// Shared sub-components used across the staff Übersicht and Zeiterfassung
// tabs. All views derive their numbers from staff-metrics-helpers and the
// time-tracking-helpers formatters, so the visual treatment stays consistent
// regardless of where the cards or calendar are rendered.

import { OriginChip } from "~/components/ui/origin-chip";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import { StatCard } from "~/components/ui/stat-card";
import { formatDuration } from "~/lib/time-tracking-helpers";
import type { PeriodMetrics } from "~/lib/hooks/use-period-metrics";
import { getDeltaStatus } from "~/lib/staff-metrics-helpers";

export function formatSignedDuration(minutes: number): string {
  const sign = minutes > 0 ? "+" : minutes < 0 ? "−" : "";
  return `${sign}${formatDuration(Math.abs(minutes))}`;
}

// ─── KPI Cards ───────────────────────────────────────────────────────────────

/**
 * Thin adapter over the kit's {@link StatCard}. Kept so the existing callers
 * keep their prop names (`primary`/`secondary`/`color`) and so
 * `getDeltaStatus`'s older "amber" spelling maps to the kit tone vocabulary in
 * exactly one place.
 */
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
  return (
    <StatCard
      label={label}
      value={primary}
      hint={secondary}
      progressPct={progressPct}
      tone={color === "amber" ? "orange" : (color ?? "gray")}
      compactValue={compactPrimary}
    />
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

const VIEW_MODE_ITEMS: readonly SegmentedControlItem<ViewMode>[] = [
  { value: "month", label: "Monat" },
  { value: "week", label: "Woche" },
];

export function ViewToggle({
  value,
  onChange,
}: {
  readonly value: ViewMode;
  readonly onChange: (v: ViewMode) => void;
}) {
  return (
    <SegmentedControl
      ariaLabel="Zeitraum-Ansicht"
      items={VIEW_MODE_ITEMS}
      value={value}
      onChange={onChange}
    />
  );
}
