import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";
import type { CareOfferingOption } from "~/lib/student-api";

const logger = createLogger({ component: "StudentCareOfferingsRoute" });

interface ApiCareOfferingsResponse {
  data: Array<{ id: number; name: string }>;
}

/**
 * Active care offerings that count as care, for the "Nach Angebot"
 * selection of the present-children picker (#4571). Readable with
 * users:read; the offering catalog itself stays behind config:read.
 */
export const GET = createGetHandler(
  async (_request, token): Promise<CareOfferingOption[]> => {
    const response = await apiGet<ApiCareOfferingsResponse>(
      "/api/students/care-offerings",
      token,
    );

    if (!response || !Array.isArray(response.data)) {
      logger.warn("student care offerings response has unexpected structure");
      return [];
    }

    return response.data.map((offering) => ({
      id: String(offering.id),
      name: offering.name,
    }));
  },
);
