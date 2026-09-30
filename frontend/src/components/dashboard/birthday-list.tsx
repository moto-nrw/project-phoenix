"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import { parseISODate } from "~/lib/date-helpers";
import type { BirthdayCelebration } from "~/lib/birthdays-api";

/**
 * The birthdays of one week, grouped by day (#1542, #3777).
 *
 * Deliberately a list of names rather than a counter: "2 Geburtstage" tells
 * nobody who to congratulate, which is the entire purpose of the card.
 *
 * Every day states weekday AND date on the left ("Mo, 28.09."), so a row
 * never leaves the reader counting which Monday is meant. Today's day carries
 * a green "Heute" badge and a green tint, so it stays recognisable at a glance
 * in the middle of the week. A colleague's row is marked "Team" instead of a
 * group: children and staff share the day, but never the same description.
 *
 * What may appear here is decided in the backend (school settings + personal
 * opt-out); this component renders whatever it is handed, in the order it is
 * handed (calendar order, children before staff on the same day).
 */
export function BirthdayList({
  celebrations,
  today,
  isLoading,
  emptyTitle,
}: {
  readonly celebrations: readonly BirthdayCelebration[];
  /** Today as "YYYY-MM-DD"; decides "wird" or "wurde" for the age. */
  readonly today: string;
  readonly isLoading: boolean;
  readonly emptyTitle: string;
}) {
  if (isLoading) {
    return (
      <div className="space-y-2" aria-hidden="true">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-8 rounded-lg" />
        ))}
      </div>
    );
  }

  // Derselbe Leerzustand wie in den Nachbarkarten der Startseite.
  if (celebrations.length === 0) {
    return <EmptyState className="py-4" title={emptyTitle} />;
  }

  return (
    <ul className="space-y-1">
      {groupByDay(celebrations).map(({ date, isToday, entries }) => (
        <li
          key={date}
          className={`flex gap-3 rounded-lg px-2 py-1.5 ${
            isToday ? "bg-moto-green/10" : ""
          }`}
        >
          <div className="flex h-5 w-20 shrink-0 items-center sm:w-24">
            {isToday ? (
              <StatusBadge
                label="Heute"
                tone="green"
                compact
                title={dayLabel(date)}
                accessibleLabel={`Heute, ${dayLabel(date)}`}
              />
            ) : (
              <span className="text-sm text-gray-600 tabular-nums">
                {dayLabel(date)}
              </span>
            )}
          </div>
          <ul className="min-w-0 flex-1 space-y-1">
            {entries.map((celebration) => (
              <li
                key={`${celebration.kind}-${celebration.id}`}
                // Der Name hat Vorrang. Auf dem Handy rutschen Gruppe, Klasse
                // und Alter darunter (die Karte wächst dort mit). In der festen
                // Rasterzelle ab sm bleibt es eine Zeile, gekürzt wird dann
                // zuerst die Beschreibung.
                className="flex min-w-0 flex-wrap items-baseline gap-x-2 sm:flex-nowrap"
              >
                <span className="max-w-full truncate text-sm font-medium text-gray-900 sm:shrink-0">
                  {celebration.name}
                </span>
                <span className="max-w-full min-w-0 truncate text-xs text-gray-500">
                  {describeCelebration(celebration, today)}
                </span>
              </li>
            ))}
          </ul>
        </li>
      ))}
    </ul>
  );
}

interface BirthdayDay {
  readonly date: string;
  readonly isToday: boolean;
  readonly entries: readonly BirthdayCelebration[];
}

/** Consecutive entries of the same day, in the order they arrive. */
function groupByDay(
  celebrations: readonly BirthdayCelebration[],
): BirthdayDay[] {
  const days: {
    date: string;
    isToday: boolean;
    entries: BirthdayCelebration[];
  }[] = [];
  for (const celebration of celebrations) {
    const last = days.at(-1);
    if (last?.date === celebration.date) {
      last.entries.push(celebration);
    } else {
      days.push({
        date: celebration.date,
        isToday: celebration.isToday,
        entries: [celebration],
      });
    }
  }
  return days;
}

/**
 * The secondary text: where the child belongs and which birthday it is. A
 * colleague's row says "Team" and nothing else: publishing a colleague's age
 * is exactly what the personal opt-out exists to prevent.
 */
function describeCelebration(
  celebration: BirthdayCelebration,
  today: string,
): string {
  if (celebration.kind === "staff") return "Team";

  const parts: string[] = [];
  if (celebration.groupName) parts.push(celebration.groupName);
  // The stored class already reads like a label ("Klasse 1a" as well as "1a"
  // depending on the school), so it is printed verbatim — prefixing "Klasse"
  // produced "Klasse Klasse 1a" on real data.
  if (celebration.schoolClass) parts.push(celebration.schoolClass);
  if (celebration.age && celebration.age > 0) {
    // ISO dates compare as strings. A birthday earlier this week is over.
    const verb = celebration.date < today ? "wurde" : "wird";
    parts.push(`${verb} ${celebration.age}`);
  }
  return parts.join(" · ");
}

const SHORT_WEEKDAYS = ["So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"] as const;

/** "Mo, 28.09.": the weekday alone would leave the reader counting. */
function dayLabel(isoDate: string): string {
  const weekday = SHORT_WEEKDAYS[parseISODate(isoDate).getDay()];
  return weekday
    ? `${weekday}, ${dayMonthLabel(isoDate)}`
    : dayMonthLabel(isoDate);
}

/** "28.09." */
export function dayMonthLabel(isoDate: string): string {
  const date = parseISODate(isoDate);
  return `${String(date.getDate()).padStart(2, "0")}.${String(date.getMonth() + 1).padStart(2, "0")}.`;
}
