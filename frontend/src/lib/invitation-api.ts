import type { ApiError } from "~/lib/auth-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InvitationAPI" });

import type {
  InvitationValidation,
  InvitationAcceptRequest,
  CreateInvitationRequest,
  InvitationRecord,
  BackendInvitationValidation,
  BackendInvitation,
} from "./invitation-helpers";
import {
  mapInvitationValidationResponse,
  mapInvitationResponse,
  mapPendingInvitationResponse,
} from "./invitation-helpers";

const parseRetryAfter = (value: string | null): number | undefined => {
  if (!value) return undefined;
  const numeric = Number(value);
  if (!Number.isNaN(numeric)) {
    return Math.max(0, Math.round(numeric));
  }
  const date = Date.parse(value);
  if (!Number.isNaN(date)) {
    const diff = date - Date.now();
    return diff > 0 ? Math.ceil(diff / 1000) : 0;
  }
  return undefined;
};

const createApiError = async (
  response: Response,
  fallbackMessage: string,
): Promise<ApiError> => {
  let message = fallbackMessage;

  try {
    const contentType = response.headers.get("Content-Type") ?? "";
    if (contentType.includes("application/json")) {
      const payload = (await response.json()) as {
        error?: string;
        message?: string;
      };
      message = payload.error ?? payload.message ?? fallbackMessage;
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
  } catch (error) {
    logger.warn("failed to parse invitation API error", {
      error: String(error),
    });
  }

  const apiError = new Error(message) as ApiError;
  apiError.status = response.status;
  const retry = parseRetryAfter(response.headers.get("Retry-After"));
  if (retry !== undefined) {
    apiError.retryAfterSeconds = retry;
  }
  return apiError;
};

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
    throw await createApiError(
      response,
      "Einladung konnte nicht geprüft werden.",
    );
  }
  const raw = (await response.json()) as unknown;
  const data = extractData<BackendInvitationValidation>(raw);
  return mapInvitationValidationResponse(data);
}

interface AcceptInvitationResult {
  tenantSlug?: string;
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
    throw await createApiError(
      response,
      "Einladung konnte nicht angenommen werden.",
    );
  }

  const raw = (await response.json()) as Record<string, unknown>;
  const result = (raw.data ?? raw) as Record<string, unknown>;
  return {
    tenantSlug:
      typeof result.tenant_slug === "string" ? result.tenant_slug : undefined,
  };
}

export async function createInvitation(
  data: CreateInvitationRequest,
): Promise<InvitationRecord> {
  const response = await fetch("/api/invitations", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
    credentials: "include",
  });

  if (!response.ok) {
    throw await createApiError(
      response,
      "Einladung konnte nicht erstellt werden.",
    );
  }

  const raw = (await response.json()) as unknown;
  const invitationData = extractData<BackendInvitation>(raw);
  return mapInvitationResponse(invitationData);
}

export async function listInvitations(): Promise<InvitationRecord[]> {
  const response = await fetch("/api/invitations", {
    credentials: "include",
  });
  if (!response.ok) {
    throw await createApiError(
      response,
      "Einladungen konnten nicht geladen werden.",
    );
  }
  const raw = (await response.json()) as unknown;
  const extracted = extractData<BackendInvitation[] | BackendInvitation>(raw);
  if (Array.isArray(extracted)) {
    return extracted.map(mapInvitationResponse);
  }
  return [mapInvitationResponse(extracted)];
}

export async function listPendingInvitations(): Promise<InvitationRecord[]> {
  return listInvitations();
}

export async function resendInvitation(id: number): Promise<void> {
  const response = await fetch(`/api/invitations/${id}/resend`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw await createApiError(
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
    throw await createApiError(
      response,
      "Einladung konnte nicht widerrufen werden.",
    );
  }
}
