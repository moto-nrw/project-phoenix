import type { NextRequest } from "next/server";

import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface StudentDeletionImpact {
  confirmation_name: string;
  fingerprint: string;
  total: number;
  counts: Record<string, number>;
  preserved: Record<string, boolean>;
}

interface BackendResponse {
  data: StudentDeletionImpact;
}

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) {
      throw new Error("Student ID is required");
    }
    const response = await apiGet<BackendResponse>(
      `/api/students/${encodeURIComponent(id)}/delete-impact`,
      token,
    );
    return response.data;
  },
);
