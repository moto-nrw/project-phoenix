/**
 * Das reine Datenmodell der Anfragenliste (#2267): aus den flachen Zeilen des
 * Servers werden Fälle pro Kind, aus den Fällen drei Abschnitte, aus den
 * Widerspruchs-Schlüsseln des Backends Gruppen. Bewusst ohne React — dieses
 * Modell entscheidet, was jemand sieht, und muss direkt prüfbar sein.
 */

import type { CareWithdrawalCompletion } from "~/lib/care-exit-api";
import type {
  AggregatedHistoryRequest,
  AggregatedOpenRequest,
  RequestReviewMetadata,
} from "~/lib/change-request-list-api";
import {
  conflictGroupLabel,
  conflictStaffValueInput,
  REQUEST_TYPE_LABELS,
  type StaffValueInput,
} from "./request-copy";

export type AnyItem = AggregatedOpenRequest | AggregatedHistoryRequest;

/** Eine offene Anfrage einer der vier Arten, also mit Prüf-Metadaten. */
export type ReviewItem = Exclude<
  AggregatedOpenRequest,
  { request_type: "enrollment" }
>;

export interface ConflictGroup {
  readonly key: string;
  readonly label: string;
  /** Die Anfragen dieser Gruppe, die bereits geladen sind. */
  readonly items: readonly ReviewItem[];
  /**
   * Wie viele Anfragen des Kindes das Backend auf diesem Schlüssel zählt. Kann
   * größer sein als `items.length`, wenn eine davon noch auf einer nicht
   * geladenen Seite liegt.
   */
  readonly expectedCount: number;
  /** Sind alle Beteiligten geladen? Nur dann darf ein Ergebnis fallen. */
  readonly complete: boolean;
  /** Womit die OGS statt eines Wunsches einen eigenen Wert einträgt. */
  readonly staffValueInput: StaffValueInput;
}

export interface OpenCase {
  readonly key: string;
  readonly studentID?: string;
  readonly studentName: string;
  groupName?: string;
  urgentToday: boolean;
  familyProtected?: boolean;
  readonly items: AggregatedOpenRequest[];
  readonly withdrawals: CareWithdrawalCompletion[];
  /** Alle Anfragen dieses Kindes betreffen nur vergangene Tage. */
  past: boolean;
  conflicts: ConflictGroup[];
}

export function itemKey(item: AnyItem): string {
  return `${item.request_type}:${item.data.id}`;
}

export function hasReviewMetadata(
  item: AggregatedOpenRequest,
): item is ReviewItem {
  return "student_id" in item;
}

function isPast(item: AggregatedOpenRequest): boolean {
  return hasReviewMetadata(item) && item.past === true;
}

function addOpenCase(
  cases: Map<string, OpenCase>,
  item: AggregatedOpenRequest,
  key: string,
  studentName: string,
  metadata?: Partial<RequestReviewMetadata>,
) {
  const existing = cases.get(key);
  if (existing) {
    existing.items.push(item);
    existing.urgentToday ||= metadata?.urgent_today === true;
    if (metadata?.group_name) existing.groupName = metadata.group_name;
    if (metadata?.family_protected === true) existing.familyProtected = true;
    return;
  }
  cases.set(key, {
    key,
    studentID: key.startsWith("request:") ? undefined : key,
    studentName,
    groupName: metadata?.group_name,
    urgentToday: metadata?.urgent_today === true,
    familyProtected: metadata?.family_protected,
    items: [item],
    withdrawals: [],
    past: false,
    conflicts: [],
  });
}

function addEnrollmentCases(
  cases: Map<string, OpenCase>,
  item: Extract<AggregatedOpenRequest, { request_type: "enrollment" }>,
) {
  if (item.data.children?.length) {
    item.data.children.forEach((child) =>
      addOpenCase(
        cases,
        item,
        child.student_id ?? `request:enrollment-child:${child.case_id}`,
        child.name,
      ),
    );
    return;
  }
  const childIDs = item.data.child_ids ?? [];
  if (childIDs.length) {
    childIDs.forEach((id, index) =>
      addOpenCase(cases, item, id, item.data.child_names[index] ?? "Anmeldung"),
    );
    return;
  }
  addOpenCase(
    cases,
    item,
    `request:enrollment:${item.data.id}`,
    item.data.child_names?.join(", ") || "Anmeldung",
  );
}

function addOpenItem(
  cases: Map<string, OpenCase>,
  item: AggregatedOpenRequest,
) {
  const wire = item as AggregatedOpenRequest & Partial<RequestReviewMetadata>;
  if (typeof wire.student_id === "string") {
    addOpenCase(
      cases,
      item,
      wire.student_id,
      wire.student_name ?? "Anfrage",
      wire,
    );
  } else if (item.request_type === "enrollment") {
    addEnrollmentCases(cases, item);
  } else {
    addOpenCase(
      cases,
      item,
      `request:${item.request_type}:${item.data.id}`,
      "Anfrage",
    );
  }
}

