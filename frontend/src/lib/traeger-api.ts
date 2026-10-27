/**
 * Client of the Träger portal ("moto träger", #4649). Talks to the BFF
 * routes under /api/traeger, which forward to the backend with the Träger
 * session. Backend int64 IDs arrive as numbers and leave here as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";
import type { SettingsSchema } from "./settings-api";
import { filenameFromDisposition } from "./file-download";
import { type TraegerRole, traegerRoles } from "./traeger-roles";

/** A Träger the person can open, offered when they belong to several. */
export interface TraegerChoice {
  readonly id: string;
  readonly name: string;
}

export type TraegerLoginResult =
  | {
      readonly status: "authenticated";
      readonly accessToken: string;
      readonly refreshToken: string;
    }
  | {
      readonly status: "organization_selection_required";
      readonly organizations: readonly TraegerChoice[];
    }
  | {
      /** The account has a second factor: a code was mailed (#4651). */
      readonly status: "mfa_required";
      readonly challengeToken: string;
      readonly maskedEmail: string;
      readonly organizationId: string;
    }
  | {
      /** The account sets up its second factor first. */
      readonly status: "mfa_enrollment_required";
      readonly enrollmentToken: string;
      readonly maskedEmail: string;
      readonly organizationId: string;
    };

interface BackendLoginResponse {
  status?: string;
  access_token?: string;
  refresh_token?: string;
  challenge_token?: string;
  masked_email?: string;
  organization_id?: number;
  organizations?: { id: number; name: string }[];
}

export interface TraegerSchool {
  readonly id: string;
  readonly name: string;
  readonly slug: string;
  readonly subdomain: string;
  /** Empty when the school has no city on record. */
  readonly city: string;
}

interface BackendSchool {
  id: number;
  name: string;
  slug: string;
  subdomain: string;
  city?: string | null;
}

