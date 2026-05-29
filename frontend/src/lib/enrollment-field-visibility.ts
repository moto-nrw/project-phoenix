import type {
  FormField,
  VisibilityCondition,
} from "~/lib/enrollment-form-schema-api";

/**
 * Evaluation context for a field's visibility condition. The renderer
 * builds one of these per scope: guardian-level fields see only the
 * guardian answers; per-child fields additionally see that child's
 * answers, grade level, and selected care offerings.
 *
 * Mirrors the backend evaluation semantics in
 * models/enrollment/form_schema.go (VisibilityCondition) + the
 * guardian-level skip in services/enrollment/form_schema_service.go.
 */
export interface ConditionContext {
  /** Guardian-level custom answers (request-level custom_data). */
  guardianAnswers: Record<string, unknown>;
  /** Per-child custom answers; present only when evaluating a child field. */
  childAnswers?: Record<string, unknown>;
  /** The child's chosen grade level (string from the select); child only. */
  gradeLevel?: string;
  /**
   * The child's selected care offering names, lowercased. Care-offering
   * conditions match by name (not id) because form templates are not
   * bound to a phase and offering ids differ per phase. Child only.
   */
  offeringNames?: Set<string>;
  /**
   * All schema fields keyed by `key`. Used to resolve whether a
   * controlling "field" condition points at a guardian- or child-level
   * field, so its answer is read from the right map.
   */
  fieldsByKey: Map<string, FormField>;
}

/** Indexes schema fields by key for condition resolution. */
export function buildFieldsByKey(
  fields: readonly FormField[],
): Map<string, FormField> {
  const map = new Map<string, FormField>();
  for (const field of fields) map.set(field.key, field);
  return map;
}

// Loose equality across the JSON round-trip: a boolean controlling field
// yields `true`/`false`, a select yields a string, the grade level a
// numeric string. Comparing string forms covers all three.
function looseEquals(a: unknown, b: unknown): boolean {
  return String(a ?? "") === String(b ?? "");
}

/** Reports whether a field should be shown given the current answers. */
export function isFieldVisible(
  field: FormField,
  ctx: ConditionContext,
): boolean {
  const condition = field.visible_when;
  if (!condition) return true;
  return evaluateCondition(condition, ctx);
}

function evaluateCondition(
  condition: VisibilityCondition,
  ctx: ConditionContext,
): boolean {
  switch (condition.source) {
    case "field":
      return evaluateFieldSource(condition, ctx);
    case "grade_level":
      return matchScalar(
        condition.operator,
        ctx.gradeLevel ?? "",
        condition.value,
      );
    case "care_offering": {
      const names = ctx.offeringNames ?? new Set<string>();
      return names.has(
        String(condition.value ?? "")
          .trim()
          .toLowerCase(),
      );
    }
    default:
      return true;
  }
}

function evaluateFieldSource(
  condition: VisibilityCondition,
  ctx: ConditionContext,
): boolean {
  const key = condition.field ?? "";
  const controller = ctx.fieldsByKey.get(key);
  const answers = controller?.applies_to_child
    ? (ctx.childAnswers ?? {})
    : ctx.guardianAnswers;
  return matchScalar(condition.operator, answers[key], condition.value);
}

function matchScalar(
  operator: VisibilityCondition["operator"],
  actual: unknown,
  expected: unknown,
): boolean {
  switch (operator) {
    case "not_empty":
      return actual !== undefined && actual !== null && actual !== "";
    case "eq":
      return looseEquals(actual, expected);
    case "neq":
      return !looseEquals(actual, expected);
    default:
      // "includes" is handled per-source (care_offering); anything else
      // defaults to visible rather than hiding the field unexpectedly.
      return true;
  }
}

/**
 * Returns a new map containing only the visible, answer-collecting
 * (non-information) field values for the given scope. Used at submit
 * time so a hidden field's stale value is never sent to the backend.
 */
export function visibleAnswerData(
  fields: readonly FormField[],
  appliesToChild: boolean,
  values: Record<string, unknown>,
  ctx: ConditionContext,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const field of fields) {
    if ((field.applies_to_child ?? false) !== appliesToChild) continue;
    if (field.type === "information") continue;
    if (!isFieldVisible(field, ctx)) continue;
    if (field.key in values) out[field.key] = values[field.key];
  }
  return out;
}
