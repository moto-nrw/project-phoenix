import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/surveys/{surveyId}/form → backend (#4781): the questionnaire
 * of an open staff Umfrage and whether the caller already took part.
 */
export const GET = proxyGet(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/form`,
);
