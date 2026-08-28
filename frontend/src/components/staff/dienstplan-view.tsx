"use client";

import { useCallback, useMemo, useState, type ReactNode } from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";

import { Printer, Settings2 } from "lucide-react";

import { PlanExportModal } from "~/components/planning/plan-export-modal";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { PeriodSwitcherDropdown } from "~/components/timetable/period-switcher-dropdown";
import { DienstplanHalbjahrGrid } from "~/components/staff/dienstplan-halbjahr-grid";
import { DienstplanResourceGrid } from "~/components/staff/dienstplan-resource-grid";
import {
  ShiftEditModal,
  type ShiftEditMode,
} from "~/components/staff/shift-edit-modal";
import { ShiftTypeManageModal } from "~/components/staff/shift-type-manage-modal";
import { SickReportModal } from "~/components/staff/sick-report-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { TenantPage } from "~/components/ui/tenant-page";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
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

import { DienstplanGridSkeleton } from "./dienstplan-skeleton";

const logger = createLogger({ component: "DienstplanView" });

// Admin week view for planned staff shifts (Dienstplan, #1376 core slice).
// One row per staff member, Mo–Fr columns, click-to-edit per cell. The
// planned shift end also drives the automatic checkout (#1798) when the
// tenant setting "Automatische Ausstempelung" is enabled.
//
// URL-Vokabular (Planung-Redesign, docs/05-dienstplan.md Abschnitt 1): genau
// `d` (Berlin-Kalendertag; die angezeigte Woche ist die Woche, die `d` enthält)
// und `view` ("woche" | "halbjahr"). Ungültige Werte fallen still auf die
// Defaults zurück (heute, "woche"). Modals bleiben reiner React-State.

type DienstplanView = "woche" | "halbjahr";

// updateUrlParams baut die URL aus dieser Allowlist neu auf, damit fremde
// Params (?utm_source=…) nicht jeden Wochen-/Ansichtswechsel überleben.
const ALLOWED_URL_PARAMS = ["d", "view"] as const;

interface ModalState {
  mode: ShiftEditMode;
  staff: StaffScheduleStaff;
  date: string;
  shift: StaffShift | null;
  // The covers attached to `shift` (rows whose originShiftId is shift.id),
  // captured at open time so a background SWR revalidation cannot swap them out
  // from under an in-progress edit — same freeze as `shift` itself (#1841).
  replacements: readonly StaffShift[];
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
  const canEdit = isAdmin(session);
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
  const view: DienstplanView =
    rawView === "halbjahr" && canViewHalbjahr ? "halbjahr" : "woche";

  const [modal, setModal] = useState<ModalState | null>(null);
  const [manageOpen, setManageOpen] = useState(false);
  const [exportOpen, setExportOpen] = useState(false);
  const [sickModal, setSickModal] = useState<StaffScheduleStaff | null>(null);
  const [periodModalOpen, setPeriodModalOpen] = useState(false);
  const [editingPeriod, setEditingPeriod] = useState<CalendarPeriod | null>(
    null,
  );

  // Montag der Woche, die `d` enthält.
  const weekAnchor = useMemo(() => startOfWeek(parseISODate(dayISO)), [dayISO]);

  // Kalenderzeiträume für die Zeitraum-Anzeige in der Kontextzeile (#1946).
  // Gleicher SWR-Key wie der Betreuungsplan, damit beide Ansichten denselben
  // Cache teilen. Der Endpoint verlangt schedules:read; die Ansicht selbst
  // ist admin-only, darum ist der Key zusätzlich auf canEdit gegated — sonst
  // feuerte der Request bei Direktaufruf durch Nicht-Admins noch vor dem
  // Redirect und liefe in einen 403.
  const {
    data: periods,
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
    shiftTypesLoading,
    mutateShiftTypes,
    scheduleError,
    scheduleLoading,
    retryLoad,
    reducedPath,
    refreshPlanCaches,
  } = useDienstplanData(weekFrom, weekTo);

  // OGS-Schließtage (#2032): die Woche markiert ihre fünf Tage, der
  // Verschieben-Dialog prüft seinen frei wählbaren Zieltag gegen alle
  // gespeicherten Zeiträume. Die Halbjahres-Sicht lädt ihr eigenes Fenster.
  const {
    closingDays,
    closingDayRanges,
    isLoading: closingDaysLoading,
  } = useClosingDaysState(weekFrom, weekTo);

