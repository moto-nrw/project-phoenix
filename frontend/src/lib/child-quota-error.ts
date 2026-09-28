import { readCodedApiError } from "./coded-api-error";

/**
 * Fehlercode, wenn das Kinderkontingent einer OGS erreicht ist (#3567). Der
 * Code ist die Identität des Fehlers; der Backend-Text ist nur Diagnose.
 */
export const CHILD_QUOTA_REACHED_CODE = "students.child_quota_reached";

/**
 * Meldung für ein erreichtes Kinderkontingent, sonst null. Die Zahlen kommen
 * aus den Details der Antwort; fehlen sie, bleibt der Satz ohne Zahlen.
 */
export function childQuotaMessage(err: unknown): string | null {
  const coded = readCodedApiError(err);
  if (coded?.code !== CHILD_QUOTA_REACHED_CODE) return null;
  const { booked_places: booked, occupied_places: occupied } = coded.details;
  const stand =
    typeof booked === "number" && typeof occupied === "number"
      ? ` Die Kontingentzahl beträgt ${occupied} von ${booked} Kindern.`
      : "";
  return `Das Kinderkontingent Ihrer Schule ist voll.${stand} Für weitere Kinder melden Sie sich bitte beim moto-Team.`;
}
