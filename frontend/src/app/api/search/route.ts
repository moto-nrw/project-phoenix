import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import type { SchoolSearchResult } from "~/lib/school-search";
import {
  mapSchoolSearchResult,
  type BackendSchoolSearchResult,
} from "~/lib/school-search-wire.server";

/**
 * The school search (#6954). The query stays in the body; this handler logs
 * nothing about it.
 */
export const POST = createPostHandler<SchoolSearchResult, { query?: unknown }>(
  async (_request, body, token) => {
    const query = typeof body?.query === "string" ? body.query : "";
    const response = await apiPost<{ data: BackendSchoolSearchResult }>(
      "/api/search",
      token,
      { query },
    );
    return mapSchoolSearchResult(response.data);
  },
);
