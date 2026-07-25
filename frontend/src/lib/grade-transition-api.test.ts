// grade-transition-api.test.ts
// Mapper tests for the Jahrgangsstufenwechsel admin API client (#405)

import { describe, it, expect, vi, afterEach } from "vitest";
import {
  listGradeTransitions,
  mapTransition,
  mapPreview,
  mapResult,
  mapSuggestion,
  mapHistoryEntry,
  type BackendGradeTransition,
  type BackendTransitionPreview,
  type BackendTransitionResult,
  type BackendSuggestedMapping,
  type BackendTransitionHistoryEntry,
} from "./grade-transition-api";

describe("mapTransition", () => {
  const backend: BackendGradeTransition = {
    id: 7,
    academic_year: "2026-2027",
    status: "draft",
    created_at: "2026-07-24T09:00:00Z",
    created_by: 42,
    notes: "Sommer-Wechsel",
    mappings: [
      { id: 1, from_class: "1a", to_class: "2a", action: "promote" },
      { id: 2, from_class: "4a", to_class: null, action: "graduate" },
    ],
    can_modify: true,
    can_apply: true,
    can_revert: false,
  };

  it("maps snake_case to camelCase and ids to strings", () => {
    const t = mapTransition(backend);
    expect(t.id).toBe("7");
    expect(t.academicYear).toBe("2026-2027");
    expect(t.status).toBe("draft");
    expect(t.createdAt).toBe("2026-07-24T09:00:00Z");
    expect(t.notes).toBe("Sommer-Wechsel");
    expect(t.canModify).toBe(true);
    expect(t.canApply).toBe(true);
    expect(t.canRevert).toBe(false);
    expect(t.mappings).toHaveLength(2);
    expect(t.mappings[0]).toEqual({
      id: "1",
      fromClass: "1a",
      toClass: "2a",
      action: "promote",
    });
    expect(t.mappings[1]?.toClass).toBeNull();
    expect(t.mappings[1]?.action).toBe("graduate");
  });

  it("handles missing optional fields", () => {
    const t = mapTransition({
      id: 8,
      academic_year: "2026-2027",
      status: "applied",
      created_at: "2026-07-24T09:00:00Z",
      created_by: 42,
      applied_at: "2026-08-01T06:00:00Z",
      applied_by: 42,
      can_modify: false,
      can_apply: false,
      can_revert: true,
    });
    expect(t.mappings).toEqual([]);
    expect(t.notes).toBeNull();
    expect(t.appliedAt).toBe("2026-08-01T06:00:00Z");
    expect(t.revertedAt).toBeNull();
  });
});

describe("mapPreview", () => {
  const backend: BackendTransitionPreview = {
    transition_id: 7,
    academic_year: "2026-2027",
    total_students: 50,
    to_promote: 40,
    to_graduate: 10,
    by_mapping: [
      {
        from_class: "1a",
        to_class: "2a",
        student_count: 20,
        action: "promote",
      },
      { from_class: "4a", student_count: 10, action: "graduate" },
    ],
    unmapped_classes: [{ class_name: "3c", student_count: 15 }],
    warnings: ["1 classes with students are not included in this transition"],
  };

  it("maps preview counts and nested lists", () => {
    const p = mapPreview(backend);
    expect(p.transitionId).toBe("7");
    expect(p.totalStudents).toBe(50);
    expect(p.toPromote).toBe(40);
    expect(p.toGraduate).toBe(10);
    expect(p.byMapping[0]).toEqual({
      fromClass: "1a",
      toClass: "2a",
      studentCount: 20,
      action: "promote",
    });
    expect(p.byMapping[1]?.toClass).toBeNull();
    expect(p.unmappedClasses).toEqual([{ className: "3c", studentCount: 15 }]);
    expect(p.warnings).toHaveLength(1);
  });
});

describe("mapResult", () => {
  const backend: BackendTransitionResult = {
    transition_id: 7,
    status: "applied",
    students_promoted: 40,
    students_graduated: 10,
    can_revert: true,
    warnings: [],
  };

  it("maps the apply/revert result", () => {
    const r = mapResult(backend);
    expect(r.transitionId).toBe("7");
    expect(r.status).toBe("applied");
    expect(r.studentsPromoted).toBe(40);
    expect(r.studentsGraduated).toBe(10);
    expect(r.canRevert).toBe(true);
    expect(r.warnings).toEqual([]);
  });
});

describe("mapSuggestion", () => {
  it("maps a promotion suggestion", () => {
    const backend: BackendSuggestedMapping = {
      from_class: "1a",
      to_class: "2a",
      student_count: 20,
      is_graduating: false,
    };
    expect(mapSuggestion(backend)).toEqual({
      fromClass: "1a",
      toClass: "2a",
      studentCount: 20,
      isGraduating: false,
    });
  });

  it("maps a graduation suggestion (no to_class)", () => {
    const backend: BackendSuggestedMapping = {
      from_class: "4b",
      student_count: 12,
      is_graduating: true,
    };
    const s = mapSuggestion(backend);
    expect(s.toClass).toBeNull();
    expect(s.isGraduating).toBe(true);
  });
});

