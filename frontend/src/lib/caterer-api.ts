// Client-side API of the caterer portal (ADR 0077). Every call goes through a
// Next.js route under /api/caterer/*, which forwards it with the caterer
// session to the backend's /caterer/* routes. The invitation and the
// password reset need no session: the mailed link is the proof.

import { authFetch } from "./api-helpers";
import { apiErrorFromResponse, transportFetch } from "./api-error";
import { downloadBlob, filenameFromDisposition } from "./file-download";

interface ProxyResponse<T> {
  success: boolean;
  data: T;
}

/** Wire shape of GET /caterer/schools. */
export interface BackendCatererSchools {
  caterer_name: string;
  schools: { id: string; name: string }[];
}

interface BackendChild {
  first_name?: string;
  last_name?: string;
  school_class?: string;
}

interface BackendOrderDay {
  date: string;
  closed: boolean;
  deadline: string;
  status: CatererOrderStatus;
  order_id?: string;
  confirmed_at?: string;
  total: number;
  classes: { school_class: string; count: number }[];
  children: BackendChild[];
  changes: {
    sequence: number;
    child: BackendChild;
    delta: number;
    detected_at: string;
  }[];
}

/** Wire shape of GET /caterer/schools/{schoolId}/orders. */
export interface BackendCatererOrders {
  school: { id: string; name: string };
  delivery_address: string;
  format_version: number;
  rhythm: "daily" | "weekly";
  detail: "total" | "by_class";
  days: BackendOrderDay[];
}

type CatererOrderStatus = "open" | "ordered" | "confirmed" | "unordered";

export interface CatererSchool {
  id: string;
  name: string;
}

export interface CatererSchools {
  catererName: string;
  schools: CatererSchool[];
}

/** A child in an order: a name only while the school shows names. */
export interface CatererOrderChild {
  firstName: string;
  lastName: string;
  schoolClass: string;
}

interface CatererOrderChange {
  sequence: number;
  child: CatererOrderChild;
  delta: number;
  detectedAt: string;
}

export interface CatererOrderDay {
  date: string;
  closed: boolean;
  deadline: string;
  status: CatererOrderStatus;
  orderId: string | null;
  confirmedAt: string | null;
  total: number;
  classes: { schoolClass: string; count: number }[];
  children: CatererOrderChild[];
  changes: CatererOrderChange[];
}

export interface CatererOrders {
  school: CatererSchool;
  deliveryAddress: string;
  rhythm: "daily" | "weekly";
  detail: "total" | "by_class";
  days: CatererOrderDay[];
}

/** What the acceptance page shows about a mailed invitation. */
export interface CatererInvitationPreview {
  email: string;
  catererName: string;
  schoolName: string;
  expiresAt: string;
  hasCatererAccount: boolean;
  addressTaken: boolean;
}

function mapChild(child: BackendChild): CatererOrderChild {
  return {
    firstName: child.first_name ?? "",
    lastName: child.last_name ?? "",
    schoolClass: child.school_class ?? "",
  };
}

function mapCatererOrders(data: BackendCatererOrders): CatererOrders {
  return {
    school: data.school,
    deliveryAddress: data.delivery_address,
    rhythm: data.rhythm,
    detail: data.detail,
    days: data.days.map((day) => ({
      date: day.date,
      closed: day.closed,
      deadline: day.deadline,
      status: day.status,
      orderId: day.order_id ?? null,
      confirmedAt: day.confirmed_at ?? null,
      total: day.total,
      classes: day.classes.map((count) => ({
        schoolClass: count.school_class,
        count: count.count,
      })),
      children: day.children.map(mapChild),
      changes: day.changes.map((change) => ({
        sequence: change.sequence,
        child: mapChild(change.child),
        delta: change.delta,
        detectedAt: change.detected_at,
      })),
    })),
  };
}

export async function fetchCatererSchools(): Promise<CatererSchools> {
  const response = await authFetch<ProxyResponse<BackendCatererSchools>>(
    "/api/caterer/schools",
  );
  return {
    catererName: response.data.caterer_name,
    schools: response.data.schools,
  };
}

export async function fetchCatererOrders(
  schoolId: string,
  from: string,
  to: string,
): Promise<CatererOrders> {
  const params = new URLSearchParams({ from, to });
  const response = await authFetch<ProxyResponse<BackendCatererOrders>>(
    `/api/caterer/schools/${encodeURIComponent(schoolId)}/orders?${params.toString()}`,
  );
  return mapCatererOrders(response.data);
}

export async function confirmCatererOrder(
  schoolId: string,
  orderId: string,
): Promise<void> {
  await authFetch<unknown>(
    `/api/caterer/schools/${encodeURIComponent(schoolId)}/orders/${encodeURIComponent(orderId)}/confirm`,
    { method: "POST", body: {} },
  );
}

function invitationUrl(schoolId: string, token: string): string {
  return `/api/caterer/invitations/${encodeURIComponent(schoolId)}/${encodeURIComponent(token)}`;
}

