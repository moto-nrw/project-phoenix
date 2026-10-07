import {
  apiErrorFromBody,
  transportFetch,
  type ApiError,
} from "~/lib/api-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianInvitationAPI" });

export interface GuardianInvitationValidation {
  email: string;
  firstName?: string;
  lastName?: string;
  expiresAt: string;
  schoolName?: string;
  tenantSlug?: string;
  schoolLogoUrl?: string;
}

interface BackendGuardianInvitationValidation {
  email: string;
  first_name?: string;
  last_name?: string;
  expires_at: string;
  school_name?: string;
  tenant_slug?: string;
  school_logo_url?: string;
}

export interface GuardianInvitationAcceptRequest {
  password: string;
  confirmPassword: string;
}

export interface GuardianInvitationAcceptResult {
  accountId: string;
  email: string;
  tenantSlug?: string;
}

interface BackendAcceptGuardianResponse {
  account_id: number;
  email: string;
  tenant_slug?: string;
}

const createApiError = async (
  response: Response,
  fallbackMessage: string,
): Promise<ApiError> => {
  let message = fallbackMessage;
  let payload: unknown;
  try {
    const contentType = response.headers.get("Content-Type") ?? "";
    if (contentType.includes("json")) {
      payload = await response.json();
      const body = payload as { error?: unknown; message?: unknown };
      const text = body?.error ?? body?.message;
      if (typeof text === "string" && text) message = text;
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
  } catch (error) {
    logger.warn("guardian_invitation_error_parse_failed", {
      error: String(error),
    });
  }
  // A real ApiError, so code, field errors and request ID reach the shared
  // display path (#2518). The message stays a diagnostic.
  return apiErrorFromBody(message, response.status, payload);
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

const mapValidation = (
  data: BackendGuardianInvitationValidation,
): GuardianInvitationValidation => ({
  email: data.email,
  firstName: data.first_name,
  lastName: data.last_name,
  expiresAt: data.expires_at,
  schoolName: data.school_name,
  tenantSlug: data.tenant_slug,
  schoolLogoUrl: data.school_logo_url,
});

export async function validateGuardianInvitation(
  token: string,
): Promise<GuardianInvitationValidation> {
  const response = await transportFetch(
    `/api/guardian-invitations/${encodeURIComponent(token)}`,
  );
  if (!response.ok) {
    throw await createApiError(
      response,
      "Einladung konnte nicht geprüft werden.",
    );
  }
  const raw = (await response.json()) as unknown;
  return mapValidation(extractData<BackendGuardianInvitationValidation>(raw));
}

export async function acceptGuardianInvitation(
  token: string,
  data: GuardianInvitationAcceptRequest,
): Promise<GuardianInvitationAcceptResult> {
  const response = await transportFetch(
    `/api/guardian-invitations/${encodeURIComponent(token)}/accept`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    },
  );
  if (!response.ok) {
    throw await createApiError(
      response,
      "Einladung konnte nicht angenommen werden.",
    );
  }
  const raw = (await response.json()) as unknown;
  const payload = extractData<BackendAcceptGuardianResponse>(raw);
  return {
    accountId: payload.account_id.toString(),
    email: payload.email,
    tenantSlug: payload.tenant_slug,
  };
}
