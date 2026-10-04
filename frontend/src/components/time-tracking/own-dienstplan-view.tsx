"use client";

import { useCallback, useMemo } from "react";

import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { DienstplanPersonWeekGrid } from "~/components/staff/dienstplan-person-week-grid";
import { DienstplanGridSkeleton } from "~/components/staff/dienstplan-skeleton";
import { OwnWeekHoursCard } from "~/components/time-tracking/own-week-hours-card";
import { Button } from "~/components/ui/button";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { isValidISODate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useClosingDaysState } from "~/lib/hooks/use-closing-days";
import { useUrlParams } from "~/lib/hooks/use-url-params";
import {
  calendarWeekDays,
  plannedWeekMinutes,
  shiftTypesFromShifts,
  targetWeekMinutes,
  visibleWeekDays,
} from "~/lib/own-dienstplan-helpers";
import { ownShiftService } from "~/lib/shift-api";
import {
  formatDeltaHours,
  formatPlannedHours,
  type StaffShift,
} from "~/lib/shift-helpers";
import { indexShiftTypes } from "~/lib/shift-type-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTimetableEnabled } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { timeTrackingService } from "~/lib/time-tracking-api";
import { formatWeekLabel } from "~/lib/timetable-helpers";

// Eigener Dienstplan der Mitarbeitenden (#3821): die Woche mit allen
// geplanten Schichten nach Art, den Summen und dem Soll, nur lesend. Das
// Raster ist dasselbe wie in der Dienstplan-Ansicht „Person“ (#3818), ohne
// Anlegen und Bearbeiten. Daten kommen ausschließlich über die eigenen
// Zeiterfassungs-Endpunkte (time_tracking:own): eigene Schichten mit Name und
// Farbe der Schichtart, und die Tagesprojektion für das Soll.
//
// URL-Vokabular: nur `d` (ein Tag der angezeigten Woche), wie im Dienstplan.

const ALLOWED_URL_PARAMS = ["d"] as const;

/** SWR-Präfix der Wochen-Schichten; steht in PLAN_CACHE_KEY_PREFIXES. */
const OWN_WEEK_SHIFTS_KEY_PREFIX = "time-tracking-own-shifts-week-";

const EMPTY_SHIFTS: readonly StaffShift[] = [];

