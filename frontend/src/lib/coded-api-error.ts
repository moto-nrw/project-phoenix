/**
 * Code und Details eines geworfenen API-Fehlers (ADR 0006). Der Code ist die
 * Identität des Fehlers; der Backend-Text ist nur Diagnose.
 */
export interface CodedApiError {
  code: string;
  details: Record<string, unknown>;
}

interface CodedErrorShape {
  code?: unknown;
  details?: unknown;
  body?: unknown;
}

function detailsObject(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null
    ? (value as Record<string, unknown>)
    : {};
}

/**
 * Liest Code und Details aus einem geworfenen API-Fehler: direkt vom Fehler
 * (`code`, `details`) oder aus der rohen Antwort in `body`, die
 * fetchWithAuth mitträgt.
 */
export function readCodedApiError(err: unknown): CodedApiError | null {
  if (typeof err !== "object" || err === null) return null;
  const coded = err as CodedErrorShape;
  if (typeof coded.code === "string") {
    return { code: coded.code, details: detailsObject(coded.details) };
  }
  if (typeof coded.body === "string") {
    try {
      const parsed = JSON.parse(coded.body) as CodedErrorShape;
      if (typeof parsed.code === "string") {
        return { code: parsed.code, details: detailsObject(parsed.details) };
      }
    } catch {
      return null;
    }
  }
  return null;
}
