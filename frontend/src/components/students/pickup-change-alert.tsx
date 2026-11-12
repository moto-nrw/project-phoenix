"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { StatusBadge } from "~/components/ui/status-badge";
import { usePickupChangeNotices } from "~/lib/hooks/use-pickup-change-notices";
import { createLogger } from "~/lib/logger";
import { pickupDayLabel } from "~/lib/pickup-day-label";
import type { PickupChangeNotice } from "~/lib/pickup-change-notices-api";
import { fetchStudentPickupData } from "~/lib/pickup-schedule-api";
import type { PickupException } from "~/lib/pickup-schedule-helpers";
import { formatPickupTime } from "~/lib/pickup-schedule-helpers";
import { useTenantMutate } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";

const logger = createLogger({ component: "PickupChangeAlert" });

const NO_FRESH_CHANGES: readonly PickupChangeNotice[] = [];

export interface PickupChange {
  readonly date: string;
  /** The time that applies; empty when the regular time applies again. */
  readonly time: string;
  /** The reader has not seen this change yet (#6855). */
  readonly isNew?: boolean;
}

/**
 * Die Tage ab heute mit einer geänderten Abholzeit (#6855). Eine von den
 * Eltern angefragte Zeit wird mit der Freigabe zur Ausnahme der OGS; die
 * Herkunft trägt die Ausnahme danach nicht mehr. Der Hinweis zeigt deshalb
 * jede Tagesausnahme mit Uhrzeit. Eine Ausnahme gibt es je Kind und Tag nur
 * einmal: gezeigt wird immer der aktuelle Stand. Zurückgenommene Ausnahmen
 * sind gelöscht und fehlen, vergangene Tage fallen heraus.
 *
 * `fresh` sind die Änderungen, die die Person noch nicht gesehen hatte: ein
 * Tag mit Ausnahme trägt „Neu“, eine zurückgenommene Änderung ohne Ausnahme
 * steht als „wieder normale Abholzeit“ mit „Neu“ dabei.
 */
export function upcomingPickupChanges(
  exceptions: readonly PickupException[],
  today: string,
  fresh: readonly PickupChangeNotice[] = [],
): PickupChange[] {
  const freshDays = new Set(
    fresh.filter((notice) => !notice.withdrawn).map((notice) => notice.date),
  );
  const changes: PickupChange[] = exceptions
    .filter(
      (exception) =>
        Boolean(exception.pickupTime) && exception.exceptionDate >= today,
    )
    .map((exception) => ({
      date: exception.exceptionDate,
      time: formatPickupTime(exception.pickupTime ?? ""),
      ...(freshDays.has(exception.exceptionDate) ? { isNew: true } : {}),
    }));
  const changedDays = new Set(changes.map((change) => change.date));
  for (const notice of fresh) {
    if (
      notice.withdrawn &&
      notice.date >= today &&
      !changedDays.has(notice.date)
    ) {
      changedDays.add(notice.date);
      changes.push({ date: notice.date, time: "", isNew: true });
    }
  }
  return changes.sort((a, b) => a.date.localeCompare(b.date));
}

function changeLine(change: PickupChange, today: string): string {
  const day = pickupDayLabel(change.date, today);
  return change.time === ""
    ? `${day}: wieder normale Abholzeit`
    : `${day}: ${change.time} Uhr`;
}

function ChangeLine({
  change,
  today,
}: Readonly<{ change: PickupChange; today: string }>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-2">
      {changeLine(change, today)}
      {change.isNew ? (
        <StatusBadge
          compact
          tone="red"
          label="Neu"
          accessibleLabel="Neu: Diese Änderung haben Sie noch nicht gesehen."
        />
      ) : null}
    </span>
  );
}

/**
 * Hinweis in der Kinderakte: An einzelnen Tagen gilt eine andere Abholzeit,
 * etwa nach einer freigegebenen Anfrage der Eltern. Er zeigt den aktuellen
 * Stand und verschwindet, wenn die Ausnahme zurückgenommen wird oder der Tag
 * vorbei ist.
 */
