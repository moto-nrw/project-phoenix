"use client";

import { useCallback, useMemo } from "react";

import { DienstplanPersonWeekGrid } from "~/components/staff/dienstplan-person-week-grid";
import { DienstplanGridSkeleton } from "~/components/staff/dienstplan-skeleton";
import { OwnWeekAgenda } from "~/components/time-tracking/own-week-agenda";
import { OwnWeekHoursCard } from "~/components/time-tracking/own-week-hours-card";
import { Button } from "~/components/ui/button";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { staffPlanConcept } from "~/lib/concepts/staffPlan";
import {
  formatDate,
  isValidISODate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useClosingDaysState } from "~/lib/hooks/use-closing-days";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useUrlParams } from "~/lib/hooks/use-url-params";
import {
  calendarWeekDays,
  earliestShiftDate,
  LOOKAHEAD_WEEKS,
  lookaheadRange,
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
import { timeTrackingKeys } from "~/lib/swr/portal-keys";
import { useModuleBooked } from "~/lib/tenant-context";
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

const EMPTY_SHIFTS: readonly StaffShift[] = [];

export function OwnDienstplanView() {
  const { isReady } = useRequirePermission("time_tracking:own");

  if (!isReady) {
    return (
      <TenantPage
        title="Mein Dienstplan"
        loading={<DienstplanGridSkeleton />}
      />
    );
  }

  return <OwnDienstplanContent />;
}

function OwnDienstplanContent() {
  // Die Seite gehört zum Modul Dienstplan und hängt nicht mehr am
  // Betreuungsplan (#4812). Diese Datenkomponente erreicht React erst nach
  // der Modulprüfung; das Soll kommt aus der Zeiterfassung und fehlt ohne
  // dieses Modul.
  const zeiterfassungBooked = useModuleBooked("zeiterfassung");
  const today = useBerlinToday();
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
    timeTrackingKeys.ownShiftsWeek(weekFrom, weekTo),
    // Ohne keepPreviousData: beim Wochenwechsel stünden sonst kurz die
    // Schichten der alten Woche unter dem neuen Wochenetikett.
    () => ownShiftService.getOwnShifts(weekFrom, weekTo),
  );

  // Derselbe Key und dieselbe Nutzlast wie Tagestabelle und Wochen-KPI der
  // Zeiterfassung: das Soll hier ist dasselbe wie dort.
  const {
    data: projection,
    error: projectionError,
    mutate: mutateProjection,
  } = useSWRAuth(
    zeiterfassungBooked
      ? `time-tracking-schedule-targets-${weekFrom}-${weekTo}`
      : null,
    () => timeTrackingService.getDailyProjection(weekFrom, weekTo),
    { revalidateOnFocus: false },
  );

  const { closingDays } = useClosingDaysState(weekFrom, weekTo);

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
  const hasShiftData = shifts !== undefined;
  const weekIsEmpty = hasShiftData && weekShifts.length === 0;

  // Leere Woche: in den Wochen danach nach der nächsten Schicht suchen, damit
  // niemand Woche für Woche weiterklickt (#5639).
  const ahead = lookaheadRange(weekTo);
  const {
    data: aheadShifts,
    error: aheadError,
    mutate: mutateAheadShifts,
  } = useSWRAuth<StaffShift[]>(
    weekIsEmpty ? timeTrackingKeys.ownShiftsAhead(ahead.from, ahead.to) : null,
    () => ownShiftService.getOwnShifts(ahead.from, ahead.to),
  );
  const aheadLoadError = useSwrLoadError(
    aheadError,
    "die Suche nach der nächsten Schicht",
    () => mutateAheadShifts(),
  );
  const nextShiftDate = aheadShifts ? earliestShiftDate(aheadShifts) : null;

  return (
    <TenantPage
      title="Mein Dienstplan"
      testId="own-dienstplan-page"
      stats={hasShiftData ? <TenantPageStats items={statsItems} /> : undefined}
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
          : projectionError
            ? {
                message:
                  "Ihr Soll für diese Woche konnte nicht geladen werden. Bitte versuchen Sie es noch einmal.",
                action: (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => void mutateProjection()}
                  >
                    Erneut laden
                  </Button>
                ),
                keepContent: true,
              }
            : aheadLoadError
              ? {
                  message: aheadLoadError.message,
                  action: errorAlertActions(aheadLoadError),
                  // Der aktuelle Leerzustand ist bestätigt. Nur die
                  // Vorausschau fehlt und darf ihn nicht ersetzen.
                  keepContent: true,
                }
              : null
      }
      empty={
        weekIsEmpty && !shiftsError
          ? {
              icon: <MotoConceptIcon concept={staffPlanConcept} size={48} />,
              title: "Keine Schichten in dieser Woche",
              description: emptyWeekDescription(aheadShifts, nextShiftDate),
              action:
                nextShiftDate === null ? undefined : (
                  <Button
                    type="button"
                    variant="primary"
                    size="md"
                    onClick={() => updateParams({ d: nextShiftDate })}
                  >
                    Zur nächsten Schicht
                  </Button>
                ),
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
      {/* Raster ab lg, darunter eine Liste je Tag: dieselbe Grenze wie
          „Mein Kalender“, damit man auf dem Handy nicht seitlich wischt. */}
      <div className="hidden lg:block">
        <DienstplanPersonWeekGrid
          shiftsByDate={shiftsByDate}
          weekDays={gridDays}
          todayIso={today}
          closingDays={closingDays}
          typesById={typesById}
          shiftTypes={shiftTypes}
        />
      </div>
      <div className="lg:hidden">
        <OwnWeekAgenda
          weekDays={weekDays}
          shiftsByDate={shiftsByDate}
          typesById={typesById}
          closingDays={closingDays}
        />
      </div>
      <OwnWeekHoursCard
        weekDays={weekDays}
        shiftsByDate={shiftsByDate}
        typesById={typesById}
        shiftTypes={shiftTypes}
      />
    </TenantPage>
  );
}

/**
 * Text des Leerzustands: wann die nächste Schicht kommt, sonst ob nur
 * ausgefallene Schichten folgen und wer weitere Schichten plant.
 * Solange die Suche läuft, bleibt es beim Hinweis auf die Leitung. Scheitert
 * sie, zeigt die Seite darüber einen Ladefehler mit Wiederholen; eine
 * erfundene „keine weiteren Schichten" wäre schlimmer.
 */
function emptyWeekDescription(
  aheadShifts: readonly StaffShift[] | undefined,
  nextShiftDate: string | null,
): string {
  if (nextShiftDate !== null) {
    // Geschützte Leerzeichen: das Datum bricht nicht mitten in der Zeile um.
    const date = formatDate(nextShiftDate, true).replaceAll(" ", "\u00A0");
    return `Ihre nächste Schicht nach dieser Woche ist am ${date}.`;
  }
  if (aheadShifts?.length) {
    return `In den ${LOOKAHEAD_WEEKS} Wochen danach sind nur ausgefallene Schichten eingetragen. Ihre Schichten plant die Leitung. Fragen Sie dort nach, wenn Sie eine Schicht erwarten.`;
  }
  if (aheadShifts !== undefined) {
    return `Auch in den ${LOOKAHEAD_WEEKS} Wochen danach ist keine Schicht eingetragen. Ihre Schichten plant die Leitung. Fragen Sie dort nach, wenn Sie eine Schicht erwarten.`;
  }
  return "Ihre Schichten plant die Leitung im Dienstplan. Sobald welche eingetragen sind, sehen Sie sie hier.";
}
