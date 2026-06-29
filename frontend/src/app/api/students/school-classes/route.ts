import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentSchoolClassesRoute" });

interface ApiSchoolClassesResponse {
  data: string[];
}

export const GET = createGetHandler(
  async (_request, token): Promise<string[]> => {
    const response = await apiGet<ApiSchoolClassesResponse>(
      "/api/students/school-classes",
      token,
    );

    if (!response || !Array.isArray(response.data)) {
      logger.warn(
        "student school classes API response has unexpected structure",
      );
      return [];
    }

    return response.data.filter(
      (item): item is string => typeof item === "string",
    );
  },
);
