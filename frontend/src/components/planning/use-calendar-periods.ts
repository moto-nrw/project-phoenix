"use client";

/**
 * Zustand des Kalenderzeitraum-Bereichs: Laden, Modal-Steuerung und die
 * Statuszeile der Seite.
 *
 * Der Hook liegt hier und nicht im Editor, weil die Seite den Kopf trägt
 * (Titel, Statuszeile, Aktionen) und der Editor nur noch seinen Inhaltsblock.
 * Beides greift auf denselben Zustand zu, ohne ihn zweimal zu laden.
 */

import { useCallback, useEffect, useMemo, useState } from "react";

import type { LinkablePhase } from "~/components/timetable/calendar-period-modal";
import { useToast } from "~/contexts/ToastContext";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { todayISO } from "~/lib/date-helpers";
import {
  type Phase,
  listPhases,
  setPhaseCalendarPeriod,
} from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CalendarPeriodsEditor" });

interface SemesterDefaults {
  name: string;
  periodType: "semester";
  startDate: string;
  endDate: string;
}

interface PeriodUsage {
  enrollmentPhaseCount: number;
  activityGroupCount: number;
  scheduleCount: number;
  studentEnrollmentCount: number;
  supervisorCount: number;
  activityInstanceCount: number;
}

export interface CalendarPeriodsState {
  readonly periods: CalendarPeriod[];
  readonly phases: Phase[];
  readonly loading: boolean;
  readonly error: string | null;
  /** Statuszeile der Kopfkarte: laufender Zeitraum, Anzahl, davon aktiv. */
  readonly statusLine: string;
  readonly modalOpen: boolean;
  readonly editing: CalendarPeriod | null;
  readonly createDefaults: Partial<SemesterDefaults> | undefined;
  readonly editingUsage: PeriodUsage | undefined;
  readonly beginCreate: () => void;
  readonly beginCreateSemester: () => void;
  readonly beginEdit: (period: CalendarPeriod) => void;
  readonly closeModal: () => void;
  readonly reload: (opts?: { silent?: boolean }) => Promise<void>;
  readonly handlePhaseLinkToggle: (
    phase: LinkablePhase,
    link: boolean,
  ) => Promise<void>;
}

/**
 * Schlägt das nächste Halbjahr ausgehend von heute vor. Deutsche Schulhalb-
 * jahre laufen 1.8.–31.1. (1. HJ) und 1.2.–31.7. (2. HJ); das sind reine
 * Vorbelegungen, die Daten passt die Administration im Dialog an.
 */
function nextSemesterDefaults(todayIso: string): SemesterDefaults {
  const year = Number(todayIso.slice(0, 4));
  const month = Number(todayIso.slice(5, 7));

  if (month >= 8) {
    // Im 1. Halbjahr → das 2. Halbjahr desselben Schuljahres vorschlagen.
    return {
      name: `2. Halbjahr ${year}/${String((year + 1) % 100).padStart(2, "0")}`,
      periodType: "semester",
      startDate: `${year + 1}-02-01`,
      endDate: `${year + 1}-07-31`,
    };
  }
  if (month === 1) {
    // Januar: noch 1. Halbjahr → 2. Halbjahr ab 1. Februar vorschlagen.
    return {
      name: `2. Halbjahr ${year - 1}/${String(year % 100).padStart(2, "0")}`,
      periodType: "semester",
      startDate: `${year}-02-01`,
      endDate: `${year}-07-31`,
    };
  }
  // Februar–Juli: im 2. Halbjahr → 1. Halbjahr des nächsten Schuljahres.
  return {
    name: `1. Halbjahr ${year}/${String((year + 1) % 100).padStart(2, "0")}`,
    periodType: "semester",
    startDate: `${year}-08-01`,
    endDate: `${year + 1}-01-31`,
  };
}

