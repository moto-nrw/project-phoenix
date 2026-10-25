import { proxyPost } from "~/lib/route-proxy.server";

/**
 * Proxy POST /api/surveys/{surveyId}/close → backend (#4781): ends participation at once.
 */
export const POST = proxyPost(
  (params) =>
    `/api/surveys/${encodeURIComponent(String(params.surveyId))}/close`,
);
