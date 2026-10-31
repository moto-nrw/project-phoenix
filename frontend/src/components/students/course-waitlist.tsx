"use client";

/**
 * Warteliste je AG für das Team (#4767). Die Reihenfolge kommt aus dem
 * Backend und folgt derselben Regel wie der Wartelistenplatz im Elternportal.
 * Nachrücken heißt: das Team bestätigt die Anfrage des ersten Kindes, wie
 * jede andere Anfrage. Gespeichert wird nichts Eigenes.
 */

import { createContext, useContext, useMemo } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  waitlistPlacesByRequest,
  waitlistsWithFreeSlot,
  waitlistsWithQueue,
  type CourseWaitlist,
  type RequestWaitlistPlace,
} from "~/lib/course-waitlist-helpers";

interface CourseWaitlistContextValue {
  readonly places: ReadonlyMap<string, readonly RequestWaitlistPlace[]>;
  /** Die Anfrage, zu der die Warteliste zuletzt gesprungen ist. */
  readonly openRequestId: string | null;
  /** Nach einer Entscheidung neu laden: die Reihenfolge hat sich verschoben. */
  readonly refresh: () => void;
}

const EMPTY_CONTEXT: CourseWaitlistContextValue = {
  places: new Map(),
  openRequestId: null,
  refresh: () => undefined,
};

const CourseWaitlistContext =
  createContext<CourseWaitlistContextValue>(EMPTY_CONTEXT);

export function CourseWaitlistProvider({
  waitlists,
  refresh,
  openRequestId = null,
  children,
}: Readonly<{
  waitlists: readonly CourseWaitlist[];
  refresh: () => void;
  openRequestId?: string | null;
  children: React.ReactNode;
}>) {
  const value = useMemo(
    () => ({
      places: waitlistPlacesByRequest(waitlists),
      openRequestId,
      refresh,
    }),
    [waitlists, openRequestId, refresh],
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

/** Ist diese Anfrage das Ziel des letzten Sprungs aus der Warteliste? */
export function useOpenedFromWaitlist(requestId: string): boolean {
  return useContext(CourseWaitlistContext).openRequestId === requestId;
}

function freeSlotSentence(waitlist: CourseWaitlist): string {
  const next = waitlist.entries[0];
  return `In „${waitlist.name}“ ist ein Platz frei. Als Nächstes ist ${next?.studentName ?? "ein Kind"} dran.`;
}

/**
 * Oben in /anfragen: die Wartelisten aller AGs mit Teilnehmergrenze, auf die
 * gerade ein Kind wartet. Je AG die Belegung und die Kinder in der
 * Reihenfolge ihrer Anfrage; „Zur Anfrage“ öffnet die Anfrage des Kindes in
 * der Liste darunter. Ist ein Platz frei, steht das an der AG und am ersten
 * Kind.
 */
export function CourseWaitlistOverview({
  waitlists,
  onOpenRequest,
}: Readonly<{
  waitlists: readonly CourseWaitlist[];
  onOpenRequest: (studentId: string, requestId: string) => void;
}>) {
  const queues = waitlistsWithQueue(waitlists);
  if (queues.length === 0) return null;
  const freeCount = waitlistsWithFreeSlot(queues).length;
  return (
    <SectionCard
      title="Wartelisten"
      titleBadge={
        freeCount > 0 ? (
          <StatusBadge
            tone="green"
            label={
              freeCount === 1
                ? "1 AG mit freiem Platz"
                : `${freeCount} AGs mit freiem Platz`
            }
          />
        ) : undefined
      }
      description="Kinder, die auf einen Platz in einer AG warten. Die älteste Anfrage ist zuerst dran. Wird ein Platz frei, geben Sie die Anfrage des ersten Kindes frei."
      collapsible
    >
      <div className="divide-y divide-gray-100">
        {queues.map((waitlist) => (
          <CourseWaitlistBlock
            key={waitlist.activityGroupId}
            waitlist={waitlist}
            onOpenRequest={onOpenRequest}
          />
        ))}
      </div>
    </SectionCard>
  );
}

function occupancyLine(waitlist: CourseWaitlist): string {
  const limit = waitlist.participantLimit ?? 0;
  const free = waitlist.freeSlots ?? 0;
  const state = free > 0 ? `${free} frei` : "voll";
  return `${waitlist.participants} von ${limit} Plätzen vergeben · ${state}`;
}

function CourseWaitlistBlock({
  waitlist,
  onOpenRequest,
}: Readonly<{
  waitlist: CourseWaitlist;
  onOpenRequest: (studentId: string, requestId: string) => void;
}>) {
  const headingId = `waitlist-${waitlist.activityGroupId}`;
  return (
    <section aria-labelledby={headingId} className="py-4 first:pt-0 last:pb-0">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <h3 id={headingId} className="text-sm font-semibold text-gray-900">
          {waitlist.name}
        </h3>
        {waitlist.slotFreeForNext && (
          <StatusBadge tone="green" label="Platz frei" />
        )}
      </div>
      <p className="mb-2 text-sm text-gray-600">{occupancyLine(waitlist)}</p>
      <ResponsiveActionList>
        {waitlist.entries.map((entry) => (
          <li
            key={entry.requestId}
            className="flex flex-col gap-2 py-2 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="flex flex-wrap items-center gap-2 text-sm text-gray-900">
                <span className="w-5 shrink-0 text-right font-medium text-gray-500 tabular-nums">
                  {entry.rank}.
                </span>
                <span className="font-medium">{entry.studentName}</span>
                {waitlist.slotFreeForNext && entry.rank === 1 && (
                  <StatusBadge tone="green" label="Als Nächstes dran" />
                )}
              </p>
              <p className="mt-0.5 pl-7 text-xs text-gray-500">
                Angefragt am {formatDate(entry.submittedAt)}
              </p>
            </div>
            <div className="flex w-full min-[480px]:w-auto sm:justify-end">
              <Button
                type="button"
                variant="surface"
                size="compact"
                aria-label={`Anfrage von ${entry.studentName} öffnen`}
                onClick={() => onOpenRequest(entry.studentId, entry.requestId)}
              >
                Zur Anfrage
              </Button>
            </div>
          </li>
        ))}
      </ResponsiveActionList>
    </section>
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
      badge: { tone: "orange", label: `Warteliste, Platz ${entry.rank}` },
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
  participantNames,
}: Readonly<{
  waitlist: CourseWaitlist;
  /** Kinder, die heute in der AG sind; fehlt die Liste, bleibt sie weg. */
  participantNames?: readonly string[];
}>) {
  const limit = waitlist.participantLimit;
  const occupancy =
    limit === null
      ? `${waitlist.participants} Kinder angemeldet · keine Teilnehmergrenze`
      : `${waitlist.participants} von ${limit} Plätzen vergeben · ${waitlist.freeSlots ?? 0} frei`;
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-gray-700">{occupancy}</p>
      {participantNames && participantNames.length > 0 && (
        <p className="text-sm text-gray-700">
          <span className="text-gray-500">Teilnehmende: </span>
          {participantNames.join(", ")}
        </p>
      )}
      {waitlist.slotFreeForNext && (
        <Alert
          type="info"
          message={`${freeSlotSentence(waitlist)} Geben Sie die Anfrage unter „Anfragen“ frei.`}
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
