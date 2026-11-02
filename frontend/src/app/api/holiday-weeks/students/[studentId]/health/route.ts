import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-weeks/students/{studentId}/health → backend
 * (#4722): was die bestätigten Ferienanmeldungen der Wochen, die die Schule
 * ausrichtet, zur Gesundheit eines eigenen Kindes angeben. Das Backend
 * schreibt jeden Abruf mit Angaben ins Zugriffsprotokoll; nach dem letzten
 * Tag der Woche ist die Liste leer.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const studentId = requirePathSegmentParam(params, "studentId");
    const response = await apiGet<{ data: unknown }>(
      `/api/holiday-weeks/students/${studentId}/health`,
      token,
    );
    return response.data;
  },
);
