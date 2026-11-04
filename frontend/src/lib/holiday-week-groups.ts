/**
 * Weeks of a holiday period that share a day (#5689). The same days can run at
 * several schools; a child takes part in only one of those weeks, so a group
 * has one choice. Weeks joined only through a third week also form one group:
 * a rare plan, and the simpler choice stays correct for the child.
 */

interface DatedWeek {
  readonly id?: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly timeModels: readonly { readonly id?: string }[];
}

type Bookings = Readonly<Record<string, string>>;

export function groupParallelWeeks<T extends DatedWeek>(
  weeks: readonly T[],
): T[][] {
  const sorted = [...weeks].sort((a, b) =>
    a.startsOn.localeCompare(b.startsOn),
  );
  const groups: T[][] = [];
  let groupEnd = "";
  for (const week of sorted) {
    const current = groups.at(-1);
    if (current && week.startsOn <= groupEnd) {
      current.push(week);
      if (week.endsOn > groupEnd) groupEnd = week.endsOn;
    } else {
      groups.push([week]);
      groupEnd = week.endsOn;
    }
  }
  return groups;
}

/** Whether all weeks of a group run on exactly the same days. */
export function sharesDates(group: readonly DatedWeek[]): boolean {
  return group.every(
    (week) =>
      week.startsOn === group[0]?.startsOn && week.endsOn === group[0]?.endsOn,
  );
}

/** The first and last day of a group. */
export function weekGroupDates(group: readonly DatedWeek[]): {
  readonly startsOn: string;
  readonly endsOn: string;
} {
  const ends = group.map((week) => week.endsOn).sort();
  return { startsOn: group[0]?.startsOn ?? "", endsOn: ends.at(-1) ?? "" };
}

/** The time model booked in the group, "" when none is. */
export function weekGroupBooking(
  bookings: Bookings,
  group: readonly DatedWeek[],
): string {
  for (const week of group) {
    const booked = week.id ? bookings[week.id] : "";
    if (booked) return booked;
  }
  return "";
}

/** The bookings with timeModelId as the only choice of the group. */
export function withWeekGroupBooking(
  bookings: Bookings,
  group: readonly DatedWeek[],
  timeModelId: string,
): Record<string, string> {
  const next = { ...bookings };
  for (const week of group) {
    if (week.id) delete next[week.id];
  }
  const chosen = group.find((week) =>
    week.timeModels.some((model) => model.id === timeModelId),
  );
  if (chosen?.id) next[chosen.id] = timeModelId;
  return next;
}
