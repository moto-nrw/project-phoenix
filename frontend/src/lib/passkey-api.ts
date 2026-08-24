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
import { getCachedSession } from "./session-cache";

export type PasskeyScope = "tenant" | "operator";

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
  return scope === "operator"
    ? "/api/operator/auth/passkeys"
    : "/api/auth/passkeys";
}

function extractOptions<TOptions>(
  response: CeremonyResponse<TOptions>,
): TOptions {
  const maybeWrapped = response.options as { publicKey?: TOptions };
  return maybeWrapped.publicKey ?? (response.options as TOptions);
}

async function currentBearerToken(): Promise<string> {
  const session = await getCachedSession();
  const token = session?.user?.token;
  if (!token) {
    throw new PasskeyApiError(401, "Nicht angemeldet.");
  }
  return token;
}

async function requestJson<T>(
  url: string,
  body?: unknown,
  token?: string,
  method: "GET" | "POST" | "DELETE" = "POST",
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (token) headers.Authorization = `Bearer ${token}`;

  const response = await fetch(url, {
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
    throw new PasskeyApiError(
      response.status,
      extractErrorMessage(data) ??
        `Passkey-Anfrage fehlgeschlagen (${response.status}).`,
      extractErrorCode(data) ?? undefined,
    );
  }

  return data as T;
}

function extractErrorMessage(data: unknown): string | null {
  if (!data || typeof data !== "object") return null;
  const rec = data as Record<string, unknown>;
  if (typeof rec.error === "string") return rec.error;
  if (typeof rec.message === "string") return rec.message;
  if (typeof rec.error_text === "string") return rec.error_text;
  return null;
}

function extractErrorCode(data: unknown): string | null {
  if (!data || typeof data !== "object") return null;
  const rec = data as Record<string, unknown>;
  return typeof rec.code === "string" ? rec.code : null;
}

export class PasskeyApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
  ) {
    super(message);
    this.name = "PasskeyApiError";
  }
}

export function isPasskeyCeremonyIncompleteError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;

  if (error.name === "AbortError" || error.name === "NotAllowedError") {
    return true;
  }

  const message = error.message.toLowerCase();
  return (
    message.includes("operation either timed out or was not allowed") ||
    message.includes("user cancelled") ||
    message.includes("user canceled")
  );
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
  const token = await currentBearerToken();
  return requestJson<EnrollmentChallengeResponse>(
    `${basePath(scope)}/enrollment/challenge`,
    {},
    token,
  );
}

export async function registerPasskey(
  scope: PasskeyScope,
  params: { code: string; name: string },
): Promise<PasskeyCredentialSummary> {
  const token = await currentBearerToken();
  const start = await requestJson<
    CeremonyResponse<PublicKeyCredentialCreationOptionsJSON>
  >(
    `${basePath(scope)}/register/options`,
    { code: params.code, name: params.name },
    token,
  );
  const registration = await startRegistration({
    optionsJSON: extractOptions(start),
  });
  return finishPasskeyRegistration(scope, token, start.session_id, {
    name: params.name,
    response: registration,
  });
}

async function finishPasskeyRegistration(
  scope: PasskeyScope,
  token: string,
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
    token,
  );
}

export async function listPasskeys(
  scope: PasskeyScope,
): Promise<PasskeyCredentialSummary[]> {
  const token = await currentBearerToken();
  return requestJson<PasskeyCredentialSummary[]>(
    basePath(scope),
    undefined,
    token,
    "GET",
  );
}

export async function revokePasskey(
  scope: PasskeyScope,
  id: string,
): Promise<void> {
  const token = await currentBearerToken();
  await requestJson<void>(
    `${basePath(scope)}/${id}`,
    undefined,
    token,
    "DELETE",
  );
}
