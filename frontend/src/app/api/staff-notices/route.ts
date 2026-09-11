import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-notices → Backend. Alle Tagesinformationen des
 * Mandanten, neueste zuerst (Leitungssicht, inklusive abgeschalteter). Das
 * Backend verlangt Adminrecht.
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/staff-notices",
      token,
    );
    return response.data;
  },
);

/** Proxy POST /api/staff-notices → Backend. Legt einen Hinweis an. */
export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/staff-notices",
      token,
      body,
    );
    return response.data;
  },
);
