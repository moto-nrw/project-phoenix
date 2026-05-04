"use client";

import { useMemo } from "react";

import { Loading } from "~/components/ui/loading";
import { staffHistoryService, staffScheduleService } from "~/lib/staff-api";
import type { StaffHistorySession } from "~/lib/staff-api";
import {
  computeStaffMetrics,
  startOfYear,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";

import { KpiCards } from "./staff-time-views";

// Quick-look summary tab for the staff detail page. Reads the schedule and
// the YTD session history to derive Soll/Ist/Saldo and renders the KPI
// cards. Other surfaces (Dienstplan, Zeiterfassung, Abwesenheiten) own
// their own data fetches.
export function UebersichtTab({ staffId }: { readonly staffId: string }) {
  const today = useMemo(() => new Date(), []);

  const { data: schedule, isLoading: scheduleLoading } = useSWRAuth(
    `staff-schedule-${staffId}`,
    () => staffScheduleService.getSchedule(staffId),
  );

  const ytdFrom = toDateKey(startOfYear(today));
  const ytdTo = toDateKey(today);
  const { data: ytdSessions, isLoading: ytdLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-ytd-${staffId}-${ytdFrom}-${ytdTo}`, () =>
    staffHistoryService.getHistory(staffId, ytdFrom, ytdTo),
  );

  const scheduleEntries = useMemo(() => schedule?.entries ?? [], [schedule]);

  const metrics = useMemo(
    () => computeStaffMetrics(scheduleEntries, ytdSessions ?? [], today),
    [scheduleEntries, ytdSessions, today],
  );

  if (scheduleLoading || ytdLoading) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="space-y-5">
      <KpiCards metrics={metrics} />
    </div>
  );
}
