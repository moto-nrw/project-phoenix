// Kinder in der Familie (#5918): Eltern geben im Vertrag an, wie viele
// Kinder ihre Familie hat, auch Geschwister ohne Betreuungsvertrag. Die
// Abrechnung des Trägers bestätigt die Zahl; erst dann zählt sie für die
// Geschwisterermäßigung.

import { apiErrorFromResponse, transportFetch } from "./api-error";

export interface ParentFamilyChildren {
  readonly statedChildren: number | null;
  /** Instant of the statement, RFC 3339. */
  readonly statedAt: string | null;
  readonly confirmedChildren: number | null;
  readonly confirmedAt: string | null;
  readonly validFrom: string | null;
  readonly validUntil: string | null;
  /** The account may state a number on this contract. */
  readonly canState: boolean;
}

interface BackendFamilyChildren {
  stated_children: number | null;
  stated_at: string | null;
  confirmed_children: number | null;
  confirmed_at: string | null;
  valid_from: string | null;
  valid_until: string | null;
  can_state: boolean;
}

const path = (organizationId: string, contractId: string) =>
  `/api/parent/me/documents/${encodeURIComponent(organizationId)}/contracts/${encodeURIComponent(contractId)}/family-children`;

async function request(
  url: string,
  init: RequestInit,
): Promise<ParentFamilyChildren> {
  const response = await transportFetch(url, {
    ...init,
    cache: "no-store",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Family children failed: ${response.status}`,
    );
  }
  const { data } = (await response.json()) as { data: BackendFamilyChildren };
  return {
    statedChildren: data.stated_children,
    statedAt: data.stated_at,
    confirmedChildren: data.confirmed_children,
    confirmedAt: data.confirmed_at,
    validFrom: data.valid_from,
    validUntil: data.valid_until,
    canState: data.can_state,
  };
}

/** The number of the family on a contract of the inbox. */
export function fetchFamilyChildren(
  organizationId: string,
  contractId: string,
): Promise<ParentFamilyChildren> {
  return request(path(organizationId, contractId), { method: "GET" });
}

/** The parents state the number of their children. */
export function stateFamilyChildren(
  organizationId: string,
  contractId: string,
  children: number,
): Promise<ParentFamilyChildren> {
  return request(path(organizationId, contractId), {
    method: "PUT",
    body: JSON.stringify({ children }),
  });
}
