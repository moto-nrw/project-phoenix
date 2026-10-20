// lib/meal-caterer-api.ts
// The school's caterer (ADR 0075): who supplies the school, its invitations,
// a new invitation, taking one back, and ending the supply. The invitation
// link reaches the caterer by mail only; the school never sees it.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";

type MealCatererInvitationStatus = "open" | "accepted" | "revoked" | "expired";

export interface MealCatererInvitation {
  id: string;
  email: string;
  catererName: string;
  deliveryAddress: string;
  status: MealCatererInvitationStatus;
  expiresAt: string;
  createdAt: string;
}

interface MealCatererSupply {
  catererName: string;
  deliveryAddress: string;
  startedAt: string;
}

export interface MealCaterer {
  supply: MealCatererSupply | null;
  invitations: MealCatererInvitation[];
}

export interface MealCatererInvitationInput {
  email: string;
  catererName: string;
  deliveryAddress: string;
}

interface BackendInvitation {
  id: string;
  email: string;
  caterer_name: string;
  delivery_address: string;
  status: MealCatererInvitationStatus;
  expires_at: string;
  created_at: string;
}

interface BackendMealCaterer {
  supply: {
    caterer_name: string;
    delivery_address: string;
    started_at: string;
  } | null;
  invitations: BackendInvitation[];
}

function unwrap<T>(body: unknown): T {
  if (body && typeof body === "object" && "data" in body) {
    return (body as { data: T }).data;
  }
  return body as T;
}

function mapInvitation(invitation: BackendInvitation): MealCatererInvitation {
  return {
    id: invitation.id,
    email: invitation.email,
    catererName: invitation.caterer_name,
    deliveryAddress: invitation.delivery_address,
    status: invitation.status,
    expiresAt: invitation.expires_at,
    createdAt: invitation.created_at,
  };
}

export async function getMealCaterer(): Promise<MealCaterer> {
  const response = await sessionFetch("/api/meal-caterer", {
    credentials: "include",
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal caterer load failed");
  }
  const data = unwrap<BackendMealCaterer>(await response.json());
  return {
    supply: data.supply
      ? {
          catererName: data.supply.caterer_name,
          deliveryAddress: data.supply.delivery_address,
          startedAt: data.supply.started_at,
        }
      : null,
    invitations: (data.invitations ?? []).map(mapInvitation),
  };
}

export async function inviteMealCaterer(
  input: MealCatererInvitationInput,
): Promise<MealCatererInvitation> {
  const response = await sessionFetch("/api/meal-caterer/invitations", {
    method: "POST",
    credentials: "include",
    body: JSON.stringify({
      email: input.email.trim(),
      caterer_name: input.catererName.trim(),
      delivery_address: input.deliveryAddress.trim(),
    }),
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal caterer invite failed");
  }
  return mapInvitation(unwrap<BackendInvitation>(await response.json()));
}

export async function revokeMealCatererInvitation(id: string): Promise<void> {
  const response = await sessionFetch(
    `/api/meal-caterer/invitations/${encodeURIComponent(id)}`,
    { method: "DELETE", credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal caterer revoke failed");
  }
}

export async function endMealCatererSupply(): Promise<void> {
  const response = await sessionFetch("/api/meal-caterer/supply", {
    method: "DELETE",
    credentials: "include",
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "meal caterer end failed");
  }
}
