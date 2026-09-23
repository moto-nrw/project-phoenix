// Kinderkontingent (#3567/#3568): the contracted maximum number of children
// of a school, booked in whole bundles. The bounds mirror the backend's
// contract model (organizationtenancy.MaxChildQuotaBundles/BundleSize), so a
// value the form accepts is one the server accepts.

export const CHILD_QUOTA_DEFAULT_BUNDLE_SIZE = 50;
const CHILD_QUOTA_MAX = 1000;
const WHOLE_NUMBER_RE = /^\d+$/;

const WHOLE_NUMBER_ERROR = `Bitte geben Sie eine ganze Zahl von 1 bis ${CHILD_QUOTA_MAX} ein.`;

export type ChildQuotaDraftResult =
  | {
      readonly ok: true;
      readonly bundles: number;
      readonly bundleSize: number;
      readonly limit: number;
    }
  | {
      readonly ok: false;
      readonly field: "bundles" | "bundleSize";
      readonly error: string;
    };

function parseBound(value: string): number | null {
  const trimmed = value.trim();
  if (!WHOLE_NUMBER_RE.test(trimmed)) return null;
  const parsed = Number(trimmed);
  return parsed >= 1 && parsed <= CHILD_QUOTA_MAX ? parsed : null;
}

/** Validates the form input: only whole bundles of a whole bundle size. */
export function validateChildQuotaDraft(draft: {
  readonly bundles: string;
  readonly bundleSize: string;
}): ChildQuotaDraftResult {
  const bundles = parseBound(draft.bundles);
  if (bundles === null) {
    return { ok: false, field: "bundles", error: WHOLE_NUMBER_ERROR };
  }
  const bundleSize = parseBound(draft.bundleSize);
  if (bundleSize === null) {
    return { ok: false, field: "bundleSize", error: WHOLE_NUMBER_ERROR };
  }
  return { ok: true, bundles, bundleSize, limit: bundles * bundleSize };
}

/** The Kinderkontingent in children; null when the school has none. */
export function childQuotaLimit(quota: {
  readonly bundles: number | null;
  readonly bundleSize: number;
}): number | null {
  return quota.bundles === null ? null : quota.bundles * quota.bundleSize;
}

/**
 * The warning shown when a Kinderkontingent is lowered below the current
 * Kontingentzahl. Saving stays possible: the contract applies even when the
 * school has not cleaned up its data yet.
 */
export function childQuotaWarning(limit: number, count: number): string | null {
  if (count <= limit) return null;
  return `${count} Kinder zählen, Kontingent ${limit}. Speichern ist trotzdem möglich. Danach kann die Schule keine weiteren Kinder aufnehmen, bis weniger als ${limit} Kinder zählen.`;
}
