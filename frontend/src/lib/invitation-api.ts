import { buildApiError } from "~/lib/auth-api";

import type {
  InvitationValidation,
  InvitationAcceptRequest,
  CreateInvitationRequest,
  PendingInvitation,
  BackendInvitationValidation,
  BackendInvitation,
} from "./invitation-helpers";
import {
  mapInvitationValidationResponse,
  mapPendingInvitationResponse,
} from "./invitation-helpers";

const hasDataProperty = <T>(value: unknown): value is { data: T } => {
  return typeof value === "object" && value !== null && "data" in value;
};

const extractData = <T>(payload: unknown): T => {
  if (hasDataProperty<T>(payload)) {
    return payload.data;
  }
  return payload as T;
};

export async function validateInvitation(
  token: string,
): Promise<InvitationValidation> {
  const response = await fetch(
    `/api/invitations/validate?token=${encodeURIComponent(token)}`,
  );
  if (!response.ok) {
    throw await buildApiError(
      response,
      "Einladung konnte nicht geprüft werden.",
    );
  }
  const raw = (await response.json()) as unknown;
  const data = extractData<BackendInvitationValidation>(raw);
  return mapInvitationValidationResponse(data);
}

interface AcceptInvitationResult {
  /** Subdomain of the invitation's school — tenant hosts resolve by subdomain, not slug (#1977). */
  tenantSubdomain?: string;
}

export async function acceptInvitation(
  token: string,
  data: InvitationAcceptRequest,
): Promise<AcceptInvitationResult> {
  const response = await fetch("/api/invitations/accept", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ token, ...data }),
  });

  if (!response.ok) {
    throw await buildApiError(
      response,
      "Einladung konnte nicht angenommen werden.",
    );
  }

  const raw = (await response.json()) as Record<string, unknown>;
  const result = (raw.data ?? raw) as Record<string, unknown>;
  return {
    tenantSubdomain:
      typeof result.tenant_subdomain === "string"
        ? result.tenant_subdomain
        : undefined,
  };
}

export async function createInvitation(
  data: CreateInvitationRequest,
): Promise<PendingInvitation> {
  const response = await fetch("/api/invitations", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
    credentials: "include",
  });

  if (!response.ok) {
    throw await buildApiError(
      response,
      "Einladung konnte nicht erstellt werden.",
    );
  }

  const raw = (await response.json()) as unknown;
  const invitationData = extractData<BackendInvitation>(raw);
  return mapPendingInvitationResponse(invitationData);
}

export async function listPendingInvitations(): Promise<PendingInvitation[]> {
  const response = await fetch("/api/invitations", {
    credentials: "include",
  });
  if (!response.ok) {
    throw await buildApiError(
      response,
      "Offene Einladungen konnten nicht geladen werden.",
    );
  }
  const raw = (await response.json()) as unknown;
  const extracted = extractData<BackendInvitation[] | BackendInvitation>(raw);
  if (Array.isArray(extracted)) {
    return extracted.map(mapPendingInvitationResponse);
  }
  return [mapPendingInvitationResponse(extracted)];
}

export async function resendInvitation(id: number): Promise<void> {
  const response = await fetch(`/api/invitations/${id}/resend`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw await buildApiError(
      response,
      "Einladung konnte nicht erneut gesendet werden.",
    );
  }
}

export async function revokeInvitation(id: number): Promise<void> {
  const response = await fetch(`/api/invitations/${id}`, {
    method: "DELETE",
    credentials: "include",
  });
  if (!response.ok) {
    throw await buildApiError(
      response,
      "Einladung konnte nicht widerrufen werden.",
    );
  }
}
