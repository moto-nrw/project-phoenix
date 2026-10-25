"use client";

// Verlauf einer Ferienanmeldung (#4663): jede Änderung des Träger-Büros mit
// Zeitpunkt, beim Umbuchen die Wochen vorher und nachher. Nur zum Lesen.

import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { HolidayPeriod } from "~/lib/traeger-api";
import type {
  HolidayBookingChange,
  HolidayRegistrationDetail,
  HolidayRegistrationEvent,
} from "~/lib/traeger-holiday-registrations-api";
import { formatHolidayWeek } from "./holiday-registration-format";

const ACTION_LABELS: Readonly<
  Record<HolidayRegistrationEvent["action"], string>
> = {
  confirm: "Bestätigt",
  cancel: "Storniert",
  freeze: "Eingefroren",
  reopen: "Wieder geöffnet",
  rebook: "Umgebucht",
};

const BERLIN_TIME = new Intl.DateTimeFormat("de-DE", {
  timeZone: "Europe/Berlin",
  hour: "2-digit",
  minute: "2-digit",
});

function bookingsText(
  period: HolidayPeriod,
  changes: readonly HolidayBookingChange[],
): string {
  if (changes.length === 0) return "keine Woche";
  return changes
    .map((change) => {
      const week = period.weeks.find((entry) => entry.id === change.weekId);
      const model = week?.timeModels.find(
        (entry) => entry.id === change.timeModelId,
      );
      if (!week || !model) return "eine entfernte Woche";
      return `${formatHolidayWeek(week.startsOn, week.endsOn)}: ${model.name}`;
    })
    .join("; ");
}

function eventDetail(
  event: HolidayRegistrationEvent,
  period: HolidayPeriod,
  registration: HolidayRegistrationDetail,
): string | null {
  if (event.action !== "rebook") return null;
  const child = registration.children.find(
    (entry) => entry.id === event.childId,
  );
  const name = child ? `${child.firstName} ${child.lastName}: ` : "";
  return `${name}vorher ${bookingsText(period, event.before)}, jetzt ${bookingsText(period, event.after)}`;
}

export function HolidayRegistrationHistory({
  period,
  registration,
}: Readonly<{
  period: HolidayPeriod;
  registration: HolidayRegistrationDetail;
}>) {
  return (
    <SectionCard title="Verlauf">
      {registration.history.length === 0 ? (
        <p className="text-sm text-gray-700">
          Das Träger-Büro hat diese Anmeldung noch nicht geändert.
        </p>
      ) : (
        <ol className="space-y-2 text-sm text-gray-700">
          {registration.history.map((event, index) => {
            const detail = eventDetail(event, period, registration);
            return (
              <li key={`${event.occurredAt}-${index}`}>
                <span className="text-gray-500">
                  {formatBerlinDate(event.occurredAt)},{" "}
                  {BERLIN_TIME.format(new Date(event.occurredAt))} Uhr
                </span>{" "}
                <span className="font-medium text-gray-900">
                  {ACTION_LABELS[event.action]}
                </span>
                {detail ? <span>. {detail}</span> : null}
              </li>
            );
          })}
        </ol>
      )}
    </SectionCard>
  );
}