export function useCalendarPeriods(): CalendarPeriodsState {
  const [periods, setPeriods] = useState<CalendarPeriod[]>([]);
  const [phases, setPhases] = useState<Phase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<CalendarPeriod | null>(null);
  const [createDefaults, setCreateDefaults] =
    useState<Partial<SemesterDefaults>>();
  const { success: toastSuccess, error: toastError } = useToast();

  // silent: neu laden ohne den Ladezustand der ganzen Fläche — nach dem
  // Verknüpfen einer Anmeldephase, damit der offene Dialog stehen bleibt.
  const reload = useCallback(async (opts?: { silent?: boolean }) => {
    if (!opts?.silent) setLoading(true);
    setError(null);
    try {
      const [periodData, phaseData] = await Promise.all([
        calendarPeriodService.list(),
        // Die Phasen füllen den Verknüpfungsbereich des Dialogs. Ihr Fehler
        // darf die Zeitraumliste nicht mitreißen.
        listPhases().catch((err: unknown) => {
          logger.warn("calendar_periods_phases_load_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as Phase[];
        }),
      ]);
      setPeriods(
        [...periodData].sort((a, b) => a.startDate.localeCompare(b.startDate)),
      );
      setPhases(phaseData);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("calendar_periods_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const beginCreate = useCallback(() => {
    setEditing(null);
    setCreateDefaults(undefined);
    setModalOpen(true);
  }, []);

  const beginCreateSemester = useCallback(() => {
    setEditing(null);
    setCreateDefaults(nextSemesterDefaults(todayISO()));
    setModalOpen(true);
  }, []);

  const beginEdit = useCallback((period: CalendarPeriod) => {
    setEditing(period);
    setCreateDefaults(undefined);
    setModalOpen(true);
  }, []);

  const closeModal = useCallback(() => setModalOpen(false), []);

  const handlePhaseLinkToggle = useCallback(
    async (phase: LinkablePhase, link: boolean) => {
      const target = editing;
      const full = phases.find((p) => p.id === phase.id);
      if (!target || !full) return;
      try {
        await setPhaseCalendarPeriod(full, link ? target.id : null);
        toastSuccess(
          link
            ? `Anmeldephase "${full.name}" mit "${target.name}" verknüpft`
            : `Verknüpfung von "${full.name}" entfernt`,
        );
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : "Verknüpfung konnte nicht gespeichert werden";
        logger.error("phase_link_toggle_failed", {
          phase_id: phase.id,
          error: message,
        });
        toastError(message);
      } finally {
        await reload({ silent: true });
      }
    },
    [editing, phases, reload, toastSuccess, toastError],
  );

  const editingUsage = useMemo(() => {
    if (!editing) return undefined;
    const usageSource =
      periods.find((period) => period.id === editing.id) ?? editing;
    return {
      enrollmentPhaseCount: usageSource.enrollmentPhaseCount ?? 0,
      activityGroupCount: usageSource.activityGroupCount ?? 0,
      scheduleCount: usageSource.scheduleCount ?? 0,
      studentEnrollmentCount: usageSource.studentEnrollmentCount ?? 0,
      supervisorCount: usageSource.supervisorCount ?? 0,
      activityInstanceCount: usageSource.activityInstanceCount ?? 0,
    };
  }, [editing, periods]);

  // Statuszeile: der heute laufende Zeitraum plus die Zahl der angelegten
  // Zeiträume, beides aus der bereits geladenen Liste.
  const today = todayISO();
  const currentPeriod = periods.find(
    (period) => period.startDate <= today && period.endDate >= today,
  );
  const statusLine = `${currentPeriod ? `${currentPeriod.name} · ` : ""}${
    periods.length
  } ${periods.length === 1 ? "Zeitraum" : "Zeiträume"} · ${
    periods.filter((period) => period.isActive).length
  } aktiv`;

  return {
    periods,
    phases,
    loading,
    error,
    statusLine,
    modalOpen,
    editing,
    createDefaults,
    editingUsage,
    beginCreate,
    beginCreateSemester,
    beginEdit,
    closeModal,
    reload,
    handlePhaseLinkToggle,
  };
}