export function OwnDienstplanView() {
  const timetableEnabled = useTimetableEnabled();
  const today = useBerlinToday();
  const tenantPath = useTenantAwarePath();
  const { params, updateParams } = useUrlParams(ALLOWED_URL_PARAMS);

  const rawDay = params.d;
  const dayISO = rawDay !== null && isValidISODate(rawDay) ? rawDay : today;
  const weekDays = useMemo(() => calendarWeekDays(dayISO), [dayISO]);
  const weekFrom = weekDays[0] ?? "";
  const weekTo = weekDays[6] ?? "";

  const {
    data: shifts,
    error: shiftsError,
    mutate: mutateShifts,
  } = useSWRAuth<StaffShift[]>(
    timetableEnabled
      ? `${OWN_WEEK_SHIFTS_KEY_PREFIX}${weekFrom}-${weekTo}`
      : null,
    // Ohne keepPreviousData: beim Wochenwechsel stünden sonst kurz die
    // Schichten der alten Woche unter dem neuen Wochenetikett.
    () => ownShiftService.getOwnShifts(weekFrom, weekTo),
  );

  // Derselbe Key und dieselbe Nutzlast wie Tagestabelle und Wochen-KPI der
  // Zeiterfassung: das Soll hier ist dasselbe wie dort.
  const { data: projection } = useSWRAuth(
    timetableEnabled
      ? `time-tracking-schedule-targets-${weekFrom}-${weekTo}`
      : null,
    () => timeTrackingService.getDailyProjection(weekFrom, weekTo),
    { revalidateOnFocus: false },
  );

  const { closingDays } = useClosingDaysState(
    timetableEnabled ? weekFrom : "",
    timetableEnabled ? weekTo : "",
  );

  const weekShifts = shifts ?? EMPTY_SHIFTS;
  const shiftsByDate = useMemo(() => {
    const byDate = new Map<string, StaffShift[]>();
    for (const shift of weekShifts) {
      const dayShifts = byDate.get(shift.date);
      if (dayShifts) dayShifts.push(shift);
      else byDate.set(shift.date, [shift]);
    }
    return byDate;
  }, [weekShifts]);
  const shiftTypes = useMemo(
    () => shiftTypesFromShifts(weekShifts),
    [weekShifts],
  );
  const typesById = useMemo(() => indexShiftTypes(shiftTypes), [shiftTypes]);
  const gridDays = useMemo(
    () => visibleWeekDays(weekDays, weekShifts),
    [weekDays, weekShifts],
  );

  const plannedMinutes = plannedWeekMinutes(weekShifts);
  const targetMinutes = targetWeekMinutes(weekDays, projection);

  const weekLabel = useMemo(() => {
    const monday = parseISODate(weekFrom);
    const friday = new Date(monday);
    friday.setDate(friday.getDate() + 4);
    return formatWeekLabel(monday, friday);
  }, [weekFrom]);
  const isOnCurrentWeek = calendarWeekDays(today)[0] === weekFrom;

  // weekFrom ist ein Montag, also landet die Navigation auf dem Montag der
  // Zielwoche.
  const goToWeek = useCallback(
    (deltaDays: number) => {
      const target = parseISODate(weekFrom);
      target.setDate(target.getDate() + deltaDays);
      updateParams({ d: toISODate(target) });
    },
    [weekFrom, updateParams],
  );

  if (!timetableEnabled) {
    return (
      <PlanningDisabledState
        pageTitle="Mein Dienstplan"
        heading="Der Dienstplan ist nicht verfügbar"
        description="Ihre Schule plant die Schichten nicht in moto."
        testId="own-dienstplan-disabled-state"
      />
    );
  }

  const statsItems = [
    { value: formatPlannedHours(plannedMinutes), label: "geplant" },
    ...(targetMinutes === null
      ? []
      : [
          { value: formatPlannedHours(targetMinutes), label: "Soll" },
          {
            value: formatDeltaHours(plannedMinutes - targetMinutes),
            label: "Differenz",
          },
        ]),
  ];

  // Noch keine Daten und kein Fehler heißt laden: useSWRAuth hält den Abruf
  // zurück, bis die Session steht, und meldet so lange isLoading=false. Eine
  // leere Woche darf erst nach einer Antwort erscheinen.
  const loading = shifts === undefined && !shiftsError;

  return (
    <TenantPage
      title="Mein Dienstplan"
      testId="own-dienstplan-page"
      back
      backHref={tenantPath("/time-tracking")}
      backLabel="Zurück zur Zeiterfassung"
      stats={loading ? undefined : <TenantPageStats items={statsItems} />}
      statsLoading={loading}
      loading={loading ? <DienstplanGridSkeleton /> : false}
      error={
        shiftsError
          ? {
              message:
                "Ihr Dienstplan konnte nicht geladen werden. Bitte versuchen Sie es noch einmal.",
              action: (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={() => void mutateShifts()}
                >
                  Erneut laden
                </Button>
              ),
            }
          : null
      }
      empty={
        !loading && !shiftsError && weekShifts.length === 0
          ? {
              title: "Keine Schichten in dieser Woche",
              description:
                "Ihre Schichten plant die Leitung im Dienstplan. Sobald welche eingetragen sind, sehen Sie sie hier.",
            }
          : undefined
      }
      searchSlot={
        <PlanningContextBar
          withoutContextRow
          dateLabel={weekLabel}
          onPrevious={() => goToWeek(-7)}
          onNext={() => goToWeek(7)}
          previousLabel="Vorherige Woche"
          nextLabel="Nächste Woche"
          onToday={
            isOnCurrentWeek ? undefined : () => updateParams({ d: null })
          }
          todayLabel="Diese Woche"
        />
      }
    >
      <DienstplanPersonWeekGrid
        shiftsByDate={shiftsByDate}
        weekDays={gridDays}
        todayIso={today}
        closingDays={closingDays}
        typesById={typesById}
        shiftTypes={shiftTypes}
      />
      <OwnWeekHoursCard
        weekDays={weekDays}
        shiftsByDate={shiftsByDate}
        typesById={typesById}
        shiftTypes={shiftTypes}
      />
    </TenantPage>
  );
}
