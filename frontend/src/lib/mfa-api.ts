import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MFAApi" });

export type LoginScope = "tenant" | "operator";

interface AuthenticatedLoginResponse {
  status: "authenticated";
  access_token: string;
  refresh_token: string;
  mfa_enrollment_required?: boolean;
}

interface MFARequiredLoginResponse {
  status: "mfa_required";
  challenge_token: string;
  masked_email: string;
  /**
   * Mirrors security.mfa_trusted_device_enabled for the tenant. Omitted by
   * older backends — treat undefined as `true` so the checkbox keeps
   * appearing for clients that haven't shipped this field yet.
   */
  trusted_device_enabled?: boolean;
}

type LoginResponse = AuthenticatedLoginResponse | MFARequiredLoginResponse;

export interface MFATokenResponse {
  access_token: string;
  refresh_token: string;
}

interface OperatorEnvelope<T> {
  status: string;
  data: T;
  message?: string;
}

function isOperator(scope: LoginScope): boolean {
  return scope === "operator";
}

function loginUrl(scope: LoginScope): string {
  return isOperator(scope) ? "/api/operator/auth/login" : "/api/auth/login";
}

function mfaUrl(scope: LoginScope, suffix: string): string {
  return isOperator(scope)
    ? `/api/operator/auth/mfa/${suffix}`
    : `/api/auth/mfa/${suffix}`;
}

interface PostJsonOptions {
  readonly bearerToken?: string;
  readonly allowEmptyBody?: boolean;
  readonly method?: "GET" | "POST" | "DELETE";
}

async function postJson<T>(
  url: string,
  body: unknown,
  options: PostJsonOptions = {},
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (options.bearerToken) {
    headers.Authorization = `Bearer ${options.bearerToken}`;
  }

  const response = await fetch(url, {
    method: options.method ?? "POST",
    headers,
    credentials: "include",
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (response.status === 204) {
    return undefined as T;
  }

  let data: unknown;
  const text = await response.text();
  try {
    data = text ? (JSON.parse(text) as unknown) : null;
  } catch {
    data = null;
  }

  if (!response.ok) {
    const err = new MFAApiError(
      response.status,
      extractErrorMessage(data) ?? `Request failed (${response.status})`,
    );
    throw err;
  }

  if (data === null) {
    if (options.allowEmptyBody) {
      return undefined as T;
    }
    throw new MFAApiError(
      500,
      "Leere Antwort vom Server. Bitte versuchen Sie es erneut.",
    );
  }
  return data as T;
}

function extractErrorMessage(data: unknown): string | null {
  if (!data || typeof data !== "object") return null;
  const rec = data as Record<string, unknown>;
  if (typeof rec.error === "string") return rec.error;
  if (typeof rec.message === "string") return rec.message;
  return null;
}

export class MFAApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "MFAApiError";
  }
}

interface LoginParams {
  email: string;
  password: string;
  tenantSlug?: string;
}

export async function login(
  scope: LoginScope,
  params: LoginParams,
): Promise<LoginResponse> {
  const url = loginUrl(scope);
  const payload = isOperator(scope)
    ? { email: params.email, password: params.password }
    : {
        email: params.email,
        password: params.password,
        tenant_slug: params.tenantSlug ?? "",
      };

  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<LoginResponse>>(
      url,
      payload,
    );
    return envelope.data;
  }
  return postJson<LoginResponse>(url, payload);
}

interface VerifyParams {
  challengeToken: string;
  code: string;
  rememberDevice: boolean;
}

export async function verifyMFA(
  scope: LoginScope,
  params: VerifyParams,
): Promise<MFATokenResponse> {
  const url = mfaUrl(scope, "verify");
  const payload = {
    challenge_token: params.challengeToken,
    code: params.code,
    remember_device: params.rememberDevice,
  };
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<MFATokenResponse>>(
      url,
      payload,
    );
    return envelope.data;
  }
  return postJson<MFATokenResponse>(url, payload);
}

interface RecoveryParams {
  challengeToken: string;
  recoveryCode: string;
  rememberDevice: boolean;
}

export async function verifyRecovery(
  scope: LoginScope,
  params: RecoveryParams,
): Promise<MFATokenResponse> {
  const url = mfaUrl(scope, "recovery/verify");
  const payload = {
    challenge_token: params.challengeToken,
    recovery_code: params.recoveryCode,
    remember_device: params.rememberDevice,
  };
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<MFATokenResponse>>(
      url,
      payload,
    );
    return envelope.data;
  }
  return postJson<MFATokenResponse>(url, payload);
}

