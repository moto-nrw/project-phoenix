"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { redirect, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";

import { Settings2 } from "lucide-react";

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
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { isValidISODate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useDienstplanData } from "~/lib/hooks/use-dienstplan-data";
import { createLogger } from "~/lib/logger";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import { startOfWeek } from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { getWeekNumber } from "~/lib/time-tracking-helpers";
import { userContextService } from "~/lib/usercontext-api";

import {
  DienstplanGridSkeleton,
  DienstplanPageSkeleton,
} from "./dienstplan-skeleton";

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
  const searchParams = useSearchParams();
  const canEdit = isAdmin(session);
  // Die Halbjahres-Sicht lädt die Planungszeiträume (/api/timetable/periods),
  // die das Backend mit `schedules:read` schützt — ohne die Berechtigung liefe
  // jeder Öffnungsversuch in einen 403. Tab und Deep-Link sind darum gegated.
  const canViewHalbjahr = hasPermission(session, "schedules:read");
  const today = useBerlinToday();

  // URL-State: der angezeigte Tag und die Ansicht werden bei jedem Render aus
  // den Search-Params abgeleitet (kein weekAnchor-useState mehr). Ein ungültiges
  // `d` (?d=foo oder ?d=2026-02-31) fällt auf heute zurück, ein unbekanntes
  // `view` auf "woche" — still, kein Fehlerzustand.
  const rawDay = searchParams.get("d");
  const dayISO = rawDay !== null && isValidISODate(rawDay) ? rawDay : today;
  const rawView = searchParams.get("view");
  const view: DienstplanView =
    rawView === "halbjahr" && canViewHalbjahr ? "halbjahr" : "woche";

  const [modal, setModal] = useState<ModalState | null>(null);
  const [manageOpen, setManageOpen] = useState(false);
  const [sickModal, setSickModal] = useState<StaffScheduleStaff | null>(null);

  // Montag der Woche, die `d` enthält.
  const weekAnchor = useMemo(() => startOfWeek(parseISODate(dayISO)), [dayISO]);

  const weekDays = useMemo(() => {
    return Array.from({ length: 5 }, (_, i) => {
      const d = new Date(weekAnchor);
      d.setDate(d.getDate() + i);
      return toISODate(d);
    });
  }, [weekAnchor]);

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
    mutateScheduleData,
    reducedPath,
    refreshPlanCaches,
  } = useDienstplanData(weekFrom, weekTo);

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

  const weekLabel = useMemo(() => {
    const start = new Date(weekAnchor);
    const end = new Date(weekAnchor);
    end.setDate(end.getDate() + 4);
    const startLabel = start.toLocaleDateString("de-DE", {
      day: "numeric",
      month: "short",
    });
    const endLabel = end.toLocaleDateString("de-DE", {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
    return `KW ${getWeekNumber(weekAnchor)}: ${startLabel} bis ${endLabel}`;
  }, [weekAnchor]);

  // Rebuild the URL from the allowlist (mirrors vertretung-view): a bare
  // window.history.replaceState, no router.replace, so a week/view change never
  // triggers a server roundtrip.
  const updateUrlParams = useCallback(
    (updates: Record<string, string | null>) => {
      const current = new URLSearchParams(window.location.search);
      const params = new URLSearchParams();
      for (const key of ALLOWED_URL_PARAMS) {
        const value = current.get(key);
        if (value !== null) params.set(key, value);
      }
      for (const [key, value] of Object.entries(updates)) {
        if (value === null) params.delete(key);
        else params.set(key, value);
      }
      const qs = params.toString();
      window.history.replaceState(
        null,
        "",
        qs ? `?${qs}` : window.location.pathname,
      );
    },
    [],
  );

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

  // Nicht-Admins landen auf /staff. Der Redirect ist ein Effekt, kein
  // Render-Nebeneffekt (Strict Mode rendert doppelt); während er läuft zeigt
  // der Render unten das Skeleton (Muster wie planung-redirect.tsx).
  useEffect(() => {
    if (sessionStatus !== "loading" && !canEdit) {
      router.replace("/staff");
    }
  }, [sessionStatus, canEdit, router]);

  if (sessionStatus === "loading" || !canEdit) {
    return <DienstplanPageSkeleton />;
  }

  // Leerzustand "keine Mitarbeitenden" (docs/05 Abschnitt 4) — geteilt zwischen
  // Wochen- und Halbjahres-Zweig, damit ohne Staff beide Ansichten denselben
  // Hinweis statt eines Rasters zeigen. Kein Artwork, kein Marketing-Ton.
  const noStaffEmptyState = (
    <div className="flex flex-col items-center gap-3 py-10 text-center">
      <p className="text-sm font-semibold text-gray-900">
        Noch keine Mitarbeitenden angelegt
      </p>
      <p className="max-w-md text-sm leading-relaxed text-gray-600">
        Sobald Mitarbeitende angelegt sind, erscheinen sie hier als Zeilen im
        Dienstplan.
      </p>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => router.push("/staff")}
      >
        Zu den Mitarbeitenden
      </Button>
    </div>
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
          <button
            type="button"
            onClick={retryLoad}
            className="rounded-md border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Erneut laden
          </button>
        </div>
      </div>
    );
  } else if (scheduleLoading) {
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
        onWeekClick={(monday) => updateUrlParams({ d: monday, view: null })}
      />
    );
  } else {
    content = (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        {shiftTypesError && (
          <Alert
            type="warning"
            message="Schichtarten konnten nicht geladen werden. Der Dienstplan wird mit neutralen Schichtfarben angezeigt; die Schichtartenverwaltung ist vorübergehend deaktiviert."
          />
        )}
        {allShifts.length === 0 && (
          // Leerzustand: Mitarbeitende vorhanden, aber keine Schichten in
          // der Woche (docs/05 Abschnitt 4) — dezente Hinweiszeile über dem
          // Raster, keine Alert-Box.
          <p className="mb-3 text-sm text-gray-500">
            In dieser Woche sind keine Schichten geplant.
          </p>
        )}
        <DienstplanResourceGrid
          staff={sortedStaff}
          shiftsByStaff={shiftsByStaff}
          assignmentsByStaff={assignmentsByStaff}
          summaryByStaff={summaryByStaff}
          weekDays={weekDays}
          todayIso={today}
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

  return (
    <div className="space-y-4">
      <PlanningContextBar
        title="Dienstplan"
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
            <Tabs
              value={view}
              onValueChange={(v) => setView(v as DienstplanView)}
            >
              <TabsList variant="default">
                <TabsTrigger value="woche">Woche</TabsTrigger>
                <TabsTrigger value="halbjahr">Halbjahr</TabsTrigger>
              </TabsList>
            </Tabs>
          ) : undefined
        }
        actions={
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => setManageOpen(true)}
            disabled={Boolean(shiftTypesError)}
          >
            <Settings2 className="mr-1.5 h-4 w-4" />
            Schichtarten verwalten
          </Button>
        }
      />

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
          onClose={() => setModal(null)}
          onSaved={() => mutateScheduleData()}
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
          mutateScheduleData();
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
    </div>
  );
}

// DienstplanView is the embeddable Dienstplan surface, hosted by /dienstplan
// (Planung-Redesign, docs/05); formerly the /staff/dienstplan page.
export function DienstplanView() {
  return <DienstplanContent />;
}
