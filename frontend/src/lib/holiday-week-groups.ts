/**
 * Weeks of a holiday period at several schools (#5689). Weeks on exactly the
 * same days form one choice: a child takes part at one school. Weeks that
 * share only some days stay separate choices, so a booking never disappears
 * from view; bookedWeeksOverlap names that case.
 */

interface DatedWeek {
  readonly id?: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly timeModels: readonly { readonly id?: string }[];
}

type Bookings = Readonly<Record<string, string>>;

/** The weeks by their days, in the order of the first week of each group. */
export function groupParallelWeeks<T extends DatedWeek>(
  weeks: readonly T[],
): T[][] {
  const groups = new Map<string, T[]>();
  for (const week of weeks) {
    const days = `${week.startsOn}/${week.endsOn}`;
    const group = groups.get(days);
    if (group) group.push(week);
    else groups.set(days, [week]);
  }
  return [...groups.values()];
}

/** Whether two of the booked weeks share a day. */
export function bookedWeeksOverlap(
  weeks: readonly DatedWeek[],
  bookedWeekIds: readonly string[],
): boolean {
  const booked = weeks.filter(
    (week) => week.id && bookedWeekIds.includes(week.id),
  );
  return booked.some((week, index) =>
    booked
      .slice(0, index)
      .some(
        (other) =>
          week.startsOn <= other.endsOn && other.startsOn <= week.endsOn,
      ),
  );
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
