import { operatorFetch } from "./api-helpers";
import type {
  BackendInvitationsListResponse,
  BackendInvitationValidation,
  CreateOperatorInvitationRequest,
  AcceptOperatorInvitationRequest,
  PendingOperatorInvitation,
  OperatorInfo,
  OperatorInvitationValidation,
} from "./operator-invitation-helpers";
import {
  mapPendingInvitation,
  mapOperatorInfo,
  mapInvitationValidation,
} from "./operator-invitation-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInvitationApi" });

export interface OperatorInvitationsData {
  invitations: PendingOperatorInvitation[];
  operators: OperatorInfo[];
}

class OperatorInvitationService {
  async createInvitation(data: CreateOperatorInvitationRequest): Promise<void> {
    await operatorFetch<unknown>("/api/operator/invitations", {
      method: "POST",
      body: data,
    });
  }

  async listInvitations(): Promise<OperatorInvitationsData> {
    const raw = await operatorFetch<BackendInvitationsListResponse>(
      "/api/operator/invitations",
    );
    return {
      invitations: (raw.invitations ?? []).map(mapPendingInvitation),
      operators: (raw.operators ?? []).map(mapOperatorInfo),
    };
  }

  async resendInvitation(id: string): Promise<void> {
    await operatorFetch<unknown>(
      `/api/operator/invitations/${encodeURIComponent(id)}/resend`,
      { method: "POST" },
    );
  }

  async revokeInvitation(id: string): Promise<void> {
    await operatorFetch<unknown>(
      `/api/operator/invitations/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
  }
}

export const operatorInvitationService = new OperatorInvitationService();

// --- Public (unauthenticated) API functions ---

export async function validateOperatorInvitation(
  token: string,
): Promise<OperatorInvitationValidation> {
  const response = await fetch("/api/operator/auth/invitations/validate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });

  if (!response.ok) {
    let message = "Einladung nicht gefunden oder abgelaufen";
    try {
      const data = (await response.json()) as { message?: string };
      if (data.message) message = data.message;
    } catch {
      // use default
    }
    throw new Error(message);
  }

  const json: unknown = await response.json();
  const data = unwrapResponse<BackendInvitationValidation>(json);
  return mapInvitationValidation(data);
}

export async function acceptOperatorInvitation(
  token: string,
  data: AcceptOperatorInvitationRequest,
): Promise<void> {
  const response = await fetch("/api/operator/auth/invitations/accept", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token, ...data }),
  });

  if (!response.ok) {
    let message = "Einladung konnte nicht angenommen werden";
    try {
      const errorData = (await response.json()) as { message?: string };
      if (errorData.message) message = errorData.message;
    } catch {
      // use default
    }
    logger.error("accept_invitation_failed", {
      status: response.status,
      error: message,
    });
    throw new Error(message);
  }
}

function unwrapResponse<T>(json: unknown): T {
  if (
    typeof json === "object" &&
    json !== null &&
    "data" in json &&
    "status" in json
  ) {
    return (json as { data: T }).data;
  }
  return json as T;
}
