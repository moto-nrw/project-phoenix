import { proxyGet } from "~/lib/route-proxy.server";

/**
 * Proxy GET /api/surveys/{surveyId}/respondents → backend (#4781): answers per
 * person, only for Umfragen that are not anonymous. An anonymous one answers
 * 409 communication.survey_anonymous.
 */
export const GET = proxyGet(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/respondents`,
);
