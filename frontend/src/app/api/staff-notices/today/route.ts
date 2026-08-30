import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-notices/today → Backend. Die heute geltenden
 * Tagesinformationen für die angemeldete Person. Lesen darf jede Person mit
 * users:read, also das ganze Team.
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/staff-notices/today",
      token,
    );
    return response.data;
  },
);
