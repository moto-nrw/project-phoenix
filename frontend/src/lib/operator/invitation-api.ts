import { operatorFetch, OperatorApiError } from "./api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorInvitationAPI" });

// --- Types ---

export interface OperatorInvitationValidation {
  email: string;
  display_name: string | null;
  expires_at: string;
  inviter_name: string;
}

export interface PendingOperatorInvitation {
  id: number;
  email: string;
  display_name: string | null;
  invited_by: number;
  inviter_name: string;
  expires_at: string;
  created_at: string;
  delivery_status: string;
  email_sent_at: string | null;
  email_error: string | null;
  email_retry_count: number;
}

export interface OperatorListItem {
  id: number;
  email: string;
  display_name: string;
  active: boolean;
  last_login: string | null;
  created_at: string;
}

export interface CreateOperatorInvitationRequest {
  email: string;
  display_name?: string;
}

export interface AcceptOperatorInvitationRequest {
  token: string;
  display_name: string;
  password: string;
}

// --- Public endpoints (no auth, direct fetch) ---

export async function validateOperatorInvitation(
  token: string,
): Promise<OperatorInvitationValidation> {
  const response = await fetch(
    `/api/operator/auth/invite-validate?token=${encodeURIComponent(token)}`,
  );

  if (!response.ok) {
    let message = "Einladung ist ungültig oder abgelaufen";
    try {
      const data = (await response.json()) as { message?: string };
      if (data.message) message = data.message;
    } catch {
      // ignore parse errors
    }
    throw new OperatorApiError(message, response.status);
  }

  const json = (await response.json()) as {
    data?: OperatorInvitationValidation;
  } & OperatorInvitationValidation;

  // Handle both wrapped { data: {...} } and unwrapped responses
  return json.data ?? json;
}

export async function acceptOperatorInvitation(
  data: AcceptOperatorInvitationRequest,
): Promise<{ operator_id: number; email: string }> {
  const response = await fetch("/api/operator/auth/invite-accept", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    let message = "Ein Fehler ist aufgetreten";
    try {
      const errorData = (await response.json()) as { message?: string };
      if (errorData.message) message = errorData.message;
    } catch {
      // ignore parse errors
    }
    throw new OperatorApiError(message, response.status);
  }

  const json = (await response.json()) as {
    data?: { operator_id: number; email: string };
  };
  return (
    json.data ?? (json as unknown as { operator_id: number; email: string })
  );
}

// --- Protected endpoints (via operatorFetch with auth) ---

export async function listOperators(): Promise<OperatorListItem[]> {
  try {
    const result = await operatorFetch<OperatorListItem[]>(
      "/api/operator/operators",
    );
    // operatorFetch unwraps the proxy envelope { success, data } → data
    // but data is the backend response { status, data: [...], message }
    // so we may need to unwrap once more
    if (Array.isArray(result)) {
      return result;
    }
    const nested = result as unknown as { data?: OperatorListItem[] };
    if (nested.data && Array.isArray(nested.data)) {
      return nested.data;
    }
    return [];
  } catch (error) {
    logger.error("list_operators_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export async function listPendingOperatorInvitations(): Promise<
  PendingOperatorInvitation[]
> {
  try {
    const result = await operatorFetch<PendingOperatorInvitation[]>(
      "/api/operator/invitations",
    );
    if (Array.isArray(result)) {
      return result;
    }
    const nested = result as unknown as {
      data?: PendingOperatorInvitation[];
    };
    if (nested.data && Array.isArray(nested.data)) {
      return nested.data;
    }
    return [];
  } catch (error) {
    logger.error("list_operator_invitations_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export async function createOperatorInvitation(
  data: CreateOperatorInvitationRequest,
): Promise<PendingOperatorInvitation> {
  try {
    const result = await operatorFetch<{
      data: PendingOperatorInvitation;
    }>("/api/operator/invitations", {
      method: "POST",
      body: data,
    });
    return result.data;
  } catch (error) {
    logger.error("create_operator_invitation_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export async function resendOperatorInvitation(id: number): Promise<void> {
  try {
    await operatorFetch(`/api/operator/invitations/${id}/resend`, {
      method: "POST",
    });
  } catch (error) {
    logger.error("resend_operator_invitation_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

export async function revokeOperatorInvitation(id: number): Promise<void> {
  try {
    await operatorFetch(`/api/operator/invitations/${id}`, {
      method: "DELETE",
    });
  } catch (error) {
    logger.error("revoke_operator_invitation_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}
