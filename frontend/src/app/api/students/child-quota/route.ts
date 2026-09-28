import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface ChildQuotaResponse {
  limited: boolean;
  booked_places?: number;
  occupied_places?: number;
}

/**
 * Proxy GET /api/students/child-quota → backend. Liefert das Kinderkontingent
 * der eigenen Schule neben der Kontingentzahl für die Kinderliste der
 * Datenverwaltung (#3569). Das Backend gibt die Route nur mit users:delete frei.
 */
export const GET = createGetHandler<ChildQuotaResponse>(
  async (_request, token) => {
    const response = await apiGet<{ data: ChildQuotaResponse }>(
      "/api/students/child-quota",
      token,
    );
    return response.data;
  },
);
