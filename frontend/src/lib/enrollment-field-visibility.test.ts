import { describe, it, expect } from "vitest";
import {
  buildFieldsByKey,
  isFieldVisible,
  visibleAnswerData,
  type ConditionContext,
} from "./enrollment-field-visibility";
import type { FormField } from "./enrollment-form-schema-api";

function field(partial: Partial<FormField> & { key: string }): FormField {
  return {
    label: partial.key,
    type: "text",
    sort_order: 0,
    ...partial,
  };
}

function ctxFrom(
  fields: FormField[],
  extra: Partial<ConditionContext> = {},
): ConditionContext {
  return {
    guardianAnswers: {},
    fieldsByKey: buildFieldsByKey(fields),
    ...extra,
  };
}

describe("isFieldVisible", () => {
  it("shows fields without a condition", () => {
    const f = field({ key: "notes" });
    expect(isFieldVisible(f, ctxFrom([f]))).toBe(true);
  });

  it("evaluates a boolean field condition (eq true)", () => {
    const controller = field({ key: "has_allergy", type: "boolean" });
    const dependent = field({
      key: "which",
      visible_when: {
        source: "field",
        field: "has_allergy",
        operator: "eq",
        value: true,
      },
    });
    const fields = [controller, dependent];

    expect(
      isFieldVisible(dependent, ctxFrom(fields, { guardianAnswers: {} })),
    ).toBe(false);
    expect(
      isFieldVisible(
        dependent,
        ctxFrom(fields, { guardianAnswers: { has_allergy: true } }),
      ),
    ).toBe(true);
  });

  it("evaluates a select field condition (eq value)", () => {
    const controller = field({
      key: "pickup",
      type: "select",
      options: [
        { label: "Alleine", value: "alone" },
        { label: "Abgeholt", value: "picked_up" },
      ],
    });
    const dependent = field({
      key: "pickup_note",
      visible_when: {
        source: "field",
        field: "pickup",
        operator: "eq",
        value: "picked_up",
      },
    });
    const fields = [controller, dependent];

    expect(
      isFieldVisible(
        dependent,
        ctxFrom(fields, { guardianAnswers: { pickup: "alone" } }),
      ),
    ).toBe(false);
    expect(
      isFieldVisible(
        dependent,
        ctxFrom(fields, { guardianAnswers: { pickup: "picked_up" } }),
      ),
    ).toBe(true);
  });

  it("supports neq and not_empty", () => {
    const controller = field({ key: "mode", type: "select" });
    const neq = field({
      key: "a",
      visible_when: {
        source: "field",
        field: "mode",
        operator: "neq",
        value: "x",
      },
    });
    const notEmpty = field({
      key: "b",
      visible_when: { source: "field", field: "mode", operator: "not_empty" },
    });
    const fields = [controller, neq, notEmpty];

    expect(
      isFieldVisible(neq, ctxFrom(fields, { guardianAnswers: { mode: "x" } })),
    ).toBe(false);
    expect(
      isFieldVisible(neq, ctxFrom(fields, { guardianAnswers: { mode: "y" } })),
    ).toBe(true);
    expect(
      isFieldVisible(notEmpty, ctxFrom(fields, { guardianAnswers: {} })),
    ).toBe(false);
    expect(
      isFieldVisible(
        notEmpty,
        ctxFrom(fields, { guardianAnswers: { mode: "y" } }),
      ),
    ).toBe(true);
  });

  it("reads a child-scope controller from childAnswers", () => {
    const controller = field({
      key: "child_flag",
      type: "boolean",
      applies_to_child: true,
    });
    const dependent = field({
      key: "child_detail",
      applies_to_child: true,
      visible_when: {
        source: "field",
        field: "child_flag",
        operator: "eq",
        value: true,
      },
    });
    const fields = [controller, dependent];

    expect(
      isFieldVisible(
        dependent,
        ctxFrom(fields, { childAnswers: { child_flag: true } }),
      ),
    ).toBe(true);
    expect(
      isFieldVisible(dependent, ctxFrom(fields, { childAnswers: {} })),
    ).toBe(false);
  });

  it("evaluates a grade-level condition", () => {
    const dependent = field({
      key: "grade_note",
      applies_to_child: true,
      visible_when: { source: "grade_level", operator: "eq", value: 1 },
    });
    expect(
      isFieldVisible(dependent, ctxFrom([dependent], { gradeLevel: "1" })),
    ).toBe(true);
    expect(
      isFieldVisible(dependent, ctxFrom([dependent], { gradeLevel: "2" })),
    ).toBe(false);
  });

  it("matches care offerings by name, case-insensitively", () => {
    const dependent = field({
      key: "lunch_note",
      applies_to_child: true,
      visible_when: {
        source: "care_offering",
        operator: "includes",
        value: "Mittagessen",
      },
    });
    expect(
      isFieldVisible(
        dependent,
        ctxFrom([dependent], { offeringNames: new Set(["mittagessen"]) }),
      ),
    ).toBe(true);
    expect(
      isFieldVisible(
        dependent,
        ctxFrom([dependent], { offeringNames: new Set(["regelbetreuung"]) }),
      ),
    ).toBe(false);
  });

  it("hides a chained field when its controller is itself hidden", () => {
    // has_extra (boolean) controls A; A (select) controls B. When has_extra
    // is false, A is hidden — even if A still holds a stale "x" answer, B must
    // collapse with its hidden controller.
    const hasExtra = field({ key: "has_extra", type: "boolean" });
    const a = field({
      key: "a",
      type: "select",
      visible_when: {
        source: "field",
        field: "has_extra",
        operator: "eq",
        value: true,
      },
    });
    const b = field({
      key: "b",
      visible_when: { source: "field", field: "a", operator: "eq", value: "x" },
    });
    const fields = [hasExtra, a, b];

    // Controller visible + matching → B visible.
    expect(
      isFieldVisible(
        b,
        ctxFrom(fields, { guardianAnswers: { has_extra: true, a: "x" } }),
      ),
    ).toBe(true);

    // has_extra=false hides A; A's stale "x" must NOT keep B visible.
    expect(
      isFieldVisible(
        b,
        ctxFrom(fields, { guardianAnswers: { has_extra: false, a: "x" } }),
      ),
    ).toBe(false);
  });

  it("hides a chained neq field when its controller is hidden", () => {
    // The neq path is the dangerous one: a hidden controller must NOT make a
    // `neq` dependent visible via "undefined !== expected". B uses neq on A;
    // A is hidden when has_extra=false → B must be hidden, not visible.
    const hasExtra = field({ key: "has_extra", type: "boolean" });
    const a = field({
      key: "a",
      type: "select",
      visible_when: {
        source: "field",
        field: "has_extra",
        operator: "eq",
        value: true,
      },
    });
    const b = field({
      key: "b",
      visible_when: {
        source: "field",
        field: "a",
        operator: "neq",
        value: "x",
      },
    });
    const fields = [hasExtra, a, b];

    // A visible + a != "x" → B visible.
    expect(
      isFieldVisible(
        b,
        ctxFrom(fields, { guardianAnswers: { has_extra: true, a: "y" } }),
      ),
    ).toBe(true);

    // has_extra=false hides A → B must collapse to hidden despite neq.
    expect(
      isFieldVisible(
        b,
        ctxFrom(fields, { guardianAnswers: { has_extra: false, a: "y" } }),
      ),
    ).toBe(false);
  });

  it("treats a cyclic visibility configuration as hidden", () => {
    // A depends on B, B depends on A — a misconfiguration. Must not recurse
    // forever; both resolve to hidden.
    const a = field({
      key: "a",
      type: "select",
      visible_when: { source: "field", field: "b", operator: "eq", value: "x" },
    });
    const b = field({
      key: "b",
      type: "select",
      visible_when: { source: "field", field: "a", operator: "eq", value: "x" },
    });
    const fields = [a, b];
    expect(
      isFieldVisible(a, ctxFrom(fields, { guardianAnswers: { a: "x", b: "x" } })),
    ).toBe(false);
  });
});

