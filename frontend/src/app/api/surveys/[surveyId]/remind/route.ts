import { proxyPost } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/surveys/{surveyId}/remind → backend (#4781): reminds everyone who has not taken part yet. The answer carries only the count, never who.
 */
export const POST = proxyPost(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/remind`,
);