export async function fetchCatererInvitation(
  schoolId: string,
  token: string,
): Promise<CatererInvitationPreview> {
  const response = await authFetch<
    ProxyResponse<{
      email: string;
      caterer_name: string;
      school_name: string;
      expires_at: string;
      has_caterer_account: boolean;
      address_taken: boolean;
    }>
  >(invitationUrl(schoolId, token));
  return {
    email: response.data.email,
    catererName: response.data.caterer_name,
    schoolName: response.data.school_name,
    expiresAt: response.data.expires_at,
    hasCatererAccount: response.data.has_caterer_account,
    addressTaken: response.data.address_taken,
  };
}

export async function acceptCatererInvitation(
  schoolId: string,
  token: string,
  password: string,
  confirmPassword: string,
): Promise<void> {
  await authFetch<unknown>(`${invitationUrl(schoolId, token)}/accept`, {
    method: "POST",
    body: { password, confirm_password: confirmPassword },
  });
}

/** The answer of the caterer login (#4497): a session, or the second step. */
export type CatererLoginResult =
  | { status: "authenticated" }
  | {
      status: "mfa_required";
      challengeToken: string;
      maskedEmail: string;
      trustedDeviceEnabled: boolean;
      trustedDeviceDays?: number;
    };

interface BackendCatererLogin {
  status?: "authenticated" | "mfa_required";
  challenge_token?: string;
  masked_email?: string;
  trusted_device_enabled?: boolean;
  trusted_device_days?: number;
}

/**
 * Signs a caterer in. The BFF installs the caterer session itself; the
 * browser only learns whether a second step follows. A refusal comes back
 * as an ApiError with its code.
 */
export async function loginCaterer(
  email: string,
  password: string,
): Promise<CatererLoginResult> {
  // The trusted-device cookie of this browser travels along (same origin),
  // so a remembered device skips the code.
  const response = await transportFetch("/api/caterer/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "caterer login failed");
  }
  const { data } = (await response.json()) as { data: BackendCatererLogin };
  if (data.status === "mfa_required") {
    return {
      status: "mfa_required",
      challengeToken: data.challenge_token ?? "",
      maskedEmail: data.masked_email ?? "",
      trustedDeviceEnabled: data.trusted_device_enabled ?? false,
      trustedDeviceDays: data.trusted_device_days,
    };
  }
  return { status: "authenticated" };
}

/** The caterer's two-factor sign-in as the account page shows it (#4497). */
export interface CatererMFAStatus {
  enabled: boolean;
  maskedEmail: string;
}

export async function fetchCatererMFAStatus(): Promise<CatererMFAStatus> {
  const data = await authFetch<{ enabled: boolean; masked_email?: string }>(
    "/api/caterer/account/mfa",
  );
  return { enabled: data.enabled, maskedEmail: data.masked_email ?? "" };
}

/** Mails the code that switches two-factor sign-in on. */
export async function startCatererMFA(): Promise<{
  challengeToken: string;
  maskedEmail: string;
}> {
  const data = await authFetch<{
    challenge_token: string;
    masked_email?: string;
  }>("/api/caterer/account/mfa/start", { method: "POST", body: {} });
  return {
    challengeToken: data.challenge_token,
    maskedEmail: data.masked_email ?? "",
  };
}

/**
 * Switches two-factor sign-in on with the mailed code. The backend answers
 * with a new session with the second factor proven, and the BFF installs it,
 * because the older sessions of the account lose access at their next
 * renewal. `sessionRenewed` is false when the code was right but the new
 * session could not be installed.
 */
export async function confirmCatererMFA(
  challengeToken: string,
  code: string,
): Promise<{ sessionRenewed: boolean }> {
  const data = await authFetch<{ session_renewed: boolean }>(
    "/api/caterer/account/mfa/confirm",
    {
      method: "POST",
      body: { challenge_token: challengeToken, code },
    },
  );
  return { sessionRenewed: data.session_renewed };
}

/** Switches two-factor sign-in off after checking the current password. */
export async function disableCatererMFA(
  currentPassword: string,
): Promise<void> {
  await authFetch<unknown>("/api/caterer/account/mfa/disable", {
    method: "POST",
    body: { currentPassword },
  });
}

export type DeliveryExportFormat = "pdf" | "xlsx" | "csv";

/**
 * Downloads the delivery list of every school the caterer supplies for
 * [from, to] (#3873): date, school, delivery address, class, meals, change.
 */
export async function downloadCatererDeliveries(
  from: string,
  to: string,
  format: DeliveryExportFormat,
): Promise<void> {
  const params = new URLSearchParams({ from, to, format });
  const response = await transportFetch(
    `/api/caterer/orders/export?${params.toString()}`,
    { credentials: "include" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "caterer delivery export failed",
    );
  }
  downloadBlob(
    await response.blob(),
    filenameFromDisposition(response) ??
      `lieferliste-${from}-bis-${to}.${format}`,
  );
}
