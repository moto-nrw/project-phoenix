"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";

import { Printer, Settings2 } from "lucide-react";

import { PlanExportModal } from "~/components/planning/plan-export-modal";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { PeriodSwitcherDropdown } from "~/components/timetable/period-switcher-dropdown";
import { DienstplanHalbjahrGrid } from "~/components/staff/dienstplan-halbjahr-grid";
import { DienstplanHoursCard } from "~/components/staff/dienstplan-hours-card";
import { DienstplanPersonWeekGrid } from "~/components/staff/dienstplan-person-week-grid";
import { DienstplanResourceGrid } from "~/components/staff/dienstplan-resource-grid";
import { DienstplanGridSkeleton } from "~/components/staff/dienstplan-skeleton";
import {
  ShiftEditModal,
  type ShiftEditMode,
} from "~/components/staff/shift-edit-modal";
import { SickReportModal } from "~/components/staff/sick-report-modal";
import { Alert } from "~/components/ui/alert";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { TenantPage } from "~/components/ui/tenant-page";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiLoadError } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import { isValidISODate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useClosingDaysState } from "~/lib/hooks/use-closing-days";
import { useDienstplanData } from "~/lib/hooks/use-dienstplan-data";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useUrlParams } from "~/lib/hooks/use-url-params";
import { createLogger } from "~/lib/logger";
import { getSettingValue } from "~/lib/settings-api";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import { startOfWeek } from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  firstSchoolDayInPeriod,
  formatWeekLabel,
} from "~/lib/timetable-helpers";
import { userContextService } from "~/lib/usercontext-api";

const logger = createLogger({ component: "DienstplanView" });

// Admin week view for planned staff shifts (Dienstplan, #1376 core slice).
// One row per staff member, Mo–Fr columns, click-to-edit per cell. The
// planned shift end also drives the automatic checkout (#1798) when the
// tenant setting "Automatisch ausstempeln" is enabled.
//
// URL-Vokabular: genau
// `d` (Berlin-Kalendertag; die angezeigte Woche ist die Woche, die `d` enthält),
// `view` ("woche" | "person" | "halbjahr") und `staff` (die Person der
// Ansicht "person", #3818). Ungültige Werte fallen still auf die Defaults
// zurück (heute, "woche", die eigene bzw. erste Person). Modals bleiben reiner
// React-State.

type DienstplanView = "woche" | "person" | "halbjahr";

// updateUrlParams baut die URL aus dieser Allowlist neu auf, damit fremde
// Params (?utm_source=…) nicht jeden Wochen-/Ansichtswechsel überleben.
const ALLOWED_URL_PARAMS = ["d", "view", "staff"] as const;

interface ModalState {
  mode: ShiftEditMode;
  staff: StaffScheduleStaff;
  date: string;
  shift: StaffShift | null;
  // The covers attached to `shift` (rows whose originShiftId is shift.id),
  // captured at open time so a background SWR revalidation cannot swap them out
  // from under an in-progress edit — same freeze as `shift` itself (#1841).
  replacements: readonly StaffShift[];
  /** Im Viertelstunden-Raster aufgezogene Spanne für eine neue Schicht. */
  initialTimes?: { startTime: string; endTime: string };
}