export function PickupChangeAlert({
  exceptions,
  today,
  fresh = NO_FRESH_CHANGES,
}: Readonly<{
  exceptions: readonly PickupException[] | undefined;
  today: string;
  fresh?: readonly PickupChangeNotice[];
}>) {
  const changes = upcomingPickupChanges(exceptions ?? [], today, fresh);
  if (changes.length === 0) return null;
  const message =
    changes.length === 1 ? (
      <ChangeLine change={changes[0]!} today={today} />
    ) : (
      <ul className="list-none space-y-0.5">
        {changes.map((change) => (
          <li key={change.date}>
            <ChangeLine change={change} today={today} />
          </li>
        ))}
      </ul>
    );
  return (
    <Alert
      type="info"
      title="Abholzeit an einzelnen Tagen geändert"
      message={message}
      announce="off"
      data-testid="pickup-change-alert"
    />
  );
}

/**
 * Der Hinweis in der Kinderakte mit dem Lesestand der Person (#6855): Öffnen
 * des Kindes heißt gesehen. Vorher lädt die Akte die Abholzeiten neu, denn
 * die Abfrage der Hinweise frischt sie nicht auf. Bestätigt werden nur die
 * Hinweise, die dabei geladen waren; „Neu“ bleibt stehen, solange die Akte
 * offen ist, der Zähler im Header geht sofort zurück.
 */
export function StudentPickupChanges({
  studentId,
  exceptions,
  today,
}: Readonly<{
  studentId: string;
  exceptions: readonly PickupException[] | undefined;
  today: string;
}>) {
  const { notices, markRead } = usePickupChangeNotices();
  const tenantMutate = useTenantMutate();
  const [seen, setSeen] = useState<readonly PickupChangeNotice[]>([]);
  const unreadHere = useMemo(
    () => notices.filter((notice) => notice.studentId === studentId),
    [notices, studentId],
  );
  const loaded = exceptions !== undefined;
  // Each loaded revision is confirmed once per visit; a failed confirmation
  // keeps the header counter and is retried on the next visit.
  const confirmed = useRef(new Set<string>());
  useEffect(() => {
    const pending = unreadHere.filter(
      (notice) =>
        !confirmed.current.has(`${notice.requestId}@${notice.informedAt}`),
    );
    if (!loaded || pending.length === 0) return;
    for (const notice of pending) {
      confirmed.current.add(`${notice.requestId}@${notice.informedAt}`);
    }
    // A started confirmation finishes even if the list or the page changes
    // meanwhile: it names its child and its revisions itself.
    // A revalidation can resolve cached data after a failed fetch. Mutate
    // with an explicit fetch so only a successful refresh is acknowledged.
    void tenantMutate(
      studentKeys.pickupData(studentId),
      () => fetchStudentPickupData(studentId),
      { revalidate: false },
    )
      .then(async (refreshed) => {
        // A scope or data-generation change discards the refresh.
        if (refreshed === undefined) return;
        setSeen((previous) => [
          ...previous.filter(
            (old) =>
              !pending.some((notice) => notice.requestId === old.requestId),
          ),
          ...pending,
        ]);
        await markRead(studentId, pending);
      })
      .catch((caught: unknown) => {
        // A failed refresh or confirmation leaves the header counter intact.
        logger.warn("pickup_change_notices_read_failed", {
          error: caught instanceof Error ? caught.message : String(caught),
        });
      });
  }, [loaded, unreadHere, tenantMutate, markRead, studentId]);
  // Until the confirmation is back, the unread changes count as new too.
  const fresh = useMemo(
    () => [
      ...seen,
      ...unreadHere.filter(
        (notice) => !seen.some((old) => old.requestId === notice.requestId),
      ),
    ],
    [seen, unreadHere],
  );
  return (
    <PickupChangeAlert exceptions={exceptions} today={today} fresh={fresh} />
  );
}
