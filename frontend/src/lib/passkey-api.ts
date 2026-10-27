import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";
import {
  browserSupportsWebAuthn,
  startAuthentication,
  startRegistration,
} from "@simplewebauthn/browser";
import type {
  AuthenticationResponseJSON,
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
  RegistrationResponseJSON,
} from "@simplewebauthn/browser";
import { hasSessionAccess } from "./session-access";
import { getCachedSession } from "./session-cache";

export type PasskeyScope = "tenant" | "operator" | "caterer";

export interface PasskeyTokenResponse {
  status: "authenticated";
  access_token: string;
  refresh_token: string;
}

export interface PasskeyCredentialSummary {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
}

interface CeremonyResponse<TOptions> {
  session_id: string;
  options: { publicKey: TOptions } | TOptions;
}

interface EnrollmentChallengeResponse {
  challenge_token: string;
  masked_email: string;
}

function basePath(scope: PasskeyScope): string {
  switch (scope) {
    case "operator":
      return "/api/operator/auth/passkeys";
    case "caterer":
      return "/api/caterer/auth/passkeys";
    default:
      return "/api/auth/passkeys";
  }
}

function extractOptions<TOptions>(
  response: CeremonyResponse<TOptions>,
): TOptions {
  const maybeWrapped = response.options as { publicKey?: TOptions };
  return maybeWrapped.publicKey ?? (response.options as TOptions);
}

/**
 * Passkey management acts for the signed-in account. Its BFF routes read the
 * session cookie; the browser holds no token (#5005).
 */
async function requireSession(): Promise<void> {
  if (!hasSessionAccess(await getCachedSession())) {
    throw new ApiError("Nicht angemeldet.", 401);
  }
}

async function requestJson<T>(
  url: string,
  body?: unknown,
  method: "GET" | "POST" | "DELETE" = "POST",
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  // A request that never reaches the API is general.unavailable (#2517);
  // an AbortError stays an AbortError.
  const response = await transportFetch(url, {
    method,
    headers,
    credentials: "include",
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  let data: unknown = null;
  try {
    data = text ? (JSON.parse(text) as unknown) : null;
  } catch {
    data = null;
  }

  if (!response.ok) {
    throw apiErrorFromBody(
      `Passkey-Anfrage fehlgeschlagen (${response.status}).`,
      response.status,
      data,
    );
  }

  return data as T;
}

export function isPasskeyCeremonyIncompleteError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;

  // WebAuthnError from @simplewebauthn/browser takes the name of the
  // browser's DOMException, so the name identifies a cancelled ceremony.
  return error.name === "AbortError" || error.name === "NotAllowedError";
}

export function isPasskeySupported(): boolean {
  return browserSupportsWebAuthn();
}

export async function loginWithPasskey(
  scope: PasskeyScope,
  params: { tenantSlug?: string } = {},
): Promise<PasskeyTokenResponse> {
  const start = await requestJson<
    CeremonyResponse<PublicKeyCredentialRequestOptionsJSON>
  >(`${basePath(scope)}/login/options`, {
    tenant_slug: params.tenantSlug ?? "",
  });
  const assertion = await startAuthentication({
    optionsJSON: extractOptions(start),
  });
  return finishPasskeyLogin(scope, start.session_id, assertion);
}

async function finishPasskeyLogin(
  scope: PasskeyScope,
  sessionID: string,
  response: AuthenticationResponseJSON,
): Promise<PasskeyTokenResponse> {
  return requestJson<PasskeyTokenResponse>(`${basePath(scope)}/login/verify`, {
    session_id: sessionID,
    response,
  });
}

export async function startPasskeyEnrollment(
  scope: PasskeyScope,
): Promise<EnrollmentChallengeResponse> {
  await requireSession();
  return requestJson<EnrollmentChallengeResponse>(
    `${basePath(scope)}/enrollment/challenge`,
    {},
  );
}

export async function registerPasskey(
  scope: PasskeyScope,
  params: { code: string; name: string },
): Promise<PasskeyCredentialSummary> {
  await requireSession();
  const start = await requestJson<
    CeremonyResponse<PublicKeyCredentialCreationOptionsJSON>
  >(`${basePath(scope)}/register/options`, {
    code: params.code,
    name: params.name,
  });
  const registration = await startRegistration({
    optionsJSON: extractOptions(start),
  });
  return finishPasskeyRegistration(scope, start.session_id, {
    name: params.name,
    response: registration,
  });
}

async function finishPasskeyRegistration(
  scope: PasskeyScope,
  sessionID: string,
  params: { name: string; response: RegistrationResponseJSON },
): Promise<PasskeyCredentialSummary> {
  return requestJson<PasskeyCredentialSummary>(
    `${basePath(scope)}/register/verify`,
    {
      session_id: sessionID,
      name: params.name,
      response: params.response,
    },
  );
}

export async function listPasskeys(
  scope: PasskeyScope,
): Promise<PasskeyCredentialSummary[]> {
  await requireSession();
  return requestJson<PasskeyCredentialSummary[]>(
    basePath(scope),
    undefined,
    "GET",
  );
}

export async function revokePasskey(
  scope: PasskeyScope,
  id: string,
): Promise<void> {
  await requireSession();
  await requestJson<void>(`${basePath(scope)}/${id}`, undefined, "DELETE");
}