describe("mapHistoryEntry", () => {
  it("maps a history row", () => {
    const backend: BackendTransitionHistoryEntry = {
      id: 3,
      transition_id: 7,
      student_id: 99,
      person_name: "Mika Muster",
      from_class: "4a",
      to_class: null,
      action: "graduated",
      created_at: "2026-08-01T06:00:00Z",
    };
    expect(mapHistoryEntry(backend)).toEqual({
      id: "3",
      transitionId: "7",
      studentId: "99",
      personName: "Mika Muster",
      fromClass: "4a",
      toClass: null,
      action: "graduated",
      createdAt: "2026-08-01T06:00:00Z",
    });
  });
});

describe("int64 ids on the wire", () => {
  // The backend serializes every id as a decimal STRING (`json:"id,string"`)
  // because a JSON number is a double here: 2^53+1 and 2^53+2 parse to the same
  // value, so a numeric wire format would let two distinct transitions collapse
  // onto one key and an apply/revert address a row nobody selected. The mappers
  // must therefore pass a string id through untouched — never via Number()
  // (#405 review).
  const big = "9007199254740993"; // 2^53 + 1, not representable as a double
  const bigger = "9007199254740995"; // 2^53 + 3, also rounds onto 2^53 + 4

  it("keeps a string id past 2^53 exact", () => {
    const t = mapTransition({
      id: big,
      academic_year: "2026-2027",
      status: "draft",
      created_at: "2026-07-24T09:00:00Z",
      created_by: big,
      mappings: [
        { id: bigger, from_class: "1a", to_class: "2a", action: "promote" },
      ],
      can_modify: true,
      can_apply: false,
      can_revert: false,
    });
    expect(t.id).toBe(big);
    expect(t.mappings[0]?.id).toBe(bigger);
    // The guard that would fail if anything routed the id through a double.
    expect(Number(t.id).toString()).not.toBe(t.id);
  });

  it("keeps preview, result and history ids exact", () => {
    expect(
      mapPreview({
        transition_id: big,
        academic_year: "2026-2027",
        total_students: 0,
        to_promote: 0,
        to_graduate: 0,
        by_mapping: [],
      }).transitionId,
    ).toBe(big);

    expect(
      mapResult({
        transition_id: big,
        status: "applied",
        students_promoted: 0,
        students_graduated: 0,
        can_revert: true,
      }).transitionId,
    ).toBe(big);

    const history = mapHistoryEntry({
      id: big,
      transition_id: bigger,
      student_id: big,
      person_name: "Mika Muster",
      from_class: "4a",
      action: "graduated",
      created_at: "2026-08-01T06:00:00Z",
    });
    expect(history.id).toBe(big);
    expect(history.transitionId).toBe(bigger);
    expect(history.studentId).toBe(big);
  });

  it("does not collapse two ids that share a double", async () => {
    const row = (id: string): BackendGradeTransition => ({
      id,
      academic_year: "2026-2027",
      status: "applied",
      created_at: "2026-07-24T09:00:00Z",
      created_by: 1,
      can_modify: false,
      can_apply: false,
      can_revert: true,
    });
    expect(Number(big)).toBe(Number("9007199254740992"));

    const rows = [row(big), row("9007199254740992")];
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        status: 200,
        json: async () => ({ data: rows }),
      })) as unknown as typeof fetch,
    );

    const result = await listGradeTransitions();
    expect(result.map((t) => t.id)).toEqual([big, "9007199254740992"]);
  });
});

describe("listGradeTransitions", () => {
  const makeRow = (id: number): BackendGradeTransition => ({
    id,
    academic_year: "2026-2027",
    status: "applied",
    created_at: "2026-07-24T09:00:00Z",
    created_by: 1,
    can_modify: false,
    can_apply: false,
    can_revert: true,
  });

  const page = (from: number, count: number) =>
    Array.from({ length: count }, (_, i) => makeRow(from + i));

  const stubFetch = (pages: BackendGradeTransition[][]) => {
    const urls: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn((url: string) => {
        urls.push(url);
        const requested = Number(/[?&]page=(\d+)/.exec(url)?.[1] ?? "1");
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ data: pages[requested - 1] ?? [] }),
        } as unknown as Response);
      }),
    );
    return urls;
  };

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("stops after a short page", async () => {
    const urls = stubFetch([page(1, 3)]);
    const result = await listGradeTransitions();
    expect(result.map((t) => t.id)).toEqual(["1", "2", "3"]);
    expect(urls).toHaveLength(1);
  });

  it("keeps fetching while pages come back full", async () => {
    // 100 + 100 + 1: the last applied transition sits on page three and would be
    // invisible to a single-page fetch — the revert dialog would then target a
    // stale row and 409 forever (#405 review).
    const urls = stubFetch([page(1, 100), page(101, 100), page(201, 1)]);
    const result = await listGradeTransitions();
    expect(result).toHaveLength(201);
    expect(result.at(-1)?.id).toBe("201");
    expect(urls).toHaveLength(3);
  });

  it("dedupes rows that shift across page boundaries", async () => {
    // A concurrent insert pushes rows down under created_at DESC, so the same id
    // can appear on two pages.
    const urls = stubFetch([page(1, 100), [makeRow(100), ...page(101, 2)]]);
    const result = await listGradeTransitions();
    expect(result).toHaveLength(102);
    expect(new Set(result.map((t) => t.id)).size).toBe(102);
    expect(urls).toHaveLength(2);
  });
});
