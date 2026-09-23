/**
 * Fehlercode, wenn das Kinderkontingent einer OGS erreicht ist (#3567). Der
 * Code ist die Identität des Fehlers; der Backend-Text ist nur Diagnose.
 */
export const CHILD_QUOTA_REACHED_CODE = "students.child_quota_reached";

interface ChildQuotaDetails {
  booked_places?: unknown;
  occupied_places?: unknown;
}

interface CodedError {
  code?: unknown;
  details?: unknown;
  body?: unknown;
}

/** Liest Code und Details aus einem geworfenen API-Fehler. */
function readCodedError(
  err: unknown,
): { code: string; details: ChildQuotaDetails } | null {
  if (typeof err !== "object" || err === null) return null;
  const coded = err as CodedError;
  if (typeof coded.code === "string") {
    return {
      code: coded.code,
      details: (coded.details ?? {}) as ChildQuotaDetails,
    };
  }
  // fetchWithAuth trägt die rohe Antwort als `body` mit.
  if (typeof coded.body === "string") {
    try {
      const parsed = JSON.parse(coded.body) as CodedError;
      if (typeof parsed.code === "string") {
        return {
          code: parsed.code,
          details: (parsed.details ?? {}) as ChildQuotaDetails,
        };
      }
    } catch {
      return null;
    }
  }
  return null;
}

/**
 * Meldung für ein erreichtes Kinderkontingent, sonst null. Die Zahlen kommen
 * aus den Details der Antwort; fehlen sie, bleibt der Satz ohne Zahlen.
 */
export function childQuotaMessage(err: unknown): string | null {
  const coded = readCodedError(err);
  if (coded?.code !== CHILD_QUOTA_REACHED_CODE) return null;
  const { booked_places: booked, occupied_places: occupied } = coded.details;
  const stand =
    typeof booked === "number" && typeof occupied === "number"
      ? ` Die Kontingentzahl beträgt ${occupied} von ${booked} Kindern.`
      : "";
  return `Das Kinderkontingent Ihrer Schule ist voll.${stand} Für weitere Kinder melden Sie sich bitte beim moto-Team.`;
}