function addWithdrawalCase(
  cases: Map<string, OpenCase>,
  row: CareWithdrawalCompletion,
) {
  const existing = cases.get(row.studentId);
  if (existing) {
    existing.withdrawals.push(row);
    existing.urgentToday ||= row.urgency === "overdue";
    return;
  }
  cases.set(row.studentId, {
    key: row.studentId,
    studentID: row.studentId,
    studentName: `${row.firstName} ${row.lastName}`.trim(),
    urgentToday: row.urgency === "overdue",
    items: [],
    withdrawals: [row],
    past: false,
    conflicts: [],
  });
}

/**
 * Baut die Widerspruchsgruppen eines Falls aus `conflict_key` des Backends.
 * Ob überhaupt ein Widerspruch vorliegt, entscheidet allein
 * `conflict_group_size` — das Backend zählt über ALLE offenen Anfragen des
 * Kindes, auch über die, die diese Seite noch nicht geladen hat. Eine Gruppe
 * mit nur einem geladenen Wunsch ist deshalb kein Widerspruch weniger,
 * sondern eine unvollständige Sicht darauf; sie wird angezeigt, aber noch
 * nicht entschieden. Hier wird nichts mehr selbst hergeleitet.
 */
export function buildConflictGroups(
  items: readonly AggregatedOpenRequest[],
): ConflictGroup[] {
  const byKey = new Map<string, { items: ReviewItem[]; expected: number }>();
  for (const item of items) {
    if (!hasReviewMetadata(item)) continue;
    const key = item.conflict_key;
    const size = item.conflict_group_size ?? 0;
    if (!key || size < 2) continue;
    const bucket = byKey.get(key);
    if (bucket) {
      bucket.items.push(item);
      bucket.expected = Math.max(bucket.expected, size);
    } else {
      byKey.set(key, { items: [item], expected: size });
    }
  }
  return [...byKey.entries()]
    .map(([key, bucket]) => ({
      key,
      label: conflictGroupLabel(key),
      items: bucket.items,
      expectedCount: bucket.expected,
      complete: bucket.items.length >= bucket.expected,
      staffValueInput: conflictStaffValueInput(key),
    }))
    .sort((left, right) => left.key.localeCompare(right.key));
}

/** Schlüssel aller Anfragen, die in einer offenen Widerspruchsgruppe stecken. */
export function conflictedItemKeys(
  conflicts: readonly ConflictGroup[],
): ReadonlySet<string> {
  const keys = new Set<string>();
  for (const group of conflicts) {
    for (const item of group.items) keys.add(itemKey(item));
  }
  return keys;
}

export function groupOpenCases(
  items: readonly AnyItem[],
  withdrawals: readonly CareWithdrawalCompletion[],
): OpenCase[] {
  const cases = new Map<string, OpenCase>();
  items.forEach((item) => addOpenItem(cases, item as AggregatedOpenRequest));
  withdrawals.forEach((row) => addWithdrawalCase(cases, row));
  for (const childCase of cases.values()) {
    childCase.conflicts = buildConflictGroups(childCase.items);
    childCase.past =
      childCase.withdrawals.length === 0 &&
      childCase.items.length > 0 &&
      childCase.items.every(isPast);
  }
  return [...cases.values()].sort((left, right) => {
    if (left.past !== right.past) return left.past ? 1 : -1;
    if (left.urgentToday !== right.urgentToday)
      return left.urgentToday ? -1 : 1;
    const byTime = (right.items[0]?.occurred_at ?? "").localeCompare(
      left.items[0]?.occurred_at ?? "",
    );
    return byTime || left.studentName.localeCompare(right.studentName, "de");
  });
}

export interface CaseBuckets {
  readonly urgent: OpenCase[];
  readonly later: OpenCase[];
  readonly expired: OpenCase[];
}

/**
 * Die drei Abschnitte der Liste. „Heute wichtig" zählt nur, solange die
 * Anfrage noch etwas ändern kann: ein abgelaufener Fall steht immer unten,
 * auch wenn sein letzter Tag heute war.
 */
export function bucketCases(cases: readonly OpenCase[]): CaseBuckets {
  return {
    urgent: cases.filter((c) => c.urgentToday && !c.past),
    later: cases.filter((c) => !c.urgentToday && !c.past),
    expired: cases.filter((c) => c.past),
  };
}

function requestDates(item: AggregatedOpenRequest): readonly string[] {
  if (item.request_type === "excused") return item.data.dates ?? [];
  if (item.request_type === "offering") return [item.data.effective_from];
  return [];
}

export function caseDates(childCase: OpenCase): string[] {
  const dates = childCase.items.flatMap(requestDates);
  dates.push(...childCase.withdrawals.map((row) => row.firstBookinglessDay));
  return [...new Set(dates)].sort();
}

export function caseTypeLabels(childCase: OpenCase): string[] {
  const labels = [
    ...new Set(
      childCase.items.map((item) => REQUEST_TYPE_LABELS[item.request_type]),
    ),
  ];
  if (childCase.withdrawals.length > 0) labels.push("Abmeldung");
  return labels;
}

/**
 * Wie viele Anfragen dieser Fall in der Arbeitsliste bedeutet. Abgelaufene
 * Anfragen zählen nicht mit: sie fordern keine Entscheidung mehr.
 */
export function openRequestCount(childCase: OpenCase): number {
  return (
    childCase.items.filter((item) => !isPast(item)).length +
    childCase.withdrawals.length
  );
}

export function pastRequestCount(childCase: OpenCase): number {
  return childCase.items.filter(isPast).length;
}
