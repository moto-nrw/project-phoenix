import type { NextRequest } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers.server";
import {
  createDeleteHandler,
  createGetHandler,
  createPutHandler,
} from "~/lib/route-wrapper.server";

function announcementID(params: Record<string, unknown>): string {
  const id = params.announcementId as string;
  if (!id) throw new Error("Announcement ID is required");
  return id;
}

/** Proxy GET /api/parent-announcements/{id} → backend (full announcement + targets). */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const response = await apiGet<{ data: unknown }>(
      `/api/parent-announcements/${announcementID(params)}`,
      token,
    );
    return response.data;
  },
);

/** Proxy PUT /api/parent-announcements/{id} → backend (edit content + targets). */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const response = await apiPut<{ data: unknown }>(
      `/api/parent-announcements/${announcementID(params)}`,
      token,
      body,
    );
    return response.data;
  },
);

/** Proxy DELETE /api/parent-announcements/{id} → backend. */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const response = await apiDelete<{ data: unknown }>(
      `/api/parent-announcements/${announcementID(params)}`,
      token,
    );
    // DELETE may return 204 No Content (response is void) — mirror the
    // students/activities proxy convention and fall back to null.
    return response?.data ?? null;
  },
);
