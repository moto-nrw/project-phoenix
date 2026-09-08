"use client";

import { useSession } from "next-auth/react";

import { fetchOgsGroupLive } from "~/lib/ogs-group-live-api";
import type {
  OgsLiveViewData,
  OgsLiveWireStudent,
} from "~/lib/ogs-group-live-api";
import { combineTimeNotes } from "~/lib/student-time-status";
import { useSWRAuth } from "~/lib/swr";

/** Eine Abholung, die heute noch kommt. */
export interface HomePickup {
  readonly student: OgsLiveWireStudent;
  /** „HH:MM" */
  readonly time: string;
  /** Was die Eltern dazu gesagt haben: Abholnotiz und Tagesnotizen in einem Satz. */
  readonly note?: string;
  /** Weicht heute vom Regelfall ab (Ausnahme im Betreuungsplan)? */
  readonly isException: boolean;
}

/** Ein Kind, das längst da sein sollte und noch nicht gekommen ist. */
export interface HomeMissingArrival {
  readonly student: OgsLiveWireStudent;
  /** Wann es kommen sollte, „HH:MM". */
  readonly expected: string;
  /** Was die Eltern zur Ankunft gesagt haben („Arzttermin, kommt danach"). */
  readonly note?: string;
}

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
  /**
   * Kinder, die heute nicht (mehr) da sind, in der Reihenfolge der Gruppe —
   * einschließlich derer, die noch erwartet werden.
   */
  readonly away: readonly OgsLiveWireStudent[];
  /**
   * Wer heute kommen sollte, dessen Ankunftszeit vorbei ist und der noch
   * nicht da ist — die Frage des Vormittags: „Wer fehlt noch?"
   */
  readonly missing: readonly HomeMissingArrival[];
  /** Abholungen ab jetzt, die früheste zuerst. */
  readonly pickups: readonly HomePickup[];
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
  missing: [],
  pickups: [],
  nextPickup: null,
};

/**
 * Wird das Kind heute noch erwartet? Nur, wenn nichts dagegen spricht (nicht
 * krank, nicht entschuldigt, nicht abgemeldet), es eine Ankunftszeit hat und
 * noch nicht angekommen ist.
 */
export function isExpectedToday(student: OgsLiveWireStudent): boolean {
  return (
    !student.sick &&
    !student.excused &&
    !student.class_trip &&
    student.day_planning_status !== "not_coming_today" &&
    Boolean(student.arrival_time) &&
    !student.actual_arrival_time
  );
}

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
 * Was die Startseite aus dem Stand der Gruppe macht: reine Rechnung, ohne
 * Netz, damit sie sich prüfen lässt.
 *
 * Abholungen zählen nur für Kinder, die da sind und noch nicht abgeholt
 * wurden, und nur, wenn die Zeit noch nicht vorbei ist. Ist die Uhr noch
 * unbekannt (erstes Rendern), gelten alle Abholzeiten als kommend — besser
 * eine Zeile zu viel als eine Karte, die erst nach einer Sekunde Inhalt hat.
 */
export function deriveHomeGroup(
  data: OgsLiveViewData,
  now: string,
): Omit<HomeGroupSnapshot, "isLoading" | "error"> {
  const found = data.groups.find((g) => g.id === data.groupId) ?? null;
  if (!found) return EMPTY;

  const students = data.students;
  const away = students.filter(isAwayToday);
  const awayIds = new Set(away.map((s) => s.id));
  const here = students.filter((s) => !awayIds.has(s.id));

  const pickups: HomePickup[] = here
    .flatMap((student) => {
      const info = data.pickupTimes.get(student.id);
      const time = info?.pickupTime;
      if (!time || student.actual_pickup_time || time < now) return [];
      return [
        {
          student,
          time,
          note: combineTimeNotes(info.notes, info.dayNotes),
          isException: info.isException,
        },
      ];
    })
    .sort((a, b) => a.time.localeCompare(b.time));

  // Ist die Uhr noch unbekannt, fehlt niemand: eine Kachel „fehlt seit",
  // die beim nächsten Bild wieder verschwindet, wäre ein falscher Alarm.
  const missing: HomeMissingArrival[] =
    now === ""
      ? []
      : away
          .filter(
            (student) =>
              isExpectedToday(student) && (student.arrival_time ?? "") < now,
          )
          .map((student) => ({
            student,
            expected: student.arrival_time!,
            note: student.arrival_notes || undefined,
          }));

  return {
    group: {
      id: found.id,
      name: found.name,
      roomName: found.roomName,
      viaSubstitution: found.viaSubstitution,
    },
    present: here.length,
    total: students.length,
    away,
    missing,
    pickups,
    nextPickup: pickups[0]?.time ?? null,
  };
}

/**
 * Der Stand der eigenen Gruppe für die Startseite (#2180): wie viele da
 * sind, wer wann abgeholt wird, wer fehlt.
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

  if (!data) return { ...EMPTY, isLoading, error };
  return { ...deriveHomeGroup(data, now), isLoading, error };
}
