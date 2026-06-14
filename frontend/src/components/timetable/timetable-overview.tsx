"use client";

import { CalendarClock, CalendarPlus, UserPlus } from "lucide-react";

import { TimetableStatCard } from "./timetable-stat-card";

interface TimetableOverviewProps {
  /** "Geplant" (week/month/year) or "Regeltermine" (series view). */
  readonly plannedLabel: string;
  readonly plannedCount: number;
  readonly plannedSublabel: string;
  readonly staffGapCount: number;
  readonly staffGapSublabel: string;
  readonly createLabel: string;
  readonly onCreate: () => void;
}

export function TimetableOverview({
  plannedLabel,
  plannedCount,
  plannedSublabel,
  staffGapCount,
  staffGapSublabel,
  createLabel,
  onCreate,
}: TimetableOverviewProps) {
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:p-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Überblick
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Betreuungsplan im Blick
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Schneller Überblick über geplante Termine und offene
            Personal-Lücken.
          </p>
        </div>
        <button
          type="button"
          onClick={onCreate}
          className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <CalendarPlus className="h-4 w-4" aria-hidden="true" />
          {createLabel}
        </button>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <TimetableStatCard
          size="lg"
          icon={<CalendarClock className="h-4 w-4" />}
          label={plannedLabel}
          value={String(plannedCount)}
          sublabel={plannedSublabel}
          tone="neutral"
        />
        <TimetableStatCard
          size="lg"
          icon={<UserPlus className="h-4 w-4" />}
          label="Ohne Personal"
          value={String(staffGapCount)}
          sublabel={staffGapSublabel}
          tone={staffGapCount > 0 ? "danger" : "neutral"}
        />
      </div>
    </section>
  );
}
