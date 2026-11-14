import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";
import {
  portalAuthAnswer,
  portalAuthPath,
  type EnvelopableAuthStep,
  type PortalKey,
} from "./portal-paths";

export type LoginScope = Exclude<PortalKey, "parent">;

interface MFARequiredLoginResponse {
  status: "mfa_required";
  challenge_token: string;
  masked_email: string;
  /**
   * Whether the code step may remember the device (#6955): the portal's and
   * the school's security.mfa_trusted_device_enabled. Older backends omit it.
   */
  trusted_device_enabled?: boolean;
  /**
   * The day count of the „… merken“ button, security.mfa_trusted_device_days
   * of the school, or the registry default where the portal names none.
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
  /** Whether the enrollment's code step may remember the device (#6955). */
  trusted_device_enabled?: boolean;
  trusted_device_days?: number;
}

export interface MFAResult {
  status: "authenticated";
}

type LoginResponse =
  MFAResult | MFARequiredLoginResponse | MFAEnrollmentRequiredLoginResponse;

interface OperatorEnvelope<T> {
  status: string;
  data: T;
  message?: string;
}

function mfaUrl(scope: LoginScope, suffix: string): string {
  return portalAuthPath(scope, `mfa/${suffix}`);
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

  const response = await transportFetch(url, {
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
    // Callers branch on the envelope's code (identity.mfa_code_invalid,
    // identity.mfa_blocked, …), never on the diagnostic text.
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }

  if (data === null) {
    if (options.allowEmptyBody) {
      return undefined as T;
    }
    throw new ApiError("empty response body", 500);
  }
  return data as T;
}

/** A portal's auth step; its envelope comes from the portal table. */
async function postStep<T>(
  scope: LoginScope,
  step: EnvelopableAuthStep,
  body: unknown,
  options: PostJsonOptions = {},
): Promise<T> {
  return portalAuthAnswer<T>(
    scope,
    step,
    await postJson<unknown>(portalAuthPath(scope, step), body, options),
  );
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
  // Tenant login requires a tenant slug. School login accepts one only for
  // a handoff from the OGS portal, where it pins a multi-school Lehrkraft to
  // the school they selected.
  const payload =
    scope === "tenant" || (scope === "school" && params.tenantSlug)
      ? {
          email: params.email,
          password: params.password,
          tenant_slug: params.tenantSlug ?? "",
        }
      : { email: params.email, password: params.password };

  return postStep<LoginResponse>(scope, "login", payload);
}

interface VerifyParams {
  challengeToken: string;
  code: string;
  rememberDevice: boolean;
  /** The Träger the Träger-Portal login resolved (#4651). */
  organizationId?: string;
}

// The Träger-Portal names the Träger the login resolved when it redeems the
// code; the other portals have no such field.
function organizationField(organizationId?: string) {
  return organizationId ? { organization_id: Number(organizationId) } : {};
}

export async function verifyMFA(
  scope: LoginScope,
  params: VerifyParams,
): Promise<MFAResult> {
  const payload = {
    challenge_token: params.challengeToken,
    code: params.code,
    remember_device: params.rememberDevice,
    ...organizationField(params.organizationId),
  };
  return postStep<MFAResult>(scope, "mfa/verify", payload);
}

interface ResendParams {
  challengeToken: string;
}

interface ResendResponse {
  challenge_token: string;
}

/**
 * resendChallenge re-issues an MFA email code AND returns the renewed
 * challenge JWT. Callers MUST swap their in-flight `challengeToken` for
 * the returned value before the next verify — the previous JWT will
 * expire on its own clock while the freshly emailed code is bound to
 * the new JWT's lifetime. (#1430 review round 2 — without this swap
 * users hit a dead end where the new code can't be verified.)
 */
export async function resendChallenge(
  scope: LoginScope,
  params: ResendParams,
): Promise<string> {
  const payload = { challenge_token: params.challengeToken };
  const response = await postStep<ResendResponse>(scope, "mfa/resend", payload);
  return response.challenge_token;
}

// ----- Enrollment (the login's enrollment token as Bearer) -----

/**
 * enrollStart mails the enrollment code. Tenant, school and Träger answer
 * with the challenge the code is bound to, and their confirm redeems exactly
 * that challenge; the operator answers 204 and returns null.
 */
export async function enrollStart(
  scope: LoginScope,
  bearerToken: string,
): Promise<string | null> {
  const url = mfaUrl(scope, "enroll/start");
  const response = await postJson<{ challenge_token?: string } | undefined>(
    url,
    undefined,
    {
      bearerToken,
      allowEmptyBody: true,
    },
  );
  if (scope === "operator") return null;
  if (!response?.challenge_token) {
    throw new ApiError("Enrollment start returned no challenge token", 500);
  }
  return response.challenge_token;
}

/**
 * enrollConfirm submits the emailed code. The BFF installs the session the
 * backend mints and answers with its status only.
 */
export async function enrollConfirm(
  scope: LoginScope,
  bearerToken: string,
  code: string,
  challengeToken: string | null,
  organizationId?: string,
  rememberDevice = false,
): Promise<MFAResult> {
  const payload = {
    ...(challengeToken === null ? {} : { challenge_token: challengeToken }),
    code,
    remember_device: rememberDevice,
    ...organizationField(organizationId),
  };
  return postStep<MFAResult>(scope, "mfa/enroll/confirm", payload, {
    bearerToken,
  });
}

// ----- Self-service trusted devices (every portal with a second factor) -----
//
// These and the admin calls below act for the signed-in account: their BFF
// routes read the session cookie, so no bearer token leaves the browser
// (#5005). Only enrollment above sends the login's enrollment token.

export interface TrustedDeviceDTO {
  id: number;
  user_agent?: string;
  ip_address?: string;
  created_at: string;
  expires_at: string;
  last_used_at?: string;
}

function trustedDevicesUrl(scope: LoginScope, suffix = ""): string {
  return portalAuthPath(scope, `mfa/trusted-devices${suffix}`);
}

export async function listTrustedDevices(
  scope: LoginScope,
): Promise<TrustedDeviceDTO[]> {
  return postStep<TrustedDeviceDTO[]>(scope, "mfa/trusted-devices", undefined, {
    method: "GET",
  });
}

export async function revokeTrustedDevice(
  scope: LoginScope,
  deviceId: number,
): Promise<void> {
  await postJson<unknown>(trustedDevicesUrl(scope, `/${deviceId}`), undefined, {
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
  accountId: string,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    adminMFAUrl(accountId, ""),
    { reason },
    { method: "DELETE", allowEmptyBody: true },
  );
}

export async function adminGetMFAState(
  accountId: string,
): Promise<MFAAdminState> {
  return postJson<MFAAdminState>(adminMFAUrl(accountId, ""), undefined, {
    method: "GET",
  });
}

export async function adminSetMFAOverride(
  accountId: string,
  override: MFAAdminOverride,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    adminMFAUrl(accountId, "/override"),
    { override, reason },
    { method: "PUT", allowEmptyBody: true },
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

export async function operatorAdminResetMFA(
  schoolId: string,
  accountId: string,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    operatorAdminMFAUrl(schoolId, accountId, ""),
    { reason },
    { method: "DELETE", allowEmptyBody: true },
  );
}

/**
 * Resets the second factor of a Träger-Büro member (#4651). The membership,
 * not a school, scopes the operator's reset; the person sets the factor up
 * again at the next sign-in.
 */
export async function operatorAdminResetTraegerMFA(
  organizationId: string,
  accountId: string,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    `/api/operator/provisioning/organizations/${encodeURIComponent(organizationId)}/traeger-members/${encodeURIComponent(accountId)}/mfa`,
    { reason },
    { method: "DELETE", allowEmptyBody: true },
  );
}

/**
 * Resets the second factor of one caterer account (#4497), scoped by the
 * caterer, not a school. Its passkeys stay; the backend mails the caterer.
 */
export async function operatorAdminResetCatererMFA(
  catererId: string,
  accountId: string,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    `/api/operator/caterers/${encodeURIComponent(catererId)}/accounts/${encodeURIComponent(accountId)}/mfa`,
    { reason },
    { method: "DELETE", allowEmptyBody: true },
  );
}

