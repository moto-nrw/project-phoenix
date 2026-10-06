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
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
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
  const response = await transportFetch(
    "/api/operator/auth/invitations/session",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Invitation session failed (${response.status})`,
    );
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
  const response = await transportFetch(
    "/api/operator/auth/invitations/validate",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        [OPERATOR_INVITATION_FLOW_HEADER]: flowID,
      },
      body: JSON.stringify({}),
    },
  );

  if (!response.ok) {
    // The operator backend answers in the shared error envelope (#2507).
    throw await apiErrorFromResponse(
      response,
      `Invitation validation failed (${response.status})`,
    );
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
  const response = await transportFetch(
    "/api/operator/auth/invitations/accept",
    {
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
    },
  );

  if (!response.ok) {
    const error = await apiErrorFromResponse(
      response,
      `Invitation accept failed (${response.status})`,
    );
    logger.warn("accept_invitation_failed", {
      status: response.status,
      code: error.code,
    });
    throw error;
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
