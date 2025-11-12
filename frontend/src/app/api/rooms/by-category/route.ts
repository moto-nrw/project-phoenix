// app/api/rooms/by-category/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendRoom } from "~/lib/room-helpers";

/**
 * Handler for GET /api/rooms/by-category
 * Returns rooms grouped by their category
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    // Fetch rooms grouped by category from backend API
    return await apiGet<Record<string, BackendRoom[]>>(
      "/api/rooms/by-category",
      token,
    );
  },
);