async function readJson(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

/**
 * Signs in at the Träger portal. Without `organizationId` a person who
 * belongs to several Träger gets the choice back instead of tokens. Every
 * Träger session needs the second factor: the answer is the mailed code to
 * enter, or the setup of the factor (#4651).
 */
export async function traegerLogin(input: {
  readonly email: string;
  readonly password: string;
  readonly organizationId?: string;
}): Promise<TraegerLoginResult> {
  const body: Record<string, unknown> = {
    email: input.email,
    password: input.password,
  };
  if (input.organizationId !== undefined) {
    body.organization_id = Number(input.organizationId);
  }
  const response = await transportFetch("/api/traeger/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }

  const result = (data ?? {}) as BackendLoginResponse;
  if (result.status === "organization_selection_required") {
    return {
      status: "organization_selection_required",
      organizations: (result.organizations ?? []).map((organization) => ({
        id: String(organization.id),
        name: organization.name,
      })),
    };
  }
  if (result.status === "mfa_required" && result.challenge_token) {
    return {
      status: "mfa_required",
      challengeToken: result.challenge_token,
      maskedEmail: result.masked_email ?? "",
      organizationId: String(result.organization_id ?? ""),
    };
  }
  if (result.status === "mfa_enrollment_required" && result.access_token) {
    return {
      status: "mfa_enrollment_required",
      enrollmentToken: result.access_token,
      maskedEmail: result.masked_email ?? "",
      organizationId: String(result.organization_id ?? ""),
    };
  }
  if (!result.access_token || !result.refresh_token) {
    throw apiErrorFromBody("Login response without tokens", 502, data);
  }
  return {
    status: "authenticated",
    accessToken: result.access_token,
    refreshToken: result.refresh_token,
  };
}

/** The schools of the signed-in person's Träger. */
export async function fetchTraegerSchools(): Promise<TraegerSchool[]> {
  const response = await transportFetch("/api/traeger/schools", {
    credentials: "include",
    cache: "no-store",
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  const schools = ((data as { data?: BackendSchool[] } | null)?.data ??
    []) as BackendSchool[];
  return schools.map((school) => ({
    id: String(school.id),
    name: school.name,
    slug: school.slug,
    subdomain: school.subdomain,
    city: school.city ?? "",
  }));
}

/**
 * Starts the Absprung into a school of the Träger (#4654) and returns the
 * short-lived hand-off token the school's host redeems into its own session.
 */
export async function startTraegerSchoolSession(
  schoolId: string,
): Promise<string> {
  const response = await transportFetch("/api/traeger/school-sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ school_id: Number(schoolId) }),
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  const token = (data as { data?: { handoff_token?: string } } | null)?.data
    ?.handoff_token;
  if (!token) {
    throw apiErrorFromBody("School session without hand-off", 502, data);
  }
  return token;
}

// --- Ferienzeiträume (#4659) ---

/** A bookable care time of a holiday week; the price is in whole cent. */
export interface HolidayTimeModel {
  /** Absent for a time model the form adds. */
  readonly id?: string;
  readonly name: string;
  /** Wall-clock "HH:MM". */
  readonly startsAt: string;
  readonly endsAt: string;
  readonly priceCents: number;
}

/** A bookable week of a holiday period with the school that hosts it. */
export interface HolidayWeek {
  readonly id?: string;
  /** Calendar dates "YYYY-MM-DD". */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolId: string;
  readonly timeModels: readonly HolidayTimeModel[];
}

/** What the Träger-Büro writes: the whole period with its weeks. */
export interface HolidayPeriodInput {
  readonly name: string;
  readonly registrationOpensOn: string;
  readonly registrationClosesOn: string;
  readonly binding: boolean;
  readonly notes: string;
  readonly weeks: readonly HolidayWeek[];
}

export interface HolidayPeriod extends HolidayPeriodInput {
  readonly id: string;
  readonly status: "draft" | "published";
  readonly publishedAt: string | null;
  /** Key of the period's one public registration link (#4661). */
  readonly publicToken: string;
  /** Registrations received so far, a summed figure. */
  readonly registrationCount: number;
}

interface BackendTimeModel {
  id: number;
  name: string;
  starts_at: string;
  ends_at: string;
  price_cents: number;
}

interface BackendWeek {
  id: number;
  starts_on: string;
  ends_on: string;
  host_school_id: number;
  time_models?: BackendTimeModel[] | null;
}

interface BackendHolidayPeriod {
  id: number;
  name: string;
  registration_opens_on: string;
  registration_closes_on: string;
  binding: boolean;
  notes: string;
  status: "draft" | "published";
  published_at: string | null;
  public_token?: string;
  registration_count?: number;
  weeks?: BackendWeek[] | null;
}

function holidayPeriodFromBackend(period: BackendHolidayPeriod): HolidayPeriod {
  return {
    id: String(period.id),
    name: period.name,
    registrationOpensOn: period.registration_opens_on,
    registrationClosesOn: period.registration_closes_on,
    binding: period.binding,
    notes: period.notes,
    status: period.status,
    publishedAt: period.published_at,
    publicToken: period.public_token ?? "",
    registrationCount: period.registration_count ?? 0,
    weeks: (period.weeks ?? []).map((week) => ({
      id: String(week.id),
      startsOn: week.starts_on,
      endsOn: week.ends_on,
      hostSchoolId: String(week.host_school_id),
      timeModels: (week.time_models ?? []).map((model) => ({
        id: String(model.id),
        name: model.name,
        startsAt: model.starts_at,
        endsAt: model.ends_at,
        priceCents: model.price_cents,
      })),
    })),
  };
}

function holidayPeriodToBackend(input: HolidayPeriodInput) {
  return {
    name: input.name,
    registration_opens_on: input.registrationOpensOn,
    registration_closes_on: input.registrationClosesOn,
    binding: input.binding,
    notes: input.notes,
    weeks: input.weeks.map((week) => ({
      ...(week.id ? { id: Number(week.id) } : {}),
      starts_on: week.startsOn,
      ends_on: week.endsOn,
      host_school_id: Number(week.hostSchoolId),
      time_models: week.timeModels.map((model) => ({
        ...(model.id ? { id: Number(model.id) } : {}),
        name: model.name,
        starts_at: model.startsAt,
        ends_at: model.endsAt,
        price_cents: model.priceCents,
      })),
    })),
  };
}

// --- Träger-Büro: Personen und Einladungen (#4650) ---

/** An active person of the Träger-Büro. */
export interface TraegerMember {
  readonly accountId: string;
  readonly email: string;
  readonly roles: readonly TraegerRole[];
}

/** An open invitation into the Träger-Büro. */
export interface TraegerInvitation {
  readonly id: string;
  readonly email: string;
  readonly roles: readonly TraegerRole[];
  /** Instant (RFC 3339) after which the link no longer works. */
  readonly expiresAt: string;
}

export interface TraegerBuero {
  readonly members: readonly TraegerMember[];
  readonly invitations: readonly TraegerInvitation[];
}

/** What the invitation link shows before it is accepted. */
export interface TraegerInvitationPreview {
  readonly organizationName: string;
  readonly email: string;
  readonly roles: readonly TraegerRole[];
  /** The address has a moto account: the invitee confirms its password. */
  readonly requiresExistingPassword: boolean;
  readonly expiresAt: string;
}

interface BackendMember {
  account_id: number;
  email: string;
  roles?: string[] | null;
}

interface BackendInvitation {
  id: number;
  email: string;
  roles?: string[] | null;
  expires_at: string;
}

interface BackendPreview {
  organization_name: string;
  email: string;
  roles?: string[] | null;
  requires_existing_password: boolean;
  expires_at: string;
}

/** Sends a request to a Träger BFF route and returns the `data` member. */
export async function traegerRequest<T>(
  path: string,
  init: { method: string; body?: unknown } = { method: "GET" },
): Promise<T> {
  const response = await transportFetch(path, {
    method: init.method,
    credentials: "include",
    cache: "no-store",
    headers:
      init.body === undefined
        ? undefined
        : { "Content-Type": "application/json" },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  return (data as { data?: T } | null)?.data as T;
}

function mapInvitation(invitation: BackendInvitation): TraegerInvitation {
  return {
    id: String(invitation.id),
    email: invitation.email,
    roles: traegerRoles(invitation.roles ?? []),
    expiresAt: invitation.expires_at,
  };
}

/** The people and the open invitations of the Träger-Büro. */
export async function fetchTraegerBuero(): Promise<TraegerBuero> {
  const data = await traegerRequest<{
    members?: BackendMember[];
    invitations?: BackendInvitation[];
  } | null>("/api/traeger/accounts");
  return {
    members: (data?.members ?? []).map((member) => ({
      accountId: String(member.account_id),
      email: member.email,
      roles: traegerRoles(member.roles ?? []),
    })),
    invitations: (data?.invitations ?? []).map(mapInvitation),
  };
}

/** Invites a person by e-mail; the backend sends the link. */
export async function inviteTraegerAccount(input: {
  readonly email: string;
  readonly roles: readonly TraegerRole[];
}): Promise<void> {
  await traegerRequest("/api/traeger/accounts/invitations", {
    method: "POST",
    body: { email: input.email, roles: input.roles },
  });
}

export async function revokeTraegerInvitation(id: string): Promise<void> {
  await traegerRequest(
    `/api/traeger/accounts/invitations/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function changeTraegerRoles(
  accountId: string,
  roles: readonly TraegerRole[],
): Promise<void> {
  await traegerRequest(
    `/api/traeger/accounts/${encodeURIComponent(accountId)}/roles`,
    { method: "PUT", body: { roles } },
  );
}

export async function removeTraegerMember(accountId: string): Promise<void> {
  await traegerRequest(
    `/api/traeger/accounts/${encodeURIComponent(accountId)}`,
    {
      method: "DELETE",
    },
  );
}

/** Reads a redeemable invitation behind the link token. */
export async function fetchTraegerInvitation(
  token: string,
): Promise<TraegerInvitationPreview> {
  const data = await traegerRequest<BackendPreview>(
    `/api/traeger/invitations/${encodeURIComponent(token)}`,
  );
  return {
    organizationName: data.organization_name,
    email: data.email,
    roles: traegerRoles(data.roles ?? []),
    requiresExistingPassword: data.requires_existing_password,
    expiresAt: data.expires_at,
  };
}

/** Accepts the invitation and returns the address to sign in with. */
export async function acceptTraegerInvitation(
  token: string,
  input: { readonly password: string; readonly confirmPassword?: string },
): Promise<string> {
  const data = await traegerRequest<{ email: string }>(
    `/api/traeger/invitations/${encodeURIComponent(token)}/accept`,
    {
      method: "POST",
      body: {
        password: input.password,
        confirm_password: input.confirmPassword ?? "",
      },
    },
  );
  return data.email;
}

const holidayPeriodsPath = "/api/traeger/holiday-periods";

/** Every holiday period of the signed-in person's Träger, newest first. */
export async function fetchHolidayPeriods(): Promise<HolidayPeriod[]> {
  const periods =
    (await traegerRequest<BackendHolidayPeriod[] | null>(holidayPeriodsPath)) ??
    [];
  return periods.map(holidayPeriodFromBackend);
}

/** One holiday period of the Träger. */
export async function fetchHolidayPeriod(id: string): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerRequest<BackendHolidayPeriod>(
      `${holidayPeriodsPath}/${encodeURIComponent(id)}`,
    ),
  );
}

/** Creates a holiday period as a draft. */
export async function createHolidayPeriod(
  input: HolidayPeriodInput,
): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerRequest<BackendHolidayPeriod>(holidayPeriodsPath, {
      method: "POST",
      body: holidayPeriodToBackend(input),
    }),
  );
}

/** Saves the whole period; weeks and time models left out are removed. */
export async function updateHolidayPeriod(
  id: string,
  input: HolidayPeriodInput,
): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerRequest<BackendHolidayPeriod>(
      `${holidayPeriodsPath}/${encodeURIComponent(id)}`,
      { method: "PUT", body: holidayPeriodToBackend(input) },
    ),
  );
}

/** Publishes a complete period for registration. */
export async function publishHolidayPeriod(id: string): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerRequest<BackendHolidayPeriod>(
      `${holidayPeriodsPath}/${encodeURIComponent(id)}/publish`,
      { method: "POST" },
    ),
  );
}

// --- Zugriffsprotokoll (#4655) ---

/** Whether an access log entry read data or changed it. */
export type TraegerAccessKind = "read" | "change";

/** The filter of the access log (#4655). Days are `YYYY-MM-DD`, both included. */
export interface TraegerAccessLogFilter {
  readonly from: string;
  readonly to: string;
  /** Empty for every school. */
  readonly schoolId: string;
  /** Empty for every person. */
  readonly accountId: string;
  /** Empty for both kinds. */
  readonly kind: TraegerAccessKind | "";
}

/** One read or change of personal data at a school of the Träger. */
export interface TraegerAccessLogEntry {
  /** Stable across pages. */
  readonly id: string;
  readonly occurredAt: string;
  readonly schoolId: string;
  readonly schoolName: string;
  /** Null when no person is recorded, e.g. for an automatic step. */
  readonly accountId: string | null;
  readonly person: string;
  readonly kind: TraegerAccessKind;
  readonly dataLabel: string;
}

export interface TraegerAccessLogPage {
  readonly entries: TraegerAccessLogEntry[];
  readonly hasMore: boolean;
  /** The instant the list reads up to; later pages of the list send it back. */
  readonly asOf: string;
}

/** A person the access log can be filtered by. */
export interface TraegerAccessLogPerson {
  readonly accountId: string;
  readonly name: string;
}

interface BackendAccessLogEntry {
  id: string;
  occurred_at: string;
  school_id: number;
  school_name: string;
  account_id: number | null;
  person: string;
  kind: TraegerAccessKind;
  data_label: string;
}

/** The query of a filter, as the backend reads it. */
function traegerAccessLogQuery(
  filter: TraegerAccessLogFilter,
  page?: number,
  asOf?: string,
): string {
  const params = new URLSearchParams({ from: filter.from, to: filter.to });
  if (filter.schoolId) params.set("school_id", filter.schoolId);
  if (filter.accountId) params.set("account_id", filter.accountId);
  if (filter.kind) params.set("kind", filter.kind);
  if (page !== undefined) params.set("page", String(page));
  if (asOf) params.set("as_of", asOf);
  return params.toString();
}

/**
 * One page of the access log, newest first. Pages start at 1; the pages after
 * the first pass the first page's `asOf`, so entries recorded meanwhile
 * (including this read itself) never shift into a later page.
 */
export async function fetchTraegerAccessLog(
  filter: TraegerAccessLogFilter,
  page: number,
  asOf?: string,
): Promise<TraegerAccessLogPage> {
  const data = await traegerRequest<{
    entries?: BackendAccessLogEntry[];
    has_more?: boolean;
    as_of?: string;
  } | null>(
    `/api/traeger/access-log?${traegerAccessLogQuery(filter, page, asOf)}`,
  );
  return {
    entries: (data?.entries ?? []).map((entry) => ({
      id: entry.id,
      occurredAt: entry.occurred_at,
      schoolId: String(entry.school_id),
      schoolName: entry.school_name,
      accountId: entry.account_id === null ? null : String(entry.account_id),
      person: entry.person,
      kind: entry.kind,
      dataLabel: entry.data_label,
    })),
    hasMore: data?.has_more ?? false,
    asOf: data?.as_of ?? "",
  };
}

/** The people with an entry between `from` and `to`, for the person filter. */
export async function fetchTraegerAccessLogPeople(
  from: string,
  to: string,
): Promise<TraegerAccessLogPerson[]> {
  const query = new URLSearchParams({ from, to }).toString();
  const data = await traegerRequest<
    { account_id: number; name: string }[] | null
  >(`/api/traeger/access-log/people?${query}`);
  return (data ?? []).map((person) => ({
    accountId: String(person.account_id),
    name: person.name,
  }));
}

/** The CSV export of a filter, with the file name the backend chose. */
export async function downloadTraegerAccessLog(
  filter: TraegerAccessLogFilter,
): Promise<{ readonly blob: Blob; readonly filename: string | null }> {
  const response = await transportFetch(
    `/api/traeger/access-log/export?${traegerAccessLogQuery(filter)}`,
    { credentials: "include", cache: "no-store" },
  );
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      await readJson(response),
    );
  }
  return {
    blob: await response.blob(),
    filename: filenameFromDisposition(response),
  };
}

// --- Träger-Einstellungen (ADR 0086, #4652) ---

const traegerSettingValuePath = (key: string) =>
  `/api/traeger/settings/values/${encodeURIComponent(key)}`;

/**
 * The settings the Träger sets for all its schools: only settings with the
 * levels `beide` or `traeger`, each with the Träger value or the registry
 * default.
 */
export async function fetchTraegerSettingsSchema(): Promise<SettingsSchema> {
  const schema = await traegerRequest<SettingsSchema | null>(
    "/api/traeger/settings/schema",
  );
  return { tabs: schema?.tabs ?? [] };
}

/** Sets the Träger value of one setting. Throws an ApiError on failure. */
export async function saveTraegerSettingValue(
  key: string,
  value: unknown,
): Promise<void> {
  await traegerRequest(traegerSettingValuePath(key), {
    method: "PUT",
    body: { value },
  });
}

/**
 * Removes the Träger value of one setting: schools fall back to the default
 * or keep their own value. Throws an ApiError on failure.
 */
export async function clearTraegerSettingValue(key: string): Promise<void> {
  await traegerRequest(traegerSettingValuePath(key), { method: "DELETE" });
}

/** The Träger of the session, as the portal frame names and marks it. */
export interface TraegerOrganization {
  readonly id: string;
  readonly name: string;
  /** A test Träger with invented sample data (#4658). */
  readonly testSystem: boolean;
}

interface BackendTraegerOrganization {
  id: number;
  name: string;
  test_system?: boolean;
}

/** The Träger of the signed-in person's session. */
export async function fetchTraegerOrganization(): Promise<TraegerOrganization> {
  const organization = await traegerRequest<BackendTraegerOrganization | null>(
    "/api/traeger/organization",
  );
  if (!organization) {
    throw apiErrorFromBody("Träger response without data", 502, organization);
  }
  return {
    id: String(organization.id),
    name: organization.name,
    testSystem: organization.test_system ?? false,
  };
}

/** The largest logo the backend takes (#4653, ADR 0112). */
export const TRAEGER_LOGO_MAX_BYTES = 1024 * 1024;

/** The stored logo of the Träger, without its bytes. */
export interface TraegerLogo {
  readonly contentType: string;
  readonly width: number;
  readonly height: number;
  readonly sizeBytes: number;
  readonly updatedAt: string;
}

/** The contact person documents of the Träger name. */
export interface TraegerContact {
  readonly name: string;
  readonly phone: string;
  readonly email: string;
}

/** A bank account as the Träger-Büro writes it. */
export interface TraegerBankAccountInput {
  readonly label: string;
  readonly accountHolder: string;
  readonly iban: string;
  readonly bic: string;
  readonly bankName: string;
  readonly isDefault: boolean;
}

/** A bank account of the Träger; exactly one is the default. */
export interface TraegerBankAccount extends TraegerBankAccountInput {
  readonly id: string;
}

/** The master data of the Träger (#4653). */
export interface TraegerMasterData {
  readonly contact: TraegerContact;
  readonly logo: TraegerLogo | null;
  readonly bankAccounts: readonly TraegerBankAccount[];
}

interface BackendTraegerLogo {
  content_type: string;
  width: number;
  height: number;
  size_bytes: number;
  updated_at: string;
}

interface BackendTraegerBankAccount {
  id: number;
  label: string;
  account_holder: string;
  iban: string;
  bic: string;
  bank_name: string;
  is_default: boolean;
}

function mapTraegerLogo(logo: BackendTraegerLogo): TraegerLogo {
  return {
    contentType: logo.content_type,
    width: logo.width,
    height: logo.height,
    sizeBytes: logo.size_bytes,
    updatedAt: logo.updated_at,
  };
}

function mapTraegerBankAccount(
  account: BackendTraegerBankAccount,
): TraegerBankAccount {
  return {
    id: String(account.id),
    label: account.label,
    accountHolder: account.account_holder,
    iban: account.iban,
    bic: account.bic,
    bankName: account.bank_name,
    isDefault: account.is_default,
  };
}

function backendBankAccount(input: TraegerBankAccountInput) {
  return {
    label: input.label,
    account_holder: input.accountHolder,
    iban: input.iban,
    bic: input.bic,
    bank_name: input.bankName,
    is_default: input.isDefault,
  };
}

const masterDataPath = "/api/traeger/master-data";
const bankAccountPath = (id: string) =>
  `${masterDataPath}/bank-accounts/${encodeURIComponent(id)}`;

/** The logo as an image source; the version reloads it after a change. */
export function traegerLogoSrc(logo: TraegerLogo): string {
  return `${masterDataPath}/logo?v=${encodeURIComponent(logo.updatedAt)}`;
}

/** Reads the master data of the Träger. The read is logged. */
export async function fetchTraegerMasterData(): Promise<TraegerMasterData> {
  const data = await traegerRequest<{
    contact?: Partial<TraegerContact> | null;
    logo?: BackendTraegerLogo | null;
    bank_accounts?: BackendTraegerBankAccount[] | null;
  } | null>(masterDataPath);
  return {
    contact: {
      name: data?.contact?.name ?? "",
      phone: data?.contact?.phone ?? "",
      email: data?.contact?.email ?? "",
    },
    logo: data?.logo ? mapTraegerLogo(data.logo) : null,
    bankAccounts: (data?.bank_accounts ?? []).map(mapTraegerBankAccount),
  };
}

/**
 * Stores the contact person and returns it as the backend keeps it: trimmed,
 * the e-mail address in lower case. Throws an ApiError on failure.
 */
export async function saveTraegerContact(
  contact: TraegerContact,
): Promise<TraegerContact> {
  const saved = await traegerRequest<Partial<TraegerContact> | null>(
    `${masterDataPath}/contact`,
    { method: "PUT", body: contact },
  );
  return {
    name: saved?.name ?? contact.name,
    phone: saved?.phone ?? contact.phone,
    email: saved?.email ?? contact.email,
  };
}

/** Uploads a PNG or JPEG logo in place of the old one. */
export async function uploadTraegerLogo(file: File): Promise<TraegerLogo> {
  const form = new FormData();
  form.append("logo", file);
  const response = await transportFetch(`${masterDataPath}/logo`, {
    method: "PUT",
    credentials: "include",
    cache: "no-store",
    body: form,
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  const logo = (data as { data?: BackendTraegerLogo } | null)?.data;
  if (!logo) {
    throw apiErrorFromBody("Logo response without data", 502, data);
  }
  return mapTraegerLogo(logo);
}

/** Removes the logo. Throws an ApiError on failure. */
export async function removeTraegerLogo(): Promise<void> {
  await traegerRequest(`${masterDataPath}/logo`, { method: "DELETE" });
}

/** Adds a bank account. Throws an ApiError on failure. */
export async function createTraegerBankAccount(
  input: TraegerBankAccountInput,
): Promise<TraegerBankAccount> {
  return mapTraegerBankAccount(
    await traegerRequest<BackendTraegerBankAccount>(
      `${masterDataPath}/bank-accounts`,
      { method: "POST", body: backendBankAccount(input) },
    ),
  );
}

/** Changes a bank account. Throws an ApiError on failure. */
export async function updateTraegerBankAccount(
  id: string,
  input: TraegerBankAccountInput,
): Promise<TraegerBankAccount> {
  return mapTraegerBankAccount(
    await traegerRequest<BackendTraegerBankAccount>(bankAccountPath(id), {
      method: "PUT",
      body: backendBankAccount(input),
    }),
  );
}

/** Deletes a bank account. Throws an ApiError on failure. */
export async function deleteTraegerBankAccount(id: string): Promise<void> {
  await traegerRequest(bankAccountPath(id), { method: "DELETE" });
}
