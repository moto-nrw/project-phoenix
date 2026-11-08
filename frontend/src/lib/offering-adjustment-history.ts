import type { AdminOfferingAdjustment } from "~/lib/enrollment-admin-api";
import { WEEKDAY_CODES, weekdayShortList } from "~/lib/weekdays";

export type OfferingAdjustmentSnapshot =
  AdminOfferingAdjustment["before"][number];

export interface OfferingAdjustmentChangeLine {
  /** Formatted weekdays ("Di, Mi"), or null for a change without known days. */
  readonly days: string | null;
  readonly text: string;
}

const WEEKDAY_ORDER: readonly string[] = WEEKDAY_CODES;
const GERMAN_LIST = new Intl.ListFormat("de", { type: "conjunction" });

// describeOfferingAdjustmentChanges turns the before/after audit snapshots of
// one offering adjustment into one line per group of weekdays that changed the
// same way (#3688). A snapshot row without determinable days does not enter the
// weekday comparison; an offering that only such rows carry and that appears on
// one side only is listed at the end as booked or cancelled.
export function describeOfferingAdjustmentChanges(
  before: readonly OfferingAdjustmentSnapshot[],
  after: readonly OfferingAdjustmentSnapshot[],
): OfferingAdjustmentChangeLine[] {
  const beforeByDay = offeringsByDay(before);
  const afterByDay = offeringsByDay(after);
  // The after snapshot names an offering as it is called now.
  const names = new Map<string, string>();
  for (const row of [...before, ...after]) {
    names.set(row.offering_id, snapshotName(row));
  }

  const groups = new Map<
    string,
    { days: string[]; removed: string[]; added: string[] }
  >();
  const allDays = new Set([...beforeByDay.keys(), ...afterByDay.keys()]);
  for (const day of [...allDays].sort(compareWeekdays)) {
    const was = beforeByDay.get(day) ?? [];
    const is = afterByDay.get(day) ?? [];
    const removed = was.filter((id) => !is.includes(id));
    const added = is.filter((id) => !was.includes(id));
    if (removed.length === 0 && added.length === 0) continue;
    const key = JSON.stringify([removed, added]);
    const group = groups.get(key) ?? { days: [], removed, added };
    group.days.push(day);
    groups.set(key, group);
  }

  const nameList = (ids: readonly string[]) =>
    GERMAN_LIST.format(ids.map((id) => names.get(id) ?? `Angebot #${id}`));
  const lines: OfferingAdjustmentChangeLine[] = [...groups.values()].map(
    ({ days, removed, added }) => ({
      days: weekdayShortList(days),
      text: describeChange(nameList(removed), nameList(added)),
    }),
  );

  const undatedRemoved = undatedOnlyIn(before, after);
  const undatedAdded = undatedOnlyIn(after, before);
  if (undatedRemoved.length > 0) {
    lines.push({ days: null, text: `${nameList(undatedRemoved)} abgemeldet` });
  }
  if (undatedAdded.length > 0) {
    lines.push({ days: null, text: `${nameList(undatedAdded)} gebucht` });
  }

  return lines.length > 0
    ? lines
    : [{ days: null, text: "Angebote unverändert" }];
}

function describeChange(removed: string, added: string): string {
  if (removed && added) return `vorher ${removed}, nachher ${added}`;
  if (added) return `${added} gebucht`;
  return `${removed} abgemeldet`;
}

// snapshotDays mirrors the backend baseline rule: a fixed offering without an
// explicit selection covers all its available days.
function snapshotDays(row: OfferingAdjustmentSnapshot): readonly string[] {
  if ((row.selected_days?.length ?? 0) > 0) return row.selected_days ?? [];
  if (row.days_of_week_mode === "fixed") return row.available_days ?? [];
  return [];
}

function offeringsByDay(
  rows: readonly OfferingAdjustmentSnapshot[],
): Map<string, string[]> {
  const byDay = new Map<string, string[]>();
  for (const row of rows) {
    for (const day of snapshotDays(row)) {
      const ids = byDay.get(day) ?? [];
      if (!ids.includes(row.offering_id)) ids.push(row.offering_id);
      byDay.set(day, ids);
    }
  }
  return byDay;
}

// undatedOnlyIn returns the ids of offerings that `rows` carries only without
// determinable days and that `other` does not carry at all.
function undatedOnlyIn(
  rows: readonly OfferingAdjustmentSnapshot[],
  other: readonly OfferingAdjustmentSnapshot[],
): string[] {
  const otherIds = new Set(other.map((row) => row.offering_id));
  const datedIds = new Set(
    rows
      .filter((row) => snapshotDays(row).length > 0)
      .map((row) => row.offering_id),
  );
  const ids = rows
    .map((row) => row.offering_id)
    .filter((id) => !datedIds.has(id) && !otherIds.has(id));
  return [...new Set(ids)];
}

function compareWeekdays(a: string, b: string): number {
  const rank = (day: string) => {
    const index = WEEKDAY_ORDER.indexOf(day);
    return index === -1 ? WEEKDAY_ORDER.length : index;
  };
  return rank(a) - rank(b);
}

function snapshotName(row: OfferingAdjustmentSnapshot): string {
  return row.offering_name || `Angebot #${row.offering_id}`;
}
