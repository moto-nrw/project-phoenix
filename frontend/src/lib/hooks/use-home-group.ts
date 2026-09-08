"use client";

import { useSession } from "next-auth/react";

import { fetchOgsGroupLive } from "~/lib/ogs-group-live-api";
import type {
  OgsLiveViewData,
  OgsLiveWireStudent,
} from "~/lib/ogs-group-live-api";
import { useSWRAuth } from "~/lib/swr";

export interface HomeGroupSnapshot {
  /** Die eigene Gruppe, oder null, wenn die Person heute keine betreut. */
  readonly group: {
    readonly id: string;
    readonly name: string;
    readonly roomName?: string;
    readonly viaSubstitution: boolean;
  } | null;
  readonly present: number;
  readonly total: number;
  /** Kinder, die heute nicht (mehr) da sind, in der Reihenfolge der Gruppe. */
  readonly away: readonly OgsLiveWireStudent[];
  /** Nächste Abholzeit „HH:MM" unter den anwesenden Kindern, oder null. */
  readonly nextPickup: string | null;
  readonly isLoading: boolean;
  readonly error: Error | undefined;
}

const EMPTY: Omit<HomeGroupSnapshot, "isLoading" | "error"> = {
  group: null,
  present: 0,
  total: 0,
  away: [],
  nextPickup: null,
};

/** Warum ein Kind heute fehlt — die Abweichung ist die Nachricht. */
export function isAwayToday(student: OgsLiveWireStudent): boolean {
  return (
    student.sick ||
    student.excused ||
    student.class_trip ||
    student.day_planning_status === "not_coming_today" ||
    student.current_location === "HOME"
  );
}

/**
 * Der Stand der eigenen Gruppe für die Startseite (#2180): wie viele da
 * sind, wer fehlt, wann die nächste Abholung ist.
 *
 * Dieselbe Quelle wie die Seite „Meine Gruppen", ohne eigene Gruppen-Id: der
 * Server wählt die Gruppe der Person (bei mehreren die erste). Ein
 * Gruppenwechsel auf jener Seite ändert die Startseite nicht — sie zeigt
 * den Stand, nicht die Auswahl.
 */
export function useHomeGroup(
  enabled: boolean,
  /** Uhrzeit „HH:MM": eine Abholung, die vorbei ist, ist nicht die nächste. */
  now: string,
): HomeGroupSnapshot {
  const { data: session } = useSession();
  const token = session?.user?.token;

  const { data, error, isLoading } = useSWRAuth<OgsLiveViewData>(
    enabled ? "home-own-group" : null,
    () => fetchOgsGroupLive(token, null),
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );

  const found = data?.groups.find((g) => g.id === data.groupId) ?? null;
  if (!data || !found) {
    return { ...EMPTY, isLoading, error };
  }

  const students = data.students;
  const away = students.filter(isAwayToday);
  const awayIds = new Set(away.map((s) => s.id));
  const upcoming = students
    .filter((s) => !awayIds.has(s.id))
    .map((s) => data.pickupTimes.get(s.id)?.pickupTime)
    .filter((t): t is string => Boolean(t) && (t as string) >= now)
    .sort((a, b) => a.localeCompare(b));

  return {
    group: {
      id: found.id,
      name: found.name,
      roomName: found.roomName,
      viaSubstitution: found.viaSubstitution,
    },
    present: students.length - away.length,
    total: students.length,
    away,
    nextPickup: upcoming[0] ?? null,
    isLoading,
    error,
  };
}
