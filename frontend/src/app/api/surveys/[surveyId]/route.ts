import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";

function surveyPath(params: Record<string, unknown>): string {
  return `/api/surveys/${encodeURIComponent(String(params.surveyId))}`;
}

/**
 * Proxy /api/surveys/{surveyId} → backend (#4781): read one Umfrage, change
 * a draft (the backend refuses published ones with
 * communication.survey_not_draft) or delete it with all its answers.
 */
export const GET = proxyGet(surveyPath);
export const PUT = proxyPut(surveyPath);
export const DELETE = proxyDelete(surveyPath);
