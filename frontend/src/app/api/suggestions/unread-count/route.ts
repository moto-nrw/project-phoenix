import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

interface BackendUnreadCountResponse {
  status: string;
  data: { unread_count: number };
}

export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<BackendUnreadCountResponse>(
      "/api/suggestions/unread-count",
      token,
    );
    return response.data;
  },
);
