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
  /**
   * Mirrors security.mfa_trusted_device_days for the tenant. The frontend
   * uses it to render "Auf diesem Gerät N Tage merken" with the exact day
   * count the backend will issue the cookie for. Omitted by older
   * backends — treat undefined as the registry default.
   */
  trusted_device_days?: number;
}

/**
 * MFAEnrollmentRequiredLoginResponse is the new (post-#1430) login shape
 * for accounts on an MFA-required tenant that have no credential yet.
 * `access_token` carries an enrollment-scoped JWT that ONLY authorizes
 * `/auth/mfa/enroll/*` — it is rejected by every other authenticated
 * route. `refresh_token` is intentionally absent: the full session is
 * minted by `/auth/mfa/enroll/confirm` after the user proves they own
 * the inbox.
 */
interface MFAEnrollmentRequiredLoginResponse {
  status: "mfa_enrollment_required";
  access_token: string;
  masked_email: string;
  mfa_enrollment_required: true;
}

type LoginResponse =
  | AuthenticatedLoginResponse
  | MFARequiredLoginResponse
  | MFAEnrollmentRequiredLoginResponse;

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
  readonly method?: "GET" | "POST" | "PUT" | "DELETE";
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

export async function enrollStart(
  scope: LoginScope,
  bearerToken: string,
): Promise<void> {
  const url = mfaUrl(scope, "enroll/start");
  await postJson<unknown>(url, undefined, {
    bearerToken,
    allowEmptyBody: true,
  });
}

/**
 * enrollConfirm submits the emailed code and returns the freshly-minted
 * access/refresh pair. Post-#1430 the backend no longer reuses the
 * enrollment-scoped token for the regular session — successful
 * confirmation mints a full session token pair that the frontend uses to
 * seed NextAuth, identical to the verify-flow contract.
 *
 * `rememberDevice: true` additionally issues a trusted-device cookie so
 * the next login on this browser skips MFA.
 */
export async function enrollConfirm(
  scope: LoginScope,
  bearerToken: string,
  code: string,
  rememberDevice = false,
): Promise<MFATokenResponse> {
  const url = mfaUrl(scope, "enroll/confirm");
  const payload = { code, remember_device: rememberDevice };
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<MFATokenResponse>>(
      url,
      payload,
      { bearerToken },
    );
    return envelope.data;
  }
  return postJson<MFATokenResponse>(url, payload, { bearerToken });
}

// ----- Self-service trusted devices (Tenant-only) -----

export interface TrustedDeviceDTO {
  id: number;
  user_agent?: string;
  ip_address?: string;
  created_at: string;
  expires_at: string;
  last_used_at?: string;
}

function trustedDevicesUrl(scope: LoginScope, suffix = ""): string {
  return isOperator(scope)
    ? `/api/operator/auth/mfa/trusted-devices${suffix}`
    : `/api/auth/mfa/trusted-devices${suffix}`;
}

export async function listTrustedDevices(
  scope: LoginScope,
  bearerToken: string,
): Promise<TrustedDeviceDTO[]> {
  const url = trustedDevicesUrl(scope);
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<TrustedDeviceDTO[]>>(
      url,
      undefined,
      { bearerToken, method: "GET" },
    );
    return envelope.data;
  }
  return postJson<TrustedDeviceDTO[]>(url, undefined, {
    bearerToken,
    method: "GET",
  });
}

export async function revokeTrustedDevice(
  scope: LoginScope,
  bearerToken: string,
  deviceId: number,
): Promise<void> {
  await postJson<unknown>(trustedDevicesUrl(scope, `/${deviceId}`), undefined, {
    bearerToken,
    method: "DELETE",
    allowEmptyBody: true,
  });
}

// ----- Admin-Override (Tenant-only; requires users:manage) -----

export type MFAAdminOverride = "none" | "force_off" | "force_on";

export interface MFAAdminState {
  enrolled: boolean;
  override: MFAAdminOverride;
}

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

export async function adminGetMFAState(
  bearerToken: string,
  accountId: string,
): Promise<MFAAdminState> {
  return postJson<MFAAdminState>(adminMFAUrl(accountId, ""), undefined, {
    bearerToken,
    method: "GET",
  });
}

export async function adminSetMFAOverride(
  bearerToken: string,
  accountId: string,
  override: MFAAdminOverride,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    adminMFAUrl(accountId, "/override"),
    { override, reason },
    { bearerToken, method: "PUT", allowEmptyBody: true },
  );
}

// ----- Operator-side admin (Operator dashboard; same modal, different URLs) -----

function operatorAdminMFAUrl(
  schoolId: string,
  accountId: string,
  suffix: string,
): string {
  return `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/mfa${suffix}`;
}

export async function operatorAdminGetMFAState(
  bearerToken: string,
  schoolId: string,
  accountId: string,
): Promise<MFAAdminState> {
  const envelope = await postJson<OperatorEnvelope<MFAAdminState>>(
    operatorAdminMFAUrl(schoolId, accountId, ""),
    undefined,
    { bearerToken, method: "GET" },
  );
  return envelope.data;
}

export async function operatorAdminResetMFA(
  bearerToken: string,
  schoolId: string,
  accountId: string,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    operatorAdminMFAUrl(schoolId, accountId, ""),
    { reason },
    { bearerToken, method: "DELETE", allowEmptyBody: true },
  );
}

export async function operatorAdminSetMFAOverride(
  bearerToken: string,
  schoolId: string,
  accountId: string,
  override: MFAAdminOverride,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    operatorAdminMFAUrl(schoolId, accountId, "/override"),
    { override, reason },
    { bearerToken, method: "PUT", allowEmptyBody: true },
  );
}

function germanMessageFor401(text: string): string {
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

function germanMessageFor429(text: string): string {
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

export function germanMFAErrorMessage(err: unknown): string {
  if (!(err instanceof MFAApiError)) {
    logger.warn("unexpected_mfa_error_shape", {
      error: err instanceof Error ? err.message : String(err),
    });
    return "Anmeldefehler. Bitte versuchen Sie es erneut.";
  }
  const text = err.message.toLowerCase();
  if (err.status === 401) return germanMessageFor401(text);
  if (err.status === 429) return germanMessageFor429(text);
  if (err.status >= 500) {
    return "Der Server ist gerade nicht erreichbar. Bitte später erneut versuchen.";
  }
  // Anything else (400/403/404/etc.) used to leak the raw backend English
  // string into the German UI ("invalid request body", "permission denied",
  // …). Log the original message so we can spot missing translations, but
  // surface a generic German fallback to the user. (#1430 review item #11)
  logger.warn("mfa_error_no_translation", {
    status: err.status,
    error: err.message,
  });
  return "Anmeldung fehlgeschlagen. Bitte erneut versuchen.";
}
