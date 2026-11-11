"use client";

// Die Betreuungstage einer tageweise gebuchten Ferienwoche im Editor (#6447):
// Montag bis Freitag in einer Reihe. Ein Feiertag oder Schließtag der
// ausrichtenden OGS ist grau und durchgestrichen; Eltern können ihn nicht
// buchen, die anderen Tage schon. Reine Anzeige, nichts davon ist klickbar.

import { CalendarOff } from "lucide-react";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionLabel } from "~/components/ui/section-label";
import {
  holidayCareDays,
  holidayDayDate,
  holidayWeekdayShort,
} from "~/lib/holiday-days";
import type { HolidayNoCareDay } from "~/lib/traeger-api";
import { cn } from "~/lib/utils";

function noCareKind(day: HolidayNoCareDay): string {
  return day.kind === "holiday" ? "Feiertag" : "Schließtag";
}

/** "Mi 06.01. Feiertag (Heilige Drei Könige)" für jeden Tag ohne Betreuung. */
export function noCareDayList(days: readonly HolidayNoCareDay[]): string {
  return days
    .map(
      (day) =>
        `${holidayWeekdayShort(day.date)} ${holidayDayDate(day.date)} ${noCareKind(day)} (${day.name})`,
    )
    .join(", ");
}

/** Die Tage der Woche, an denen Eltern buchen können. */
export function bookableHolidayDays(
  startsOn: string,
  endsOn: string,
  noCareDays: readonly HolidayNoCareDay[],
): string[] {
  return holidayCareDays(startsOn, endsOn).filter(
    (day) => !noCareDays.some((none) => none.date === day),
  );
}

export function HolidayWeekCareDays({
  startsOn,
  endsOn,
  noCareDays,
  labelId,
}: Readonly<{
  startsOn: string;
  endsOn: string;
  noCareDays: readonly HolidayNoCareDay[];
  labelId: string;
}>) {
  const days = holidayCareDays(startsOn, endsOn);
  if (days.length === 0) return null;
  const bookable = bookableHolidayDays(startsOn, endsOn, noCareDays).length;
  return (
    <div className="space-y-2 pt-2">
      <SectionLabel as="p" id={labelId}>
        Buchbare Tage · {bookable} von {days.length}
      </SectionLabel>
      <ul
        aria-labelledby={labelId}
        className="grid max-w-xl grid-cols-5 gap-1.5 sm:gap-2"
      >
        {days.map((day) => {
          const none = noCareDays.find((d) => d.date === day);
          const weekday = holidayWeekdayShort(day);
          const date = holidayDayDate(day);
          return (
            <InsetPanel
              as="li"
              key={day}
              tone={none ? "muted" : "white"}
              dashed={Boolean(none)}
              padding="compact"
              aria-label={
                none
                  ? `${weekday} ${date}: keine Betreuung, ${noCareKind(none)} (${none.name})`
                  : `${weekday} ${date}: buchbar`
              }
              title={none ? `${noCareKind(none)}: ${none.name}` : undefined}
              className="flex min-w-0 flex-col items-center px-1 text-center"
            >
              <span
                className={cn(
                  "text-xs font-semibold",
                  none ? "text-gray-400" : "text-gray-500",
                )}
              >
                {weekday}
              </span>
              <span
                className={cn(
                  "text-sm font-medium tabular-nums",
                  none ? "text-gray-400 line-through" : "text-gray-900",
                )}
              >
                {date}
              </span>
              <span
                className={cn(
                  "mt-0.5 flex max-w-full items-center gap-1 text-xs",
                  none ? "font-medium text-gray-500" : "text-gray-400",
                )}
              >
                {none ? (
                  <CalendarOff
                    className="h-3 w-3 shrink-0"
                    aria-hidden="true"
                  />
                ) : null}
                <span className="truncate">
                  {none ? noCareKind(none) : "buchbar"}
                </span>
              </span>
            </InsetPanel>
          );
        })}
      </ul>
    </div>
  );
}
