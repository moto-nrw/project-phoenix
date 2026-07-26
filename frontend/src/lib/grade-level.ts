const MIN_SUPPORTED_GRADE_LEVEL = 1;
const MAX_SUPPORTED_GRADE_LEVEL = 13;

/**
 * Guards tenant/API grade caps before they reach form constraints. Missing,
 * fractional, or unsupported values are configuration errors; callers should
 * surface an unavailable state instead of silently falling back to an OGS
 * default that may reject otherwise valid submissions.
 */
export function isSupportedGradeLevelMax(value: unknown): value is number {
  return (
    typeof value === "number" &&
    Number.isInteger(value) &&
    value >= MIN_SUPPORTED_GRADE_LEVEL &&
    value <= MAX_SUPPORTED_GRADE_LEVEL
  );
}
