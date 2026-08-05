import { apiGet, apiPut } from "./api-helpers.server";

/**
 * Server-side birthday calls used by the BFF routes (#1542).
 *
 * These forward the backend payload verbatim; the snake_case → camelCase
 * mapping happens client-side in birthdays-api.ts, like the dashboard does.
 */

interface BackendOverviewEnvelope {
  data: unknown;
}

export async function fetchBirthdayOverview(token: string): Promise<unknown> {
  const response = await apiGet<BackendOverviewEnvelope>(
    "/api/birthdays",
    token,
  );
  return response.data;
}

export async function fetchBirthdayOptOut(token: string): Promise<unknown> {
  const response = await apiGet<BackendOverviewEnvelope>(
    "/api/birthdays/opt-out",
    token,
  );
  return response.data;
}

export async function updateBirthdayOptOut(
  token: string,
  optOut: boolean,
): Promise<unknown> {
  const response = await apiPut<BackendOverviewEnvelope, { opt_out: boolean }>(
    "/api/birthdays/opt-out",
    token,
    { opt_out: optOut },
  );
  return response.data;
}
