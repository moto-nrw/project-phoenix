/**
 * Pseudonymous user ID of the usage analytics (#3603).
 *
 * Only an OGS account of a school with Analyse-Freigabe is recognized across
 * sessions, and only under this ID: a hash of school and account, never the
 * account ID itself. The backend computes the same value
 * (`backend/analytics/pseudonym.go`), so the browser's page views and
 * recordings and the backend's core actions meet in one profile. The same
 * account at another school is another person.
 */

/** Starts every pseudonymous ID; the filter recognizes them by it. */
export const PSEUDONYM_PREFIX = "pseudo_";

/** Whether an ID has the shape of a pseudonymous ID. */
export function isPseudonym(value: unknown): value is string {
  return (
    typeof value === "string" &&
    new RegExp(`^${PSEUDONYM_PREFIX}[0-9a-f]{32}$`).test(value)
  );
}

/**
 * The pseudonymous ID of an account at a school, or null when either ID is
 * not numeric or the browser offers no Web Crypto (an insecure origin).
 */
export async function analyticsPseudonym(
  schoolId: string,
  accountId: string,
): Promise<string | null> {
  if (!/^\d+$/.test(schoolId) || !/^\d+$/.test(accountId)) return null;
  const subtle = globalThis.crypto?.subtle;
  if (!subtle) return null;

  const digest = await subtle.digest(
    "SHA-256",
    new TextEncoder().encode(`moto-analytics:v1:${schoolId}:${accountId}`),
  );
  const hex = Array.from(new Uint8Array(digest).slice(0, 16), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
  return `${PSEUDONYM_PREFIX}${hex}`;
}
