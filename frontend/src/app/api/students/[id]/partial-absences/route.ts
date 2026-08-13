import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

interface PartialAbsenceBody {
  date: string;
  from_time: string;
  reason?: string;
}

export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) throw new Error("Student ID is required");
    const response = await apiGet<{ data: unknown }>(
      `/api/students/${id}/partial-absences${request.nextUrl.search}`,
      token,
    );
    return response.data;
  },
);

export const POST = createPostHandler<unknown, PartialAbsenceBody>(
  async (_request, body, token, params) => {
    const id = params.id as string;
    if (!id) throw new Error("Student ID is required");
    const response = await apiPost<{ data: unknown }, PartialAbsenceBody>(
      `/api/students/${id}/partial-absences`,
      token,
      body,
    );
    return response.data;
  },
);
