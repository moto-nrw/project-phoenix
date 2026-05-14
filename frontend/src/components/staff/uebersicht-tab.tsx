"use client";

import { useMemo } from "react";

import { Loading } from "~/components/ui/loading";
import {
  staffAbsenceService,
  staffHistoryService,
  staffScheduleService,
} from "~/lib/staff-api";
import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";
import {
  computeStaffMetrics,
  startOfYear,
  toDateKey,
} from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";

import { KpiCards } from "./staff-time-views";

const EMPTY_SCHEDULE: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "",
  entries: [],
  weeklyTotals: [],
  validFrom: "",
};

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

  // Anchor the cumulative balance at the schedule's validFrom when available,
  // otherwise fall back to Jan 1 of the current year. This keeps the
  // Stundenkonto-card honest: it never claims Soll for time periods in which
  // no schedule was in effect.
  const accountAnchor = useMemo(() => {
    const vf = schedule?.validFrom ?? "";
    if (vf.length >= 10) {
      const [y, m, d] = vf.slice(0, 10).split("-").map(Number);
      if (y && m && d) return new Date(y, m - 1, d);
    }
    return startOfYear(today);
  }, [schedule?.validFrom, today]);

  const accountFrom = toDateKey(accountAnchor);
  const accountTo = toDateKey(today);
  const { data: accountSessions, isLoading: sessionsLoading } = useSWRAuth<
    StaffHistorySession[]
  >(`staff-history-account-${staffId}-${accountFrom}-${accountTo}`, () =>
    staffHistoryService.getHistory(staffId, accountFrom, accountTo),
  );
  const { data: accountAbsences, isLoading: absencesLoading } = useSWRAuth<
    StaffAbsenceRow[]
  >(`staff-absences-account-${staffId}-${accountFrom}-${accountTo}`, () =>
    staffAbsenceService.getAbsences(staffId, accountFrom, accountTo),
  );

  const metrics = useMemo(
    () =>
      computeStaffMetrics(
        schedule ?? EMPTY_SCHEDULE,
        accountSessions ?? [],
        accountAbsences ?? [],
        today,
      ),
    [schedule, accountSessions, accountAbsences, today],
  );

  if (scheduleLoading || sessionsLoading || absencesLoading) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="space-y-5">
      <KpiCards metrics={metrics} />
    </div>
  );
}
