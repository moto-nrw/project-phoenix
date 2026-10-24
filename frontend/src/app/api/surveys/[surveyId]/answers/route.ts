import { proxyPost } from "~/lib/route-proxy.server";

/**
 * Proxy POST /api/surveys/{surveyId}/answers → backend (#4781): submits the
 * caller's answers once. The body passes through untouched and is never
 * logged here.
 */
export const POST = proxyPost(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/answers`,
);
