const MIN_SUPPORTED_GRADE_LEVEL = 1;
/** Highest grade the backend accepts anywhere (form schema, offerings, classes). */
export const MAX_SUPPORTED_GRADE_LEVEL = 13;

/**
 * Guards tenant/API grade caps before they reach form constraints. Missing,
 * fractional, or unsupported values are configuration errors; callers should
 * surface an unavailable state instead of silently falling back to an OGS
 * default that may reject otherwise valid submissions.
 */
export function isSupportedGradeLevelMax(value: unknown): value is number {
  return isSupportedGradeLevel(value);
}

/** A whole grade from 1 to the highest grade the backend accepts. */
export function isSupportedGradeLevel(value: unknown): value is number {
  return (
    typeof value === "number" &&
    Number.isInteger(value) &&
    value >= MIN_SUPPORTED_GRADE_LEVEL &&
    value <= MAX_SUPPORTED_GRADE_LEVEL
  );
}

/** The tenant's grade cap when it is usable, otherwise null. */
export function supportedGradeLevelMax(value: unknown): number | null {
  return isSupportedGradeLevelMax(value) ? value : null;
}

/** „3. Klasse“: a grade level, not a named school class. */
export function gradeLevelLabel(gradeLevel: number | string): string {
  return `${gradeLevel}. Klasse`;
}