  const refreshAfterPlanMutation = useCallback(() => {
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
      updateUrlParams({ view: next === "woche" ? null : next }),
    [updateUrlParams],
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
  // Hinweis statt eines Rasters zeigen. Kein Artwork, kein Marketing-Ton.
  const noStaffEmptyState = (
    <EmptyState
      title="Noch keine Mitarbeitenden angelegt"
      description="Sobald Mitarbeitende angelegt sind, erscheinen sie hier als Zeilen im Dienstplan."
      action={
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => router.push("/staff")}
        >
          Zu den Mitarbeitenden
        </Button>
      }
    />
  );

  // Inhaltsbereich als Zustandskaskade, damit die PlanningContextBar (oben)
  // IMMER sichtbar bleibt — auch beim Daten-Laden und im Fehlerfall. Reihenfolge:
  // Fehler → Laden → keine Mitarbeitenden → Ansicht. Fehler- und Ladezustand
  // gelten für BEIDE Ansichten (K1/K2): ein fehlgeschlagener Staff-/Overview-Load
  // darf im Halbjahr nicht mehr als Leerzustand erscheinen, und beim Laden zeigt
  // der Inhaltsbereich das Grid-Skeleton statt die ganze Seite zu verwerfen.
  let content: ReactNode;
  if (scheduleError) {
    content = (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="space-y-3">
          <Alert
            type="error"
            message="Der Dienstplan konnte nicht vollständig geladen werden. Bearbeiten ist deaktiviert, bis die Daten erfolgreich geladen wurden."
          />
          <Button type="button" variant="outline" size="md" onClick={retryLoad}>
            Erneut laden
          </Button>
        </div>
      </div>
    );
  } else if (showSkeleton || scheduleLoading) {
    content = <DienstplanGridSkeleton />;
  } else if (sortedStaff.length === 0) {
    content = (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        {noStaffEmptyState}
      </div>
    );
  } else if (view === "halbjahr") {
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
  } else {
    content = (
      // Kein zusätzlicher Kartenrahmen um das Raster (#2031) — die ResourceGrid
      // bringt ihre Fläche selbst mit, wie das Wochenraster im Betreuungsplan.
      <div className="space-y-3">
        {shiftTypesError && (
          <Alert
            type="warning"
            message="Schichtarten konnten nicht geladen werden. Der Dienstplan wird mit neutralen Schichtfarben angezeigt; die Schichtartenverwaltung ist vorübergehend deaktiviert."
          />
        )}
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
        />
      </div>
    );
  }

  // Statuszeile der Kopfkarte: der angezeigte Zeitraum und die Zahlen, die
  // die Fläche ohnehin geladen hat. In der Halbjahres-Sicht steht die
  // Wochenzahl nicht, weil dort keine einzelne Woche zu sehen ist.
  const statusLine = [
    weekLabel,
    view === "woche"
      ? `${allShifts.length} ${allShifts.length === 1 ? "Dienst" : "Dienste"}`
      : null,
    `${sortedStaff.length} ${sortedStaff.length === 1 ? "Person" : "Personen"}`,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <TenantPage
      title="Dienstplan"
      stats={statusLine}
      statsLoading={showSkeleton || scheduleLoading}
      actions={
        // Unter sm nur die Symbole: der volle Text brach in der Kopfzeile
        // zweizeilig um. EIN Button je Aktion mit ausgeblendetem Label statt
        // zweier Breakpoint-Varianten — das `aria-label` hält sie für
        // Screenreader und Tests unter demselben Namen auffindbar.
        <>
          <Button
            type="button"
            variant="primary"
            size="md"
            aria-label="Schichtarten verwalten"
            className="max-sm:h-8 max-sm:w-8 max-sm:justify-center max-sm:p-0"
            onClick={() => setManageOpen(true)}
            disabled={Boolean(shiftTypesError)}
          >
            <Settings2 className="h-4 w-4 shrink-0 sm:mr-1.5" aria-hidden />
            <span className="hidden whitespace-nowrap sm:inline">
              Schichtarten verwalten
            </span>
          </Button>
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
            // Ein-Tab-Umschalter wäre sinnlos, also entfällt er ganz.
            canViewHalbjahr ? (
              <SegmentedControl
                ariaLabel="Ansicht"
                value={view}
                onChange={(next) => setView(next as DienstplanView)}
                items={[
                  { value: "woche", label: "Woche" },
                  { value: "halbjahr", label: "Halbjahr" },
                ]}
              />
            ) : undefined
          }
        >
          {/* Zeitraum-Anzeige (#1946): gleicher Switcher wie im Betreuungsplan,
              damit der aktive Kalenderzeitraum an einer einheitlichen Stelle
              sichtbar, wechselbar und verwaltbar ist. */}
          <PeriodSwitcherDropdown
            periods={periods ?? []}
            weekDays={weekDayDates}
            isLoading={showSkeleton || periodsLoading}
            onCreate={() => {
              setEditingPeriod(null);
              setPeriodModalOpen(true);
            }}
            onEdit={(period) => {
              setEditingPeriod(period);
              setPeriodModalOpen(true);
            }}
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
          closingDayRanges={closingDayRanges}
          onClose={() => setModal(null)}
          onSaved={refreshAfterPlanMutation}
        />
      )}
      {periodModalOpen && (
        <CalendarPeriodModal
          isOpen
          initial={editingPeriod}
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
          isWeekOnScreen={view === "woche"}
          canExportInternal={canExportInternal}
          onClose={() => setExportOpen(false)}
        />
      )}
      <ShiftTypeManageModal
        isOpen={manageOpen && !shiftTypesError}
        shiftTypes={shiftTypes ?? []}
        isLoading={shiftTypesLoading}
        loadError={Boolean(shiftTypesError)}
        onClose={() => setManageOpen(false)}
        onChanged={() => {
          mutateShiftTypes();
          refreshAfterPlanMutation();
        }}
      />
      {sickModal && (
        <SickReportModal
          isOpen
          staff={sickModal}
          onClose={() => setSickModal(null)}
          onCreated={() => {
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