describe("visibleAnswerData", () => {
  it("drops hidden fields, info fields, and other-scope fields", () => {
    const controller = field({ key: "has_allergy", type: "boolean" });
    const visible = field({
      key: "which",
      visible_when: {
        source: "field",
        field: "has_allergy",
        operator: "eq",
        value: true,
      },
    });
    const info = field({
      key: "infotext_1",
      type: "information",
      content: "Hi",
    });
    const childField = field({ key: "child_only", applies_to_child: true });
    const fields = [controller, visible, info, childField];

    const values = {
      has_allergy: true,
      which: "Nüsse",
      infotext_1: "should never be set",
      child_only: "ignored at guardian scope",
    };

    const out = visibleAnswerData(fields, false, values, {
      guardianAnswers: values,
      fieldsByKey: buildFieldsByKey(fields),
    });

    expect(out).toEqual({ has_allergy: true, which: "Nüsse" });
  });

  it("excludes a value once its controlling answer hides it", () => {
    const controller = field({ key: "has_allergy", type: "boolean" });
    const dependent = field({
      key: "which",
      visible_when: {
        source: "field",
        field: "has_allergy",
        operator: "eq",
        value: true,
      },
    });
    const fields = [controller, dependent];
    const values = { has_allergy: false, which: "stale value" };

    const out = visibleAnswerData(fields, false, values, {
      guardianAnswers: values,
      fieldsByKey: buildFieldsByKey(fields),
    });

    expect(out).toEqual({ has_allergy: false });
  });
});