function DienstplanContent() {
  const { data: session, status: sessionStatus } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const { params, updateParams: updateUrlParams } =
    useUrlParams(ALLOWED_URL_PARAMS);
  // Schichten schreibt das Backend mit time_tracking:manage (Admins halten es
  // über das Wildcard-Recht). Am Rollennamen `admin` festgemacht, kam eine
  // Leitungsrolle, die die Schule selbst anlegt, nie in den Dienstplan (#3469).
  const canEdit = hasPermission(session, "time_tracking:manage");
  // Die Halbjahres-Sicht zeigt Soll-/Plan-Abgleiche aus /overview. Dieser
  // Endpunkt verlangt zusätzlich `schedules:read`; Tab und Deep-Link sind
  // darum weiterhin auf beide Berechtigungen gegated, auch wenn die
  // Zeitraumsliste für Schichtserien ebenfalls im reduzierten Pfad lesbar ist.
  const canViewHalbjahr =
    hasPermission(session, "time_tracking:manage") &&
    hasPermission(session, "schedules:read");
  // POST /api/staff-shifts/export verlangt dieselbe Dreierkombination wie
  // /overview — der Ausdruck nennt Namen und liest die Schichtprojektion. Ohne
  // `users:read` liefe der Dialog in ein 403, darum wird der Knopf gar nicht
  // erst angeboten (gleiche Schranke wie im Betreuungsplan).
  const canExportPlan =
    hasPermission(session, "time_tracking:manage") &&
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "users:read");
  const canExportInternal = hasPermission(session, "schedules:manage");
  const today = useBerlinToday();

  // URL-State: der angezeigte Tag und die Ansicht werden bei jedem Render aus
  // den Search-Params abgeleitet (kein weekAnchor-useState mehr). Ein ungültiges
  // `d` (?d=foo oder ?d=2026-02-31) fällt auf heute zurück, ein unbekanntes
  // `view` auf "woche" — still, kein Fehlerzustand.
  const rawDay = params.d;
  const dayISO = rawDay !== null && isValidISODate(rawDay) ? rawDay : today;
  const rawView = params.view;
  let view: DienstplanView = "woche";
  if (rawView === "halbjahr" && canViewHalbjahr) view = "halbjahr";
  else if (rawView === "person" && canViewHalbjahr) view = "person";

  const [modal, setModal] = useState<ModalState | null>(null);
  const [exportOpen, setExportOpen] = useState(false);
  const [sickModal, setSickModal] = useState<StaffScheduleStaff | null>(null);
  // Anlegen eines Kalenderzeitraums; bearbeitet wird auf /calendar-periods.
  const [periodModalOpen, setPeriodModalOpen] = useState(false);

  // Montag der Woche, die `d` enthält.
  const weekAnchor = useMemo(() => startOfWeek(parseISODate(dayISO)), [dayISO]);

  // Kalenderzeiträume für die Zeitraum-Anzeige in der Kontextzeile (#1946).
  // Gleicher SWR-Key wie der Betreuungsplan, damit beide Ansichten denselben
  // Cache teilen. Der Endpoint verlangt schedules:read; die Ansicht selbst
  // gehört den Schichtplanenden, darum ist der Key zusätzlich auf canEdit
  // gegated — sonst feuerte der Request bei Direktaufruf ohne das Recht noch
  // vor dem Redirect und liefe in einen 403.
  const {
    data: periods,
    error: periodsError,
    isLoading: periodsLoading,
    mutate: mutatePeriods,
  } = useSWRAuth(
    sessionStatus === "authenticated" && canEdit
      ? "database-calendar-periods-list"
      : null,
    () => calendarPeriodService.list(),
  );

  // Route-Gate wie Betreuungsplan/Vertretung: Settings-Schema ->
  // timetable.enabled. fetchSettingsSchema liefert null, wenn der Nutzer keine
  // Settings lesen darf; die Seite rendert dann normal (gleiche Graceful-
  // Default-Logik wie die Sidebar).
  const { data: settingsSchema, isLoading: settingsSchemaLoading } =
    useSettingsSchema(sessionStatus === "authenticated", {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    });
  const timetableDisabled =
    getSettingValue(settingsSchema, "timetable.enabled") === false;

  // Mo–Fr als Date-Objekte für den PeriodSwitcherDropdown.
  const weekDayDates = useMemo(
    () =>
      Array.from({ length: 5 }, (_, i) => {
        const d = new Date(weekAnchor);
        d.setDate(d.getDate() + i);
        return d;
      }),
    [weekAnchor],
  );

  // Dieselben fünf Tage als ISO-Strings für Grid und Datenabruf.
  const weekDays = useMemo(() => weekDayDates.map(toISODate), [weekDayDates]);

  const weekFrom = weekDays[0] ?? "";
  const weekTo = weekDays[4] ?? "";

  const {
    canManageAbsences,
    sortedStaff,
    shiftsByStaff,
    assignmentsByStaff,
    summaryByStaff,
    typesById,
    allShifts,
    shiftTypes,
    shiftTypesError,
    scheduleError,
    scheduleLoading,
    retryLoad,
    reducedPath,
    refreshPlanCaches,
    mutateShiftTypes,
  } = useDienstplanData(weekFrom, weekTo);

  // Ladefehler stehen dort, wo die Daten fehlen (#2514): der Wochenplan als
  // Fehler des Gerüsts, Schichtarten und Kalenderzeiträume über dem Raster.
  const scheduleLoad = useApiLoadError();
  const shiftTypesLoad = useApiLoadError();
  const periodsLoad = useApiLoadError();
  const { show: showScheduleLoadError, clear: clearScheduleLoadError } =
    scheduleLoad;
  const { show: showShiftTypesLoadError, clear: clearShiftTypesLoadError } =
    shiftTypesLoad;
  const { show: showPeriodsLoadError, clear: clearPeriodsLoadError } =
    periodsLoad;
  useEffect(() => {
    if (scheduleError) {
      void showScheduleLoadError(scheduleError, {
        object: "die Dienstplanung",
        retry: retryLoad,
      });
    } else {
      clearScheduleLoadError();
    }
  }, [scheduleError, retryLoad, showScheduleLoadError, clearScheduleLoadError]);
  useEffect(() => {
    if (shiftTypesError) {
      void showShiftTypesLoadError(shiftTypesError, {
        object: "die Liste der Schichtarten",
        retry: () => void mutateShiftTypes(),
      });
    } else {
      clearShiftTypesLoadError();
    }
  }, [
    shiftTypesError,
    mutateShiftTypes,
    showShiftTypesLoadError,
    clearShiftTypesLoadError,
  ]);
  useEffect(() => {
    if (periodsError) {
      void showPeriodsLoadError(periodsError, {
        object: "die Liste der Kalenderzeiträume",
        retry: () => void mutatePeriods(),
      });
    } else {
      clearPeriodsLoadError();
    }
  }, [
    periodsError,
    mutatePeriods,
    showPeriodsLoadError,
    clearPeriodsLoadError,
  ]);

  // OGS-Schließtage (#2032): die Woche markiert ihre fünf Tage, der
  // Verschieben-Dialog prüft seinen frei wählbaren Zieltag gegen alle
  // gespeicherten Zeiträume. Die Halbjahres-Sicht lädt ihr eigenes Fenster.
  const {
    closingDays,
    closingDayRanges,
    isLoading: closingDaysLoading,
  } = useClosingDaysState(weekFrom, weekTo);

  const refreshAfterPlanMutation = useCallback(() => {
    // Bewusst still: die Änderung ist gespeichert; ein misslungenes
    // Nachladen zeigt den alten Stand, bis SWR beim nächsten Fokus neu lädt.
    refreshPlanCaches().catch((err: unknown) => {
      logger.error("post_plan_mutation_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    });
  }, [refreshPlanCaches]);

  // Own staff identity for the "Zeiterfassung öffnen" row-header target (own
  // person → /time-tracking, others → the staff detail page). Reuses the
  // time-tracking page's SWR key so the request is deduped; null when the
  // account is not linked to a staff person.
  const { data: ownStaff } = useSWRAuth(
    "time-tracking-own-staff",
    () => userContextService.getCurrentStaff(),
    { revalidateOnFocus: false },
  );
  const currentStaffId = ownStaff?.id ?? null;

  // Person der Ansicht "person": aus `staff`, sonst die eigene Person, sonst
  // die erste in der Liste.
  const personMember =
    sortedStaff.find((member) => member.id === params.staff) ??
    sortedStaff.find((member) => member.id === currentStaffId) ??
    sortedStaff[0] ??
    null;
  const personShiftCount = personMember
    ? [...(shiftsByStaff.get(personMember.id)?.values() ?? [])].reduce(
        (count, shifts) => count + shifts.length,
        0,
      )
    : 0;

  const isOnCurrentWeek =
    toISODate(startOfWeek(parseISODate(today))) === toISODate(weekAnchor);

  // Dasselbe Wochenetikett wie in Vertretung und Betreuungsplan
  // ("KW 31 · 27.07.–31.07.2026"). Der Dienstplan schrieb es früher als
  // "KW 31: 27. Juli bis 31. Juli 2026" selbst zusammen: drei benachbarte
  // Flächen mit drei Schreibweisen für dieselbe Woche, und auf einem Telefon
  // 220px Text in einer 199px breiten Kopfzeile, also abgeschnitten.
  const weekLabel = useMemo(() => {
    const end = new Date(weekAnchor);
    end.setDate(end.getDate() + 4);
    return formatWeekLabel(weekAnchor, end);
  }, [weekAnchor]);

  // Wochen-Navigation schreibt den Montag der Zielwoche nach `d`.
  const goToWeek = useCallback(
    (deltaDays: number) => {
      const target = new Date(weekAnchor);
      target.setDate(target.getDate() + deltaDays);
      updateUrlParams({ d: toISODate(target) });
    },
    [weekAnchor, updateUrlParams],
  );

  const goToToday = useCallback(
    () => updateUrlParams({ d: today }),
    [today, updateUrlParams],
  );

  // "woche" ist der Default und wird als Param-Entfernung geschrieben, damit die
  // URL sauber bleibt; Deep-Links funktionieren in beide Richtungen.
  const setView = useCallback(
    (next: DienstplanView) =>
      updateUrlParams({
        view: next === "woche" ? null : next,
        staff: next === "person" ? (personMember?.id ?? null) : null,
      }),
    [updateUrlParams, personMember],
  );

  if (sessionStatus !== "loading" && !canEdit) {
    redirect(tenantPath("/staff"));
  }

  // Solange die Session oder das Settings-Schema lädt, ist noch nicht
  // entscheidbar, ob das Feature aktiv ist und welche Aktionen erlaubt sind.
  // Die PlanningContextBar (Titel, Wochen-Navigation) rendert trotzdem sofort
  // — nur der Inhaltsbereich unten fällt auf das Grid-Skeleton zurück, und
  // datenabhängige Toolbar-Teile (Ansichts-Umschalter, Export-Knopf) bleiben
  // aus, bis die Berechtigungen feststehen.
  const showSkeleton = sessionStatus === "loading" || settingsSchemaLoading;

  if (!showSkeleton && timetableDisabled) {
    return <DienstplanDisabledState />;
  }

  // Leerzustand "keine Mitarbeitenden" (docs/05 Abschnitt 4) — geteilt zwischen
  // Wochen- und Halbjahres-Zweig, damit ohne Staff beide Ansichten denselben
  // Hinweis statt eines Rasters zeigen. Er kommt aus dem Gerüst (`empty`),
  // nicht aus einer eigenen Karte im Inhalt.
  const noStaffEmpty = {
    title: "Noch keine Mitarbeitenden angelegt",
    description:
      "Sobald Mitarbeitende angelegt sind, erscheinen sie hier als Zeilen im Dienstplan.",
    action: (
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => router.push("/staff")}
      >
        Zu den Mitarbeitenden
      </Button>
    ),
  };

  // Laden, Fehler und Leerzustand gehören dem Gerüst (Bauart 3 Regel 5): die
  // Kopfkarte mit der Zeitnavigation bleibt stehen, der Inhaltsbereich wird
  // ersetzt. Ein fehlgeschlagener Staff-/Overview-Load ist `error`, nie
  // `empty` — auch nicht in der Halbjahres-Sicht.
  const pageLoading =
    showSkeleton ||
    scheduleLoading ||
    (scheduleError !== undefined && scheduleLoad.error === null);
  // Bis der Katalogtext geladen ist, hält `pageLoading` das Raster zurück:
  // ohne Daten darf nichts bearbeitbar wirken.
  const pageError = scheduleError ? scheduleLoad.error : null;
  const pageEmpty =
    !pageError && !pageLoading && sortedStaff.length === 0
      ? noStaffEmpty
      : null;

  // Der Inhalt wird nur gerendert, wenn das Gerüst ihn durchlässt — bei
  // Laden, Fehler oder Leerzustand ersetzt `TenantPage` ihn vollständig.
  let content: ReactNode;
  if (view === "halbjahr") {
    // Halbjahres-Sicht (Personen × Kalenderwochen, docs/05 Abschnitt 3). Der
    // Leerzustand "Kein Planungszeitraum" lebt in der Grid-Komponente selbst.
    content = (
      <DienstplanHalbjahrGrid
        dayISO={dayISO}
        staff={sortedStaff}
        reducedPath={reducedPath}
        todayIso={today}
        closingDayRanges={closingDayRanges}
        onWeekClick={(monday) => updateUrlParams({ d: monday, view: null })}
      />
    );
  } else if (view === "person" && personMember) {
    content = (
      <div className="space-y-3">
        <LoadErrorAlert error={shiftTypesLoad.error} />
        <DienstplanPersonWeekGrid
          member={personMember}
          shiftsByDate={shiftsByStaff.get(personMember.id)}
          weekDays={weekDays}
          todayIso={today}
          closingDays={closingDays}
          closingDaysLoading={closingDaysLoading}
          typesById={typesById}
          shiftTypes={shiftTypes ?? []}
          summary={summaryByStaff.get(personMember.id)}
          onCreate={(date, startTime, endTime) =>
            setModal({
              mode: "create",
              staff: personMember,
              date,
              shift: null,
              replacements: [],
              initialTimes: { startTime, endTime },
            })
          }
          onEdit={(date, shift) =>
            setModal({
              mode: "edit",
              staff: personMember,
              date,
              shift,
              replacements: allShifts.filter(
                (s) => s.originShiftId === shift.id,
              ),
            })
          }
        />
      </div>
    );
  } else {
    content = (
      // Kein zusätzlicher Kartenrahmen um das Raster (#2031) — die ResourceGrid
      // bringt ihre Fläche selbst mit, wie das Wochenraster im Betreuungsplan.
      <div className="space-y-3">
        <LoadErrorAlert error={shiftTypesLoad.error} />
        {allShifts.length === 0 && (
          // Leerzustand: Mitarbeitende vorhanden, aber keine Schichten in der
          // Woche. Als Hinweis aus dem Kit, nicht als freier Satz über dem
          // Raster — Meldungen sind im Portal überall Alerts.
          <Alert
            type="info"
            message="In dieser Woche sind keine Schichten geplant."
          />
        )}
        <DienstplanResourceGrid
          staff={sortedStaff}
          shiftsByStaff={shiftsByStaff}
          assignmentsByStaff={assignmentsByStaff}
          summaryByStaff={summaryByStaff}
          weekDays={weekDays}
          todayIso={today}
          closingDays={closingDays}
          closingDaysLoading={closingDaysLoading}
          closingDayRanges={closingDayRanges}
          typesById={typesById}
          shiftTypes={shiftTypes ?? []}
          reducedPath={reducedPath}
          currentStaffId={currentStaffId}
          onCellClick={(member, date, shift) =>
            setModal({
              mode: shift ? "edit" : "create",
              staff: member,
              date,
              shift,
              replacements: shift
                ? allShifts.filter((s) => s.originShiftId === shift.id)
                : [],
            })
          }
          onSickReport={
            canManageAbsences ? (member) => setSickModal(member) : undefined
          }
          onOpenPersonWeek={
            canViewHalbjahr
              ? (member) =>
                  updateUrlParams({ view: "person", staff: member.id })
              : undefined
          }
        />
        {/* Stunden je Schichtart (#3819): dieselben Wochensummen wie im
            Zeilenkopf, aufgeteilt. Im reduzierten Pfad gibt es keine. */}
        {!reducedPath && (
          <DienstplanHoursCard
            staff={sortedStaff}
            summaryByStaff={summaryByStaff}
            shiftTypes={shiftTypes ?? []}
          />
        )}
      </div>
    );
  }

  // Statuszeile der Kopfkarte: der angezeigte Zeitraum und die Zahlen, die
  // die Fläche ohnehin geladen hat. In der Halbjahres-Sicht steht die
  // Wochenzahl nicht, weil dort keine einzelne Woche zu sehen ist.
  // Kein Zeitraum in der Statuszeile: den trägt das Bedienband direkt
  // darunter, mit Pfeilen. Zweimal dieselbe Woche in der Kopfkarte kostete
  // auf dem Telefon eine Zeile, die nichts sagte.
  const displayedShiftCount =
    view === "person" ? personShiftCount : allShifts.length;
  const statusLine = [
    view !== "halbjahr"
      ? `${displayedShiftCount} ${displayedShiftCount === 1 ? "Dienst" : "Dienste"}`
      : null,
    `${sortedStaff.length} ${sortedStaff.length === 1 ? "Person" : "Personen"}`,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <TenantPage
      title="Dienstplan"
      stats={statusLine}
      statsLoading={pageLoading}
      // Struktur-Skelett statt der generischen Karten: ein Wochenraster lädt
      // nicht wie eine Kartenliste.
      loading={pageLoading ? <DienstplanGridSkeleton /> : false}
      error={pageError}
      empty={pageEmpty}
      actions={
        // Unter sm gibt das Gerüst jeder Textaktion eine volle Zeile; ein
        // Knopf ohne Beschriftung wäre dort ein schwarzer Balken mit einem
        // einsamen Symbol. Deshalb trägt der Knopf sein Label auf jeder
        // Breite.
        <>
          {/* Die Schichtarten liegen in der Datenverwaltung (#3114); eine
              Kopf-Aktion hat keinen Entwurf zu verlieren und führt deshalb im
              selben Fenster dorthin. */}
          <ButtonLink
            href={tenantPath("/database/shift-types")}
            variant="primary"
            size="md"
          >
            <Settings2 className="mr-1.5 h-4 w-4 shrink-0" aria-hidden />
            <span className="whitespace-nowrap">Schichtarten verwalten</span>
          </ButtonLink>
          {/* Drucken/Exportieren (#2079) meint immer die Woche, die gerade auf
              dem Bildschirm steht -- deshalb hier und nicht auf der zentralen
              Exportseite. Im Menü, weil neben dem Titel eine sichtbare Aktion
              steht und nicht zwei. */}
          {canExportPlan && (
            <OverflowMenu
              items={[
                {
                  label: "Drucken oder exportieren",
                  icon: <Printer className="h-4 w-4" aria-hidden />,
                  onClick: () => setExportOpen(true),
                },
              ]}
              ariaLabel="Weitere Aktionen"
            />
          )}
        </>
      }
      searchSlot={
        <PlanningContextBar
          onPrevious={() => goToWeek(-7)}
          onNext={() => goToWeek(7)}
          previousLabel="Vorherige Woche"
          nextLabel="Nächste Woche"
          dateLabel={weekLabel}
          onToday={isOnCurrentWeek ? undefined : goToToday}
          viewSwitcher={
            // Ohne schedules:read gibt es nur die Wochenansicht — ein
            // Ein-Tab-Umschalter wäre sinnlos, also entfällt er ganz. Die
            // Ansicht „Person“ (#3818) teilt diese Schranke: sie zeigt das
            // Soll aus /overview wie die Halbjahres-Sicht.
            canViewHalbjahr ? (
              <SegmentedControl
                ariaLabel="Ansicht"
                value={view}
                onChange={(next) => setView(next as DienstplanView)}
                items={[
                  { value: "woche", label: "Woche" },
                  { value: "person", label: "Person" },
                  { value: "halbjahr", label: "Halbjahr" },
                ]}
              />
            ) : undefined
          }
        >
          {view === "person" && sortedStaff.length > 0 && (
            <CustomSelect
              value={personMember?.id ?? ""}
              options={sortedStaff.map((member) => ({
                value: member.id,
                label: `${member.lastName}, ${member.firstName}`,
              }))}
              onChange={(id) => updateUrlParams({ staff: id })}
              ariaLabel="Person"
              className="w-full sm:w-64"
            />
          )}
          {/* Zeitraum-Anzeige (#1946): gleicher Switcher wie im Betreuungsplan,
              damit der aktive Kalenderzeitraum an einer einheitlichen Stelle
              sichtbar, wechselbar und verwaltbar ist. */}
          <PeriodSwitcherDropdown
            periods={periods ?? []}
            weekDays={weekDayDates}
            isLoading={showSkeleton || periodsLoading}
            onCreate={() => setPeriodModalOpen(true)}
            onSelect={(period) =>
              updateUrlParams({
                d: firstSchoolDayInPeriod(
                  period.startDate,
                  period.endDate,
                  period.startDate,
                ),
              })
            }
          />
        </PlanningContextBar>
      }
    >
      <LoadErrorAlert error={periodsLoad.error} className="mb-3" />
      {content}

      {modal && (
        <ShiftEditModal
          isOpen
          mode={modal.mode}
          staffId={modal.staff.id}
          staffName={`${modal.staff.firstName} ${modal.staff.lastName}`}
          date={modal.date}
          shift={modal.shift}
          shiftTypes={shiftTypes ?? []}
          staffOptions={sortedStaff}
          existingReplacements={modal.replacements}
          initialStartTime={modal.initialTimes?.startTime}
          initialEndTime={modal.initialTimes?.endTime}
          onClose={() => setModal(null)}
          onSaved={refreshAfterPlanMutation}
        />
      )}
      {periodModalOpen && (
        <CalendarPeriodModal
          isOpen
          initial={null}
          onClose={() => setPeriodModalOpen(false)}
          onSaved={() => {
            setPeriodModalOpen(false);
            void mutatePeriods();
          }}
          onDeleted={() => {
            setPeriodModalOpen(false);
            void mutatePeriods();
          }}
        />
      )}
      {/* Erst bei Bedarf gemountet, wie die übrigen Dialoge dieser Fläche:
          ein dauerhaft eingehängter Dialog zieht seinen Kontext (Toasts) auch
          dann in jeden Test dieser Seite, wenn ihn niemand öffnet. */}
      {canExportPlan && exportOpen && (
        <PlanExportModal
          isOpen
          plan="dienstplan"
          weekDay={dayISO}
          isWeekOnScreen={view !== "halbjahr"}
          canExportInternal={canExportInternal}
          onClose={() => setExportOpen(false)}
        />
      )}
      {sickModal && (
        <SickReportModal
          isOpen
          staff={sickModal}
          onClose={() => setSickModal(null)}
          onCreated={() => {
            // Bewusst still wie oben: gespeichert ist die Krankmeldung schon.
            // refreshPlanCaches invalidiert per Präfix "dienstplan-overview-"
            // bereits den Overview-Key mit — ein separater Overview-Mutate wäre
            // redundant.
            refreshPlanCaches().catch((err: unknown) => {
              logger.error("post_sick_report_refresh_failed", {
                error: err instanceof Error ? err.message : String(err),
              });
            });
          }}
        />
      )}
    </TenantPage>
  );
}

function DienstplanDisabledState() {
  return (
    <PlanningDisabledState
      pageTitle="Dienstplan"
      heading="Dienstplan ist deaktiviert"
      description="Der Dienstplan gehört zum Planungsbereich, der für diese Schule ausgeschaltet ist. Er kann in den Einstellungen unter „Betrieb“ wieder aktiviert werden."
      testId="dienstplan-disabled-state"
    />
  );
}

// DienstplanView is the embeddable Dienstplan surface, hosted by /dienstplan
// (Planung-Redesign, docs/05); formerly the /staff/dienstplan page.
export function DienstplanView() {
  return <DienstplanContent />;
}
