import { describe, expect, it } from "vitest";

import type { AggregatedOpenRequest } from "~/lib/change-request-list-api";
import {
  bucketCases,
  buildConflictGroups,
  conflictedItemKeys,
  groupOpenCases,
  openRequestCount,
  pastRequestCount,
} from "./case-model";

function item(
  id: string,
  overrides: Record<string, unknown> = {},
): AggregatedOpenRequest {
  return {
    request_type: "excused",
    occurred_at: "2026-08-29T09:00:00Z",
    student_id: "10",
    student_name: "Mia Muster",
    expected_version: `v${id}`,
    urgent_today: false,
    bulk_eligible: true,
    family_protected: false,
    data: { id, dates: ["2026-08-29"], absence_status: "sick" },
    ...overrides,
  } as never;
}

describe("case-model", () => {
  it("bündelt Anfragen pro Kind", () => {
    const cases = groupOpenCases([item("1"), item("2")], []);
    expect(cases).toHaveLength(1);
    expect(openRequestCount(cases[0]!)).toBe(2);
  });

  it("sortiert einen abgelaufenen Fall nach Abgelaufen, auch wenn er heute betrifft", () => {
    const cases = groupOpenCases(
      [item("1", { past: true, urgent_today: true })],
      [],
    );
    const buckets = bucketCases(cases);
    expect(buckets.urgent).toHaveLength(0);
    expect(buckets.expired).toHaveLength(1);
    expect(pastRequestCount(buckets.expired[0]!)).toBe(1);
    // Eine abgelaufene Anfrage fordert keine Entscheidung mehr.
    expect(openRequestCount(buckets.expired[0]!)).toBe(0);
  });

  it("hält einen Fall offen, solange eine Anfrage noch wirkt", () => {
    const cases = groupOpenCases([item("1", { past: true }), item("2")], []);
    expect(bucketCases(cases).expired).toHaveLength(0);
    expect(bucketCases(cases).later).toHaveLength(1);
  });

  it("bildet eine Widerspruchsgruppe allein nach der Gruppengröße des Backends", () => {
    const single = buildConflictGroups([
      item("1", { conflict_key: "absence:2026-08-29", conflict_group_size: 1 }),
      item("2", { conflict_key: "absence:2026-08-30", conflict_group_size: 1 }),
    ]);
    expect(single).toHaveLength(0);

    const groups = buildConflictGroups([
      item("1", { conflict_key: "absence:2026-08-29", conflict_group_size: 2 }),
      item("2", { conflict_key: "absence:2026-08-29", conflict_group_size: 2 }),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0]!.label).toBe("Abwesenheit am 29.08.2026");
    // Bei einer Abwesenheit gibt es keinen freien Wert zum Eintragen.
    expect(groups[0]!.staffValueInput).toBe("status");
    expect(groups[0]!.complete).toBe(true);
    expect(conflictedItemKeys(groups).size).toBe(2);
  });

  it("merkt sich, dass ein Beteiligter noch nicht geladen ist", () => {
    // Das Backend zählt über alle offenen Anfragen des Kindes, auch über die
    // auf einer noch nicht geladenen Seite.
    const groups = buildConflictGroups([
      item("1", { conflict_key: "absence:2026-08-29", conflict_group_size: 3 }),
      item("2", { conflict_key: "absence:2026-08-29", conflict_group_size: 3 }),
    ]);
    expect(groups[0]!.expectedCount).toBe(3);
    expect(groups[0]!.items).toHaveLength(2);
    expect(groups[0]!.complete).toBe(false);
  });

  it("erlaubt einen eigenen Wert nur bei Zeiten", () => {
    const groups = buildConflictGroups([
      item("1", {
        request_type: "care_schedule",
        conflict_key: "care:1:pickup",
        conflict_group_size: 2,
        data: { id: "1", diff: [] },
      }),
      item("2", {
        request_type: "care_schedule",
        conflict_key: "care:1:pickup",
        conflict_group_size: 2,
        data: { id: "2", diff: [] },
      }),
    ]);
    expect(groups[0]!.staffValueInput).toBe("time");
    expect(groups[0]!.label).toBe("Betreuungszeit am Montag");
  });
});
