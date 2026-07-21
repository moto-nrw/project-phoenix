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
import { OPERATOR_INVITATION_FLOW_HEADER } from "./operator-invitation-flow";

const logger = createLogger({ component: "OperatorInvitationApi" });

export interface OperatorInvitationsData {
  invitations: PendingOperatorInvitation[];
  operators: OperatorInfo[];
}

// --- Authenticated API functions ---

export async function createOperatorInvitation(
  data: CreateOperatorInvitationRequest,
): Promise<void> {
  // Convert camelCase frontend shape to the snake_case the backend expects.
  await operatorFetch<unknown>("/api/operator/invitations", {
    method: "POST",
    body: {
      email: data.email,
      display_name: data.displayName,
    },
  });
}

export async function listOperatorInvitations(): Promise<OperatorInvitationsData> {
  const raw = await operatorFetch<BackendInvitationsListResponse>(
    "/api/operator/invitations",
  );
  return {
    invitations: (raw.invitations ?? []).map(mapPendingInvitation),
    operators: (raw.operators ?? []).map(mapOperatorInfo),
  };
}

export async function resendOperatorInvitation(id: string): Promise<void> {
  await operatorFetch<unknown>(
    `/api/operator/invitations/${encodeURIComponent(id)}/resend`,
    { method: "POST" },
  );
}

export async function revokeOperatorInvitation(id: string): Promise<void> {
  await operatorFetch<unknown>(
    `/api/operator/invitations/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

// --- Public (unauthenticated) API functions ---

export async function establishOperatorInvitationSession(
  token: string,
): Promise<string> {
  const response = await fetch("/api/operator/auth/invitations/session", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  if (!response.ok) {
    throw new Error("Einladungstoken konnte nicht geschützt werden");
  }
  const body: unknown = await response.json();
  if (
    typeof body !== "object" ||
    body === null ||
    !("flow_id" in body) ||
    typeof body.flow_id !== "string" ||
    body.flow_id === ""
  ) {
    throw new Error("Einladungssitzung ist ungültig");
  }
  return body.flow_id;
}

export async function validateOperatorInvitation(
  flowID: string,
): Promise<OperatorInvitationValidation> {
  const response = await fetch("/api/operator/auth/invitations/validate", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      [OPERATOR_INVITATION_FLOW_HEADER]: flowID,
    },
    body: JSON.stringify({}),
  });

  if (!response.ok) {
    let message = "Einladung nicht gefunden oder abgelaufen";
    try {
      // Operator backend serializes errors as { status, message } via
      // ErrResponse json:"message" — matching api-helpers.ts, read both keys
      // so the helper survives any intermediate layer that uses `error`.
      const data = (await response.json()) as {
        message?: string;
        error?: string;
      };
      message = data.message ?? data.error ?? message;
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
  flowID: string,
  data: AcceptOperatorInvitationRequest,
): Promise<void> {
  // Convert camelCase frontend shape to the snake_case the backend expects.
  const response = await fetch("/api/operator/auth/invitations/accept", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      [OPERATOR_INVITATION_FLOW_HEADER]: flowID,
    },
    body: JSON.stringify({
      display_name: data.displayName,
      password: data.password,
      confirm_password: data.confirmPassword,
    }),
  });

  if (!response.ok) {
    let message = "Einladung konnte nicht angenommen werden";
    try {
      // Dual-read matches api-helpers.ts — operator backend natively emits
      // { message } but the helper tolerates { error } from intermediate layers.
      const errorData = (await response.json()) as {
        message?: string;
        error?: string;
      };
      message = errorData.message ?? errorData.error ?? message;
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
