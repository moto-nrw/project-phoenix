"use client";

/**
 * Ferienwochen, die die Schule für ihren Träger ausrichtet, für Betreuungs-
 * und Dienstplan (ADR 0116).
 *
 * Die Wochenliste nennt Zeitraum, Betreuungszeit und die Zahl der Kinder,
 * kein Kind. Lesen dürfen sie, wer die Seite „Ferienwochen“ öffnet
 * (config:read), wer den Betreuungsplan liest (schedules:read) und wer den
 * Dienstplan plant (time_tracking:manage). Ohne eines dieser Rechte lädt der
 * Hook nichts; eine 401/403 bleibt still. Andere Fehler zeigt die Ansicht als
 * Ladefehler mit „Erneut versuchen“: ein stilles Fehlen sähe aus wie eine
 * Woche ohne Ferienbetreuung.
 *
 * Ein SWR-Key für alle Wochen, wie bei den Schließtagen: die Liste ist kurz
 * (nur Wochen, die noch nicht vorbei sind), ein Wochenwechsel lädt nichts.
 */

import { useSession } from "next-auth/react";

import { ApiError } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import {
  fetchHostedHolidayWeeks,
  type HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { holidayKeys } from "~/lib/swr/portal-keys";

const OPTIONS = {
  revalidateOnFocus: false,
  dedupingInterval: 60_000,
} as const;

const EMPTY: readonly HostedHolidayWeek[] = [];

function isMissingPermission(error: unknown): boolean {
  return (
    error instanceof ApiError && (error.status === 401 || error.status === 403)
  );
}

export interface PlanningHolidayWeeks {
  readonly weeks: readonly HostedHolidayWeek[];
  /** Ladefehler für `LoadErrorAlert`, `null` ohne Fehler. */
  readonly loadError: ReturnType<typeof useSwrLoadError>;
  /** Darf die Seite „Ferienwochen“ öffnen: die Markierung wird ein Link. */
  readonly canOpenList: boolean;
}

export function usePlanningHolidayWeeks(): PlanningHolidayWeeks {
  const { data: session } = useSession();
  const canOpenList = hasPermission(session, "config:read");
  const canRead =
    canOpenList ||
    hasPermission(session, "schedules:read") ||
    hasPermission(session, "time_tracking:manage");
  const { data, error, mutate } = useSWRAuth(
    // Derselbe Eintrag wie auf der Seite „Ferienwochen“: ein Cache für beide.
    canRead ? holidayKeys.hostedWeeks() : null,
    fetchHostedHolidayWeeks,
    OPTIONS,
  );
  const loadError = useSwrLoadError(
    isMissingPermission(error) ? null : error,
    "die Ferienbetreuung",
    () => mutate(),
  );
  return { weeks: data ?? EMPTY, loadError, canOpenList };
}
