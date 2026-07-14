import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * GET /api/staff/[id]/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Admin endpoint: list absences (Krank/Urlaub/etc.) for a specific staff
 * member. Mirrors /api/staff/[id]/time-tracking/history so the staff detail
 * view can render absence rows next to work sessions.
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const searchParams = request.nextUrl.searchParams;
    const from = searchParams.get("from");
    const to = searchParams.get("to");

    const response = await apiGet<{ data: unknown }>(
      `/api/staff/${id}/absences?from=${from}&to=${to}`,
      token,
    );
    return response.data;
  },
);

/**
 * POST /api/staff/[id]/absences
 * Admin endpoint: create an absence for a specific staff member. A `sick`
 * absence triggers the Dienst-/Betreuungsplan cascade backend-side (#1843).
 * The body is forwarded verbatim; the backend owns validation.
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiPost<{ data: unknown }>(
      `/api/staff/${id}/absences`,
      token,
      body,
    );
    return response.data;
  },
);