interface ResendParams {
  challengeToken: string;
}

export async function resendChallenge(
  scope: LoginScope,
  params: ResendParams,
): Promise<void> {
  const url = mfaUrl(scope, "resend");
  const payload = { challenge_token: params.challengeToken };
  await postJson<unknown>(url, payload);
}

// ----- Enrollment + self-service (authenticated, requires Bearer token) -----

function enrollUrl(scope: LoginScope, suffix: string): string {
  return isOperator(scope)
    ? `/api/operator/auth/mfa/${suffix}`
    : `/api/auth/mfa/${suffix}`;
}

interface RecoveryCodesResponse {
  recovery_codes: string[];
}

export async function enrollStart(
  scope: LoginScope,
  bearerToken: string,
): Promise<void> {
  const url = enrollUrl(scope, "enroll/start");
  await postJson<unknown>(url, undefined, {
    bearerToken,
    allowEmptyBody: true,
  });
}

export async function enrollConfirm(
  scope: LoginScope,
  bearerToken: string,
  code: string,
): Promise<RecoveryCodesResponse> {
  const url = enrollUrl(scope, "enroll/confirm");
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<RecoveryCodesResponse>>(
      url,
      { code },
      { bearerToken },
    );
    return envelope.data;
  }
  return postJson<RecoveryCodesResponse>(url, { code }, { bearerToken });
}

// ----- Admin-Override (Tenant-only; requires users:manage) -----

function adminMFAUrl(accountId: string, suffix: string): string {
  return `/api/auth/accounts/${encodeURIComponent(accountId)}/mfa${suffix}`;
}

export async function adminResetMFA(
  bearerToken: string,
  accountId: string,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    adminMFAUrl(accountId, ""),
    { reason },
    { bearerToken, method: "DELETE", allowEmptyBody: true },
  );
}

export async function adminRegenerateRecoveryCodes(
  bearerToken: string,
  accountId: string,
  reason: string,
): Promise<RecoveryCodesResponse> {
  return postJson<RecoveryCodesResponse>(
    adminMFAUrl(accountId, "/recovery-codes"),
    { reason },
    { bearerToken },
  );
}

export function germanMFAErrorMessage(err: unknown): string {
  if (!(err instanceof MFAApiError)) {
    logger.warn("unexpected_mfa_error_shape", {
      error: err instanceof Error ? err.message : String(err),
    });
    return "Anmeldefehler. Bitte versuchen Sie es erneut.";
  }
  const text = err.message.toLowerCase();
  if (err.status === 401) {
    if (text.includes("locked") || text.includes("gesperrt")) {
      return "Konto vorübergehend gesperrt. Bitte versuchen Sie es in 15 Minuten erneut.";
    }
    // Check "invalid" before "expired": the backend conflates both into the
    // single string "invalid or expired code" for security (no oracle on
    // whether the code was wrong vs. timed out). In practice the dominant
    // cause is a mistyped code, so we surface the friendlier "ungültig"
    // message and let the user retry. A genuinely expired code falls into
    // the same branch — both states are resolved by requesting a new code.
    if (text.includes("invalid") || text.includes("ungültig")) {
      return "Der eingegebene Code ist ungültig. Bitte erneut versuchen.";
    }
    if (text.includes("expired") || text.includes("abgelaufen")) {
      return "Der Code ist abgelaufen. Fordern Sie einen neuen Code an.";
    }
    return "Anmeldung fehlgeschlagen. Bitte erneut versuchen.";
  }
  if (err.status === 429) {
    // Distinguish the two 429 paths from the backend:
    //   ErrMFALocked      ("account locked due to too many failed attempts")
    //   ErrMFARateLimited ("too many code requests, please wait")
    if (text.includes("locked") || text.includes("gesperrt")) {
      return "Konto vorübergehend gesperrt. Bitte versuchen Sie es in 15 Minuten erneut.";
    }
    if (text.includes("code request") || text.includes("code-anfrage")) {
      return "Zu viele Code-Anfragen. Bitte warten Sie ein paar Minuten, bevor Sie einen neuen Code anfordern.";
    }
    return "Zu viele Versuche. Bitte warten Sie einen Moment.";
  }
  if (err.status >= 500) {
    return "Der Server ist gerade nicht erreichbar. Bitte später erneut versuchen.";
  }
  return err.message;
}
