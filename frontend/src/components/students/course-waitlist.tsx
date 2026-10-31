"use client";

/**
 * Warteliste je AG für das Team (#4767). Die Reihenfolge kommt aus dem
 * Backend und folgt derselben Regel wie der Wartelistenplatz im Elternportal.
 * Nachrücken heißt: das Team bestätigt die Anfrage des ersten Kindes, wie
 * jede andere Anfrage. Gespeichert wird nichts Eigenes.
 */

import { createContext, useContext, useMemo } from "react";

import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  waitlistPlacesByRequest,
  waitlistsWithFreeSlot,
  type CourseWaitlist,
  type RequestWaitlistPlace,
} from "~/lib/course-waitlist-helpers";

interface CourseWaitlistContextValue {
  readonly places: ReadonlyMap<string, readonly RequestWaitlistPlace[]>;
  /** Nach einer Entscheidung neu laden: die Reihenfolge hat sich verschoben. */
  readonly refresh: () => void;
}

const EMPTY_CONTEXT: CourseWaitlistContextValue = {
  places: new Map(),
  refresh: () => undefined,
};

const CourseWaitlistContext =
  createContext<CourseWaitlistContextValue>(EMPTY_CONTEXT);

export function CourseWaitlistProvider({
  waitlists,
  refresh,
  children,
}: Readonly<{
  waitlists: readonly CourseWaitlist[];
  refresh: () => void;
  children: React.ReactNode;
}>) {
  const value = useMemo(
    () => ({ places: waitlistPlacesByRequest(waitlists), refresh }),
    [waitlists, refresh],
  );
  return (
    <CourseWaitlistContext.Provider value={value}>
      {children}
    </CourseWaitlistContext.Provider>
  );
}

export function useCourseWaitlistRefresh(): () => void {
  return useContext(CourseWaitlistContext).refresh;
}

function freeSlotSentence(waitlist: CourseWaitlist): string {
  const next = waitlist.entries[0];
  return `In „${waitlist.name}“ ist ein Platz frei. Als Nächstes ist ${next?.studentName ?? "ein Kind"} dran.`;
}

/**
 * Oben in /anfragen: für jede AG mit freiem Platz und wartendem Kind ein
 * Hinweis auf das nächste Kind. Ein Hinweis bleibt ein Alert, mehrere stehen
 * in einer Karte.
 */
export function CourseWaitlistHints({
  waitlists,
}: Readonly<{ waitlists: readonly CourseWaitlist[] }>) {
  const free = waitlistsWithFreeSlot(waitlists);
  if (free.length === 0) return null;
  if (free.length === 1 && free[0]) {
    return (
      <Alert
        type="info"
        message={`${freeSlotSentence(free[0])} Bestätigen Sie unten die Anfrage, dann rückt das Kind nach.`}
      />
    );
  }
  return (
    <SectionCard
      title="Freie Plätze in AGs"
      description="Bestätigen Sie unten die Anfrage des nächsten Kindes, dann rückt es nach."
    >
      <ul className="divide-y divide-gray-100">
        {free.map((waitlist) => (
          <li key={waitlist.activityGroupId} className="py-2 text-sm">
            <span className="font-medium text-gray-900">{waitlist.name}</span>
            <span className="text-gray-600">
              {" "}
              · Als Nächstes: {waitlist.entries[0]?.studentName ?? "–"}
            </span>
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}

function placeLine(place: RequestWaitlistPlace): {
  badge: { tone: "green" | "orange" | "gray"; label: string };
  text: string;
} {
  const { waitlist, entry } = place;
  if (waitlist.slotFreeForNext && entry.rank === 1) {
    return {
      badge: { tone: "green", label: "Platz frei" },
      text: "Dieses Kind ist als Nächstes dran.",
    };
  }
  if (waitlist.freeSlots === 0) {
    return {
      badge: { tone: "orange", label: `Warteliste Platz ${entry.rank}` },
      text: "Die AG ist voll. Wird ein Platz frei, steht es hier.",
    };
  }
  return {
    badge: { tone: "gray", label: `Platz ${entry.rank}` },
    text: "Zuerst ist eine ältere Anfrage dran.",
  };
}

/** Im Anfrage-Kärtchen: der Platz des Kindes je angefragter AG. */
export function RequestWaitlistPlaces({
  requestId,
}: Readonly<{ requestId: string }>) {
  const places = useContext(CourseWaitlistContext).places.get(requestId);
  // Eine AG ohne Teilnehmergrenze hat keine Warteliste, nur offene Anfragen.
  const limited = (places ?? []).filter(
    (place) => place.waitlist.participantLimit !== null,
  );
  if (limited.length === 0) return null;
  return (
    <div className="mt-3 border-t border-gray-200 pt-2">
      {limited.map((place) => {
        const line = placeLine(place);
        return (
          <div key={place.waitlist.activityGroupId} className="text-sm">
            <span className="text-xs text-gray-500">
              {place.waitlist.name}:{" "}
            </span>
            <StatusBadge tone={line.badge.tone} label={line.badge.label} />
            <p className="mt-0.5 text-xs text-gray-500">{line.text}</p>
          </div>
        );
      })}
    </div>
  );
}

/**
 * Im Fenster der AG: belegte und freie Plätze und die Warteliste mit Rang.
 * Nur zur Information; bestätigt wird in „Anfragen“.
 */
export function CourseWaitlistSummary({
  waitlist,
}: Readonly<{ waitlist: CourseWaitlist }>) {
  const limit = waitlist.participantLimit;
  const occupancy =
    limit === null
      ? `${waitlist.participants} Kinder angemeldet · keine Teilnehmergrenze`
      : `${waitlist.participants} von ${limit} Plätzen belegt · ${waitlist.freeSlots ?? 0} frei`;
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-gray-700">{occupancy}</p>
      {waitlist.slotFreeForNext && (
        <Alert
          type="info"
          message={`${freeSlotSentence(waitlist)} Bestätigen Sie die Anfrage unter „Anfragen“.`}
        />
      )}
      {waitlist.entries.length === 0 ? (
        <p className="text-sm text-gray-500">Niemand wartet auf einen Platz.</p>
      ) : (
        <ol className="divide-y divide-gray-100" aria-label="Warteliste">
          {waitlist.entries.map((entry) => (
            <li
              key={entry.requestId}
              className="flex items-baseline gap-3 py-1.5 text-sm"
            >
              <span className="w-6 shrink-0 text-right font-medium text-gray-500 tabular-nums">
                {entry.rank}.
              </span>
              <span className="text-gray-900">{entry.studentName}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
