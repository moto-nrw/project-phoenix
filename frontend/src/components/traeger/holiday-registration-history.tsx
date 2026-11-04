"use client";

// Verlauf einer Ferienanmeldung (#4663): jede Änderung des Träger-Büros mit
// Zeitpunkt, beim Umbuchen die Wochen vorher und nachher, bei der
// Debitorennummer die Nummer vorher und nachher (#5621), beim Ändern der
// Angaben die geänderten Felder ohne ihre Werte (#5629), bei Geschwistern
// die anderen Anmeldungen (#5628), auch wenn eine Stornierung die Gruppe
// löst (#5662). Nur zum Lesen.

import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { HolidayPeriod } from "~/lib/traeger-api";
import type {
  HolidayBookingChange,
  HolidayRegistrationDetail,
  HolidayRegistrationEvent,
} from "~/lib/traeger-holiday-registrations-api";
import { editedFieldsText } from "./holiday-registration-edit-fields";
import { formatHolidayWeek } from "./holiday-registration-format";

const ACTION_LABELS: Readonly<
  Record<HolidayRegistrationEvent["action"], string>
> = {
  confirm: "Bestätigt",
  cancel: "Storniert",
  freeze: "Eingefroren",
  reopen: "Wieder geöffnet",
  rebook: "Umgebucht",
  debtor_number: "Debitorennummer geändert",
  edit: "Angaben geändert",
  proof_upload: "Nachweis hochgeladen",
  sibling_merge: "Als Geschwister zusammengeführt",
  sibling_split: "Von Geschwistern getrennt",
  sibling_cancel: "Aus Geschwistergruppe gelöst (storniert)",
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

/** The debtor number a confirmation entered or a change changed (#5621). */
function debtorNumberText(change: {
  readonly before: string;
  readonly after: string;
}): string {
  if (!change.before) return `Debitorennummer ${change.after}`;
  if (!change.after) return `Debitorennummer ${change.before} entfernt`;
  return `Debitorennummer vorher ${change.before}, jetzt ${change.after}`;
}

function eventDetail(
  event: HolidayRegistrationEvent,
  period: HolidayPeriod,
  registration: HolidayRegistrationDetail,
): string | null {
  if (event.action === "edit") {
    return editedFieldsText(event.fields, registration.form.fields) || null;
  }
  if (event.debtorNumber) return debtorNumberText(event.debtorNumber);
  if (event.siblings.length > 0) {
    return `Anmeldung Nr. ${event.siblings.join(", ")}`;
  }
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
        // Eine Debitorennummer hat bis zu 35 Zeichen ohne Leerzeichen (#5621);
        // sie bricht um, statt am Kartenrand abgeschnitten zu werden.
        <ol className="space-y-2 text-sm wrap-anywhere text-gray-700">
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
                {detail ? (
                  <span>
                    {event.action === "edit" ? ": " : ". "}
                    {detail}
                  </span>
                ) : null}
              </li>
            );
          })}
        </ol>
      )}
    </SectionCard>
  );
}
