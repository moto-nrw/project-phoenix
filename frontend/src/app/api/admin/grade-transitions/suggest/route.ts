// app/api/admin/grade-transitions/suggest/route.ts
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { BackendSuggestedMapping } from "~/lib/grade-transition-api";

interface SuggestResponse {
  data: BackendSuggestedMapping[];
}

export const GET = createGetHandler(
  async (_request, token): Promise<BackendSuggestedMapping[]> => {
    const response = await apiGet<SuggestResponse>(
      "/api/admin/grade-transitions/suggest",
      token,
    );
    return response.data ?? [];
  },
);
