import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/surveys/{surveyId}/results → backend (#4781): sums and
 * distributions only. An anonymous Umfrage below the minimum number of
 * answers comes back with `available: false` and no question data.
 */
export const GET = proxyGet(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/results`,
);
