import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers";
import { createDeleteHandler } from "~/lib/route-wrapper";

export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const statusDayId = params.statusDayId as string;
    if (!id || !statusDayId) {
      throw new Error("Student ID and status day ID are required");
    }
    await apiDelete(`/api/students/${id}/status-days/${statusDayId}`, token);
    return { deleted: true };
  },
);
