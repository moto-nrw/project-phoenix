/**
 * Client of the Träger portal ("moto träger", #4649). Talks to the BFF
 * routes under /api/traeger, which forward to the backend with the Träger
 * session. Backend int64 IDs arrive as numbers and leave here as strings.
 */

import { apiErrorFromBody, transportFetch } from "./api-error";
import type { SettingsSchema } from "./settings-api";

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

async function failOnError(response: Response): Promise<unknown> {
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  return data;
}

/**
 * The settings the Träger sets for all its schools (ADR 0086, #4652): only
 * settings with the levels `beide` or `traeger`, each with the Träger value
 * or the registry default.
 */
export async function fetchTraegerSettingsSchema(): Promise<SettingsSchema> {
  const response = await transportFetch("/api/traeger/settings/schema", {
    credentials: "include",
    cache: "no-store",
  });
  const data = await failOnError(response);
  const schema = (data as { data?: SettingsSchema } | null)?.data;
  return { tabs: schema?.tabs ?? [] };
}

/** Sets the Träger value of one setting. Throws an ApiError on failure. */
export async function saveTraegerSettingValue(
  key: string,
  value: unknown,
): Promise<void> {
  const response = await transportFetch(
    `/api/traeger/settings/values/${encodeURIComponent(key)}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ value }),
    },
  );
  await failOnError(response);
}

/**
 * Removes the Träger value of one setting: schools fall back to the default
 * or keep their own value. Throws an ApiError on failure.
 */
export async function clearTraegerSettingValue(key: string): Promise<void> {
  const response = await transportFetch(
    `/api/traeger/settings/values/${encodeURIComponent(key)}`,
    { method: "DELETE", credentials: "include" },
  );
  await failOnError(response);
}
