/**
 * Weekday conflicts of the per-day selection rule (#4514). Offerings of a
 * selection group with `at_most_one_per_day` may be combined, but a child
 * may not hold two of them on the same weekday. Mirrors `checkDayRule` in
 * backend/modules/careplan/selection/day_rule.go, which enforces the rule on
 * every write; this copy only tells the family in advance.
 */

export const AT_MOST_ONE_PER_DAY = "at_most_one_per_day";

export interface DayRuleOffering {
  readonly id: string;
  readonly days_of_week_mode: string;
  readonly available_days: readonly string[];
  readonly selection_group?: string | null;
  readonly selection_rule?: string | null;
}

export interface CareOfferingDayConflict {
  readonly group: string;
  readonly day: string;
  /** The conflicting offerings in catalog order. */
  readonly offeringIds: readonly string[];
}

const WEEKDAY_ORDER = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];

/**
 * Every weekday on which the selection holds more than one offering of a
 * per-day group. A fixed offering covers all its days; a parent_choice
 * offering the days in `daysByOffering`, automatic days included.
 */
export function careOfferingDayConflicts(
  offerings: readonly DayRuleOffering[],
  selectedIds: ReadonlySet<string>,
  daysByOffering: Readonly<Record<string, Iterable<string> | undefined>>,
): CareOfferingDayConflict[] {
  const holders = new Map<string, Map<string, string[]>>();
  for (const offering of offerings) {
    const group = offering.selection_group?.trim() ?? "";
    if (
      group === "" ||
      offering.selection_rule !== AT_MOST_ONE_PER_DAY ||
      !selectedIds.has(offering.id)
    ) {
      continue;
    }
    const days =
      offering.days_of_week_mode === "fixed"
        ? offering.available_days
        : (daysByOffering[offering.id] ?? []);
    const byDay = holders.get(group) ?? new Map<string, string[]>();
    holders.set(group, byDay);
    for (const day of new Set(days)) {
      byDay.set(day, [...(byDay.get(day) ?? []), offering.id]);
    }
  }
  const conflicts: CareOfferingDayConflict[] = [];
  for (const [group, byDay] of holders) {
    for (const day of WEEKDAY_ORDER) {
      const ids = byDay.get(day);
      if (ids && ids.length > 1) {
        conflicts.push({ group, day, offeringIds: ids });
      }
    }
  }
  return conflicts;
}

/**
 * Joins offering names for a conflict message: quoted the way `locale`
 * quotes, then listed with its conjunction („A“ und „B“).
 */
export function conflictOfferingList(
  names: readonly string[],
  locale: string,
): string {
  const [open, close] = QUOTES[locale] ?? ["“", "”"];
  const quoted = names.map((name) => `${open}${name}${close}`);
  return new Intl.ListFormat(locale, { type: "conjunction" }).format(quoted);
}

const QUOTES: Record<string, readonly [string, string]> = {
  de: ["„", "“"],
  pl: ["„", "”"],
  ru: ["«", "»"],
  uk: ["«", "»"],
};
