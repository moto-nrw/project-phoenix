/**
 * Client of the Träger portal ("moto träger", #4649). Talks to the BFF
 * routes under /api/traeger, which forward to the backend with the Träger
 * session. Backend int64 IDs arrive as numbers and leave here as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";
import { filenameFromDisposition } from "./file-download";

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
export function traegerAccessLogQuery(
  filter: TraegerAccessLogFilter,
  page?: number,
): string {
  const params = new URLSearchParams({ from: filter.from, to: filter.to });
  if (filter.schoolId) params.set("school_id", filter.schoolId);
  if (filter.accountId) params.set("account_id", filter.accountId);
  if (filter.kind) params.set("kind", filter.kind);
  if (page !== undefined) params.set("page", String(page));
  return params.toString();
}

async function traegerGet(path: string): Promise<unknown> {
  const response = await transportFetch(path, {
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
  return (data as { data?: unknown } | null)?.data ?? null;
}

/** One page of the access log, newest first. Pages start at 1. */
export async function fetchTraegerAccessLog(
  filter: TraegerAccessLogFilter,
  page: number,
): Promise<TraegerAccessLogPage> {
  const data = (await traegerGet(
    `/api/traeger/access-log?${traegerAccessLogQuery(filter, page)}`,
  )) as { entries?: BackendAccessLogEntry[]; has_more?: boolean } | null;
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
  };
}

/** The people with an entry between `from` and `to`, for the person filter. */
export async function fetchTraegerAccessLogPeople(
  from: string,
  to: string,
): Promise<TraegerAccessLogPerson[]> {
  const query = new URLSearchParams({ from, to }).toString();
  const data = (await traegerGet(`/api/traeger/access-log/people?${query}`)) as
    { account_id: number; name: string }[] | null;
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
