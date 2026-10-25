/**
 * Client of the Träger portal ("moto träger", #4649). Talks to the BFF
 * routes under /api/traeger, which forward to the backend with the Träger
 * session. Backend int64 IDs arrive as numbers and leave here as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";

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
    };

interface BackendLoginResponse {
  status?: string;
  access_token?: string;
  refresh_token?: string;
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
 * belongs to several Träger gets the choice back instead of tokens.
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

async function traegerData<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await transportFetch(url, {
    credentials: "include",
    cache: "no-store",
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
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

const holidayPeriodsPath = "/api/traeger/holiday-periods";

/** Every holiday period of the signed-in person's Träger, newest first. */
export async function fetchHolidayPeriods(): Promise<HolidayPeriod[]> {
  const periods =
    (await traegerData<BackendHolidayPeriod[] | null>(holidayPeriodsPath)) ??
    [];
  return periods.map(holidayPeriodFromBackend);
}

/** One holiday period of the Träger. */
export async function fetchHolidayPeriod(id: string): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerData<BackendHolidayPeriod>(
      `${holidayPeriodsPath}/${encodeURIComponent(id)}`,
    ),
  );
}

/** Creates a holiday period as a draft. */
export async function createHolidayPeriod(
  input: HolidayPeriodInput,
): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerData<BackendHolidayPeriod>(holidayPeriodsPath, {
      method: "POST",
      body: JSON.stringify(holidayPeriodToBackend(input)),
    }),
  );
}

/** Saves the whole period; weeks and time models left out are removed. */
export async function updateHolidayPeriod(
  id: string,
  input: HolidayPeriodInput,
): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerData<BackendHolidayPeriod>(
      `${holidayPeriodsPath}/${encodeURIComponent(id)}`,
      { method: "PUT", body: JSON.stringify(holidayPeriodToBackend(input)) },
    ),
  );
}

/** Publishes a complete period for registration. */
export async function publishHolidayPeriod(id: string): Promise<HolidayPeriod> {
  return holidayPeriodFromBackend(
    await traegerData<BackendHolidayPeriod>(
      `${holidayPeriodsPath}/${encodeURIComponent(id)}/publish`,
      { method: "POST" },
    ),
  );
}
