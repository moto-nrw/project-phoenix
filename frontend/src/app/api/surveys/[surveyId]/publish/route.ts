import { proxyPost } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/surveys/{surveyId}/publish → backend (#4781): publishes the draft; from then on nothing but closing changes.
 */
export const POST = proxyPost(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/publish`,
);
