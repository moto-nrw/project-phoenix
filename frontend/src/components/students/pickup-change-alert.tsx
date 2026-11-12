"use client";

import { Alert } from "~/components/ui/alert";
import { formatStatusDate, parseISODate, toISODate } from "~/lib/date-helpers";
import type { PickupException } from "~/lib/pickup-schedule-helpers";
import { formatPickupTime } from "~/lib/pickup-schedule-helpers";

export interface PickupChange {
  readonly date: string;
  readonly time: string;
}

/**
 * Die Tage ab heute mit einer geänderten Abholzeit (#6855). Eine von den
 * Eltern angefragte Zeit wird mit der Freigabe zur Ausnahme der OGS; die
 * Herkunft trägt die Ausnahme danach nicht mehr. Der Hinweis zeigt deshalb
 * jede Tagesausnahme mit Uhrzeit. Eine Ausnahme gibt es je Kind und Tag nur
 * einmal: gezeigt wird immer der aktuelle Stand. Zurückgenommene Ausnahmen
 * sind gelöscht und fehlen, vergangene Tage fallen heraus.
 */
export function upcomingPickupChanges(
  exceptions: readonly PickupException[],
  today: string,
): PickupChange[] {
  return exceptions
    .filter(
      (exception) =>
        Boolean(exception.pickupTime) && exception.exceptionDate >= today,
    )
    .map((exception) => ({
      date: exception.exceptionDate,
      time: formatPickupTime(exception.pickupTime ?? ""),
    }))
    .sort((a, b) => a.date.localeCompare(b.date));
}

function dayLabel(date: string, today: string): string {
  if (date === today) return "Heute";
  const tomorrow = parseISODate(today);
  tomorrow.setDate(tomorrow.getDate() + 1);
  if (date === toISODate(tomorrow)) return "Morgen";
  return formatStatusDate(date);
}

function changeLine(change: PickupChange, today: string): string {
  return `${dayLabel(change.date, today)}: ${change.time} Uhr`;
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
}: Readonly<{
  exceptions: readonly PickupException[] | undefined;
  today: string;
}>) {
  const changes = upcomingPickupChanges(exceptions ?? [], today);
  if (changes.length === 0) return null;
  const message =
    changes.length === 1 ? (
      changeLine(changes[0]!, today)
    ) : (
      <ul className="list-none space-y-0.5">
        {changes.map((change) => (
          <li key={change.date}>{changeLine(change, today)}</li>
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
