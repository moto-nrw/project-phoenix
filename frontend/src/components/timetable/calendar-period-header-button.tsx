"use client";

/**
 * CalendarPeriodHeaderButton — small affordance in the planner header that
 * shows the currently active calendar period and lets an admin open the
 * editor with one click. Mounts on /database/timetables next to the
 * MaterializeButton.
 *
 * Why it exists: per iteration-4 / E15 the calendar period concept should
 * stay invisible for the common case (one school year, set up once via
 * tenant provisioning). A standalone admin page was the wrong shape — too
 * prominent for something 99% of schools never touch. This button is the
 * minimal surface: shows the truth ("Schuljahr 2025/2026"), nudges to fix
 * when missing ("Schuljahr anlegen"), and otherwise stays out of the way.
 */

import { useMemo } from "react";

import { calendarPeriodService } from "~/lib/calendar-period-api";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { useSWRAuth } from "~/lib/swr";

const SWR_KEY = "database-calendar-periods-list";

interface CalendarPeriodHeaderButtonProps {
  /** Called with the matching period when an admin wants to edit it. */
  onEdit: (period: CalendarPeriod) => void;
  /** Called when there is no active period and the admin wants to create one. */
  onCreate: () => void;
}

export function CalendarPeriodHeaderButton({
  onEdit,
  onCreate,
}: CalendarPeriodHeaderButtonProps) {
  const { data, isLoading } = useSWRAuth(SWR_KEY, () =>
    calendarPeriodService.list(),
  );

  const activePeriod = useMemo(() => activeForToday(data ?? []), [data]);

  if (isLoading) {
    // Stay quiet on first paint — the materialize button + week navigator
    // already give the page weight; an animated skeleton would only add noise.
    return null;
  }

  if (!activePeriod) {
    return (
      <button
        type="button"
        onClick={onCreate}
        className="inline-flex items-center gap-2 rounded-md border border-[#FACC15] bg-[#FEF9C3] px-3 py-2 text-xs font-semibold text-[#854D0E] shadow-sm transition-colors hover:bg-[#FEF08A]"
        title="Ohne aktive Kalenderperiode kann der Plan nicht materialisiert werden."
      >
        Schuljahr anlegen
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={() => onEdit(activePeriod)}
      className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 shadow-sm transition-colors hover:border-slate-300 hover:bg-slate-50"
      title="Aktive Kalenderperiode bearbeiten"
    >
      <span>{activePeriod.name}</span>
      <span aria-hidden className="text-slate-400">
        ▾
      </span>
    </button>
  );
}

/**
 * Picks the period that is_active AND covers today's civil date. If multiple
 * match (unusual — overlapping school year + holiday), prefers the lowest
 * id, mirroring the materialization service's selectPeriod tie-break.
 */
function activeForToday(periods: CalendarPeriod[]): CalendarPeriod | null {
  const today = todayISO();
  const matches = periods
    .filter((p) => p.isActive)
    .filter((p) => p.startDate <= today && today <= p.endDate);
  if (matches.length === 0) return null;
  matches.sort((a, b) => Number(a.id) - Number(b.id));
  return matches[0]!;
}

function todayISO(): string {
  const d = new Date();
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}
