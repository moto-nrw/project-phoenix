import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";

export type LoginScope = "tenant" | "operator" | "school" | "traeger";

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
  switch (scope) {
    case "operator":
      return "/api/operator/auth/login";
    case "school":
      return "/api/school/auth/login";
    default:
      return "/api/auth/login";
  }
}

function mfaUrl(scope: LoginScope, suffix: string): string {
  switch (scope) {
    case "operator":
      return `/api/operator/auth/mfa/${suffix}`;
    case "school":
      // School MFA endpoints answer top-level JSON like the tenant ones
      // (no operator envelope), so only the URL differs.
      return `/api/school/auth/mfa/${suffix}`;
    case "traeger":
      return `/api/traeger/auth/mfa/${suffix}`;
    default:
      return `/api/auth/mfa/${suffix}`;
  }
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
): Promise<MFATokenResponse> {
  const url = mfaUrl(scope, "verify");
  const payload = {
    challenge_token: params.challengeToken,
    code: params.code,
    remember_device: params.rememberDevice,
    ...organizationField(params.organizationId),
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
  const url = mfaUrl(scope, "resend");
  const payload = { challenge_token: params.challengeToken };
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<ResendResponse>>(
      url,
      payload,
    );
    return envelope.data.challenge_token;
  }
  const response = await postJson<ResendResponse>(url, payload);
  return response.challenge_token;
}

// ----- Enrollment + self-service (authenticated, requires Bearer token) -----

/**
 * enrollStart triggers the enrollment email code.
 *
 * Tenant/operator answer 204 — the confirm step verifies "the account's
 * newest active code". The SCHOOL endpoint instead returns the challenge
 * token the code is bound to, and its confirm step requires exactly that
 * challenge back (#2207). Returns the challenge token when the backend
 * provided one, else null.
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
  return response?.challenge_token ?? null;
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
  challengeToken?: string,
  organizationId?: string,
): Promise<MFATokenResponse> {
  const url = mfaUrl(scope, "enroll/confirm");
  // The school confirm is bound to the exact challenge enroll/start minted
  // (see enrollStart); tenant/operator ignore the extra field.
  const payload = {
    code,
    remember_device: rememberDevice,
    ...(challengeToken ? { challenge_token: challengeToken } : {}),
    ...organizationField(organizationId),
  };
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
  return isOperator(scope)
    ? `/api/operator/auth/mfa/trusted-devices${suffix}`
    : `/api/auth/mfa/trusted-devices${suffix}`;
}

export async function listTrustedDevices(
  scope: LoginScope,
): Promise<TrustedDeviceDTO[]> {
  const url = trustedDevicesUrl(scope);
  if (isOperator(scope)) {
    const envelope = await postJson<OperatorEnvelope<TrustedDeviceDTO[]>>(
      url,
      undefined,
      { method: "GET" },
    );
    return envelope.data;
  }
  return postJson<TrustedDeviceDTO[]>(url, undefined, {
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
