// app/api/admin/grade-transitions/classes/route.ts
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface ClassesResponse {
  data: string[];
}

export const GET = createGetHandler(
  async (_request, token): Promise<string[]> => {
    const response = await apiGet<ClassesResponse>(
      "/api/admin/grade-transitions/classes",
      token,
    );
    return response.data ?? [];
  },
);
