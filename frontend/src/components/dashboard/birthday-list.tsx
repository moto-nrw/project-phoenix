"use client";

import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { BirthdayCelebration } from "~/lib/birthdays-api";

/**
 * The dashboard birthday display (#1542).
 *
 * Deliberately a list of names rather than a counter: "2 Geburtstage" tells
 * nobody who to congratulate, which is the entire purpose of the card. Entries
 * that are not from today (the weekend a Monday carries) are labelled, so the
 * card never implies someone is celebrating right now when they are not.
 *
 * What may appear here is decided in the backend (school settings + personal
 * opt-out); this component renders whatever it is handed.
 */
export function BirthdayList({
  celebrations,
  isLoading,
}: {
  readonly celebrations: readonly BirthdayCelebration[];
  readonly isLoading: boolean;
}) {
  if (isLoading) {
    return (
      <div className="space-y-2">
        {[1, 2].map((i) => (
          <div key={i} className="h-12 animate-pulse rounded-xl bg-gray-100" />
        ))}
      </div>
    );
  }

  if (celebrations.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-gray-500">
        Heute keine Geburtstage
      </p>
    );
  }

  return (
    <ul className="grid gap-2 sm:grid-cols-2">
      {celebrations.map((celebration) => (
        <li
          key={`${celebration.kind}-${celebration.id}`}
          className="flex items-center justify-between gap-3 rounded-xl border border-gray-200/50 bg-gray-50/50 p-3"
        >
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-gray-900">
              {celebration.name}
            </p>
            <p className="truncate text-xs text-gray-500">
              {describeCelebration(celebration)}
            </p>
          </div>
          {celebration.isToday ? (
            celebration.kind === "staff" ? (
              <StatusBadge label="Team" tone="blue" />
            ) : (
              <StatusBadge label="Heute" tone="green" />
            )
          ) : (
            <StatusBadge
              label={weekdayLabel(celebration.date)}
              tone="gray"
              title={formatDate(celebration.date)}
            />
          )}
        </li>
      ))}
    </ul>
  );
}

/**
 * The secondary line: where the child belongs and which birthday it is. Staff
 * entries carry neither an age nor a group — a colleague's line stays "Team",
 * which is all the card is allowed to say about them.
 */
function describeCelebration(celebration: BirthdayCelebration): string {
  if (celebration.kind === "staff") {
    return "Mitarbeitende Person";
  }

  const parts: string[] = [];
  if (celebration.groupName) parts.push(celebration.groupName);
  // The stored class already reads like a label ("Klasse 1a" as well as "1a"
  // depending on the school), so it is printed verbatim — prefixing "Klasse"
  // produced "Klasse Klasse 1a" on real data.
  if (celebration.schoolClass) parts.push(celebration.schoolClass);
  if (celebration.age && celebration.age > 0) {
    parts.push(`wird ${celebration.age}`);
  }
  return parts.join(" · ");
}

const WEEKDAYS = [
  "Sonntag",
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
  "Samstag",
] as const;

/**
 * Names the day a past birthday fell on ("Samstag"), which is more useful on a
 * Monday morning than the bare date. Parsed as a local date, never via
 * `new Date("YYYY-MM-DD")` (that is UTC midnight and shifts the weekday).
 */
function weekdayLabel(isoDate: string): string {
  const [year, month, day] = isoDate.split("-").map(Number);
  if (!year || !month || !day) return formatDate(isoDate);
  return (
    WEEKDAYS[new Date(year, month - 1, day).getDay()] ?? formatDate(isoDate)
  );
}
