// grade-transition-api.test.ts
// Mapper tests for the Jahrgangsstufenwechsel admin API client (#405)

import { describe, it, expect } from "vitest";
import {
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