export async function operatorAdminSetMFAOverride(
  schoolId: string,
  accountId: string,
  override: MFAAdminOverride,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    operatorAdminMFAUrl(schoolId, accountId, "/override"),
    { override, reason },
    { method: "PUT", allowEmptyBody: true },
  );
}

// ----- Operator account-wide override (the "mailbox lockout" switch) -----

// The account-wide override lives outside any school — it applies
// across every tenant the account belongs to. Only the operator
// surface can read or write it; tenant admins must use the per-school
// override (which only affects their own tenant). See #1430 review
// round 2 for the threat model.

function operatorGlobalMFAUrl(accountId: string): string {
  return `/api/operator/accounts/${encodeURIComponent(accountId)}/mfa/global-override`;
}

export async function operatorAdminGetGlobalMFAOverride(
  accountId: string,
): Promise<MFAAdminState> {
  const envelope = await postJson<OperatorEnvelope<MFAAdminState>>(
    operatorGlobalMFAUrl(accountId),
    undefined,
    { method: "GET" },
  );
  return envelope.data;
}

export async function operatorAdminSetGlobalMFAOverride(
  accountId: string,
  override: MFAAdminOverride,
  reason: string,
): Promise<void> {
  await postJson<unknown>(
    operatorGlobalMFAUrl(accountId),
    { override, reason },
    { method: "PUT", allowEmptyBody: true },
  );
}
