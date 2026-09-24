/**
 * Kinderkontingent der eigenen Schule für die Datenverwaltung (#3569). Die
 * Zahlen tragen die Namen der 409-Details (students.child_quota_reached).
 */

/** Kinderkontingent (booked) und Kontingentzahl (occupied) der Schule. */
export interface ChildQuota {
  readonly booked: number;
  readonly occupied: number;
}

interface ChildQuotaResponse {
  readonly limited?: boolean;
  readonly booked_places?: number;
  readonly occupied_places?: number;
}

/** Wandelt die Backend-Antwort; ohne Kinderkontingent null. */
function mapChildQuota(
  response: ChildQuotaResponse | undefined,
): ChildQuota | null {
  if (
    response?.limited !== true ||
    typeof response.booked_places !== "number" ||
    typeof response.occupied_places !== "number"
  ) {
    return null;
  }
  return {
    booked: response.booked_places,
    occupied: response.occupied_places,
  };
}

export async function fetchChildQuota(): Promise<ChildQuota | null> {
  const response = await fetch("/api/students/child-quota", {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`Child quota failed: ${response.status}`);
  }
  const json = (await response.json()) as { data?: ChildQuotaResponse };
  return mapChildQuota(json.data);
}
