"use client";

/**
 * Ferienbuchungen, die ein Schließtag betrifft (#6434). Der Schließtag-Dialog
 * des Trägers zeigt vor dem Speichern, welche gebuchten Ferienwochen an den
 * gewählten Tagen ohne Betreuung sind und wie viele Familien danach eine
 * E-Mail bekommen. Die Seite mit den offenen Hinweisen nutzt dieselbe Liste.
 * Die Liste nennt keine Kinder: Das Büro braucht den Umfang, die Eltern
 * bekommen die Einzelheiten per E-Mail. Beim Ändern und Löschen nennt sie
 * auch die Tage, die wieder öffnen (#6457).
 */

import { CalendarCheck, CalendarX, Mail } from "lucide-react";

import { formatDayWithWeekday } from "~/components/traeger/closing-day-format";
import { Alert } from "~/components/ui/alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FramedList } from "~/components/ui/framed-list";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionLabel } from "~/components/ui/section-label";
import { SkeletonRows } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  HolidayClosureImpact,
  HolidayClosureWeek,
} from "~/lib/traeger-closing-days";

/** „1 Kind“ oder „3 Kinder“. */
export function childrenLabel(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

/** „1 Familie“ oder „3 Familien“. */
export function familiesLabel(count: number): string {
  return count === 1 ? "1 Familie" : `${count} Familien`;
}

/**
 * Tage einer Woche mit Grund, wenn einer bekannt ist:
 * „Di, 07.07.2027 (Pädagogischer Tag) und Mi, 08.07.2027 (Pädagogischer Tag)“.
 */
function daysLabel(list: HolidayClosureWeek["closedDays"]): string {
  const days = list.map((day) =>
    day.reason
      ? `${formatDayWithWeekday(day.date)} (${day.reason})`
      : formatDayWithWeekday(day.date),
  );
  if (days.length <= 1) return days.join("");
  return `${days.slice(0, -1).join(", ")} und ${days.at(-1)}`;
}

/** Geschlossene und wieder geöffnete Tage einer Woche, je eine Zeile. */
export function HolidayClosureDayLines({
  week,
}: Readonly<{ week: HolidayClosureWeek }>) {
  return (
    <>
      {week.closedDays.length > 0 ? (
        <p className="flex items-start gap-1.5 text-sm text-gray-900">
          <CalendarX
            className="mt-0.5 h-4 w-4 shrink-0 text-gray-500"
            aria-hidden="true"
          />
          <span className="min-w-0 wrap-anywhere">
            {`Geschlossen: ${daysLabel(week.closedDays)}`}
          </span>
        </p>
      ) : null}
      {week.reopenedDays.length > 0 ? (
        <p className="flex items-start gap-1.5 text-sm text-gray-900">
          <CalendarCheck
            className="mt-0.5 h-4 w-4 shrink-0 text-gray-500"
            aria-hidden="true"
          />
          <span className="min-w-0 wrap-anywhere">
            {`Wieder geöffnet: ${daysLabel(week.reopenedDays)}`}
          </span>
        </p>
      ) : null}
    </>
  );
}

/** Die betroffenen Wochen als Liste: Zeitraum, Schule, Tage und Kinder. */
function HolidayClosureWeekList({
  weeks,
  schoolName,
}: Readonly<{
  weeks: readonly HolidayClosureWeek[];
  schoolName: (id: string) => string;
}>) {
  return (
    <FramedList maxHeight="md">
      {weeks.map((week) => (
        <li
          key={week.weekId}
          className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
        >
          <div className="min-w-0 space-y-0.5">
            <p className="text-sm font-semibold wrap-anywhere text-gray-900">
              {week.periodName}
            </p>
            <p className="text-sm text-gray-600 tabular-nums">
              {formatDate(week.startsOn)} bis {formatDate(week.endsOn)}
              {" · "}
              <span className="wrap-anywhere">
                {schoolName(week.hostSchoolId)}
              </span>
            </p>
            <HolidayClosureDayLines week={week} />
          </div>
          <div className="flex shrink-0 flex-wrap gap-1.5 sm:justify-end">
            <StatusBadge label={childrenLabel(week.children)} tone="blue" />
            {week.waitlisted > 0 ? (
              <StatusBadge
                label={`${week.waitlisted} auf der Warteliste`}
                tone="gray"
              />
            ) : null}
          </div>
        </li>
      ))}
    </FramedList>
  );
}

/** Was mit Buchung und Beitrag passiert; gilt für jede Woche der Liste. */
export function HolidayClosureConsequences({
  keepsContribution,
}: Readonly<{ keepsContribution: boolean }>) {
  return (
    <ul className="space-y-1 text-sm text-gray-600">
      <li>Die Buchungen bleiben bestehen. Niemand verliert seinen Platz.</li>
      {keepsContribution ? <li>Der Beitrag bleibt gleich.</li> : null}
    </ul>
  );
}

/** Was an den betroffenen Tagen gilt, je nachdem, ob sie schließen oder öffnen. */
function impactMessage(impact: HolidayClosureImpact): string {
  const booked = `${childrenLabel(impact.children)} ${impact.children === 1 ? "hat" : "haben"} an diesen Tagen Ferienbetreuung gebucht.`;
  const closes = impact.weeks.some((week) => week.closedDays.length > 0);
  const reopens = impact.weeks.some((week) => week.reopenedDays.length > 0);
  const parts = [booked];
  if (closes) parts.push("An den geschlossenen Tagen gibt es keine Betreuung.");
  if (reopens)
    parts.push("An den wieder geöffneten Tagen gibt es doch Betreuung.");
  return parts.join(" ");
}

/**
 * Die Vorschau im Schließtag-Dialog: lädt, nichts betroffen oder die Liste mit
 * dem Hinweis, wer eine E-Mail bekommt. `action` nennt den Knopf, nach dem
 * die E-Mail rausgeht: Speichern oder Löschen.
 */
export function ClosingDayHolidayImpact({
  impact,
  loading,
  error,
  schoolName,
  action = "Speichern",
}: Readonly<{
  impact: HolidayClosureImpact | null;
  loading: boolean;
  error?: FormErrorInput;
  schoolName: (id: string) => string;
  action?: "Speichern" | "Löschen";
}>) {
  return (
    <section
      aria-label="Ferienbetreuung an diesen Tagen"
      aria-live="polite"
      aria-busy={loading}
      className="space-y-3"
    >
      <SectionLabel as="h3">Ferienbetreuung an diesen Tagen</SectionLabel>
      {error ? <LoadErrorAlert error={error} /> : null}
      {loading && !impact && !error ? (
        <InsetPanel padding="md">
          <SkeletonRows rows={2} />
        </InsetPanel>
      ) : null}
      {impact && impact.weeks.length === 0 ? (
        <Alert
          type="success"
          size="compact"
          message="Keine gebuchte Ferienbetreuung betroffen."
          announce="off"
        />
      ) : null}
      {impact && impact.weeks.length > 0 ? (
        <div className={loading ? "space-y-3 opacity-60" : "space-y-3"}>
          <Alert
            type="warning"
            icon={<Mail className="h-5 w-5" aria-hidden="true" />}
            title={`${familiesLabel(impact.families)} ${impact.families === 1 ? "bekommt" : "bekommen"} nach dem ${action} eine E-Mail`}
            message={impactMessage(impact)}
            announce="off"
          />
          <HolidayClosureWeekList
            weeks={impact.weeks}
            schoolName={schoolName}
          />
          <HolidayClosureConsequences
            keepsContribution={impact.keepsContribution}
          />
        </div>
      ) : null}
    </section>
  );
}
