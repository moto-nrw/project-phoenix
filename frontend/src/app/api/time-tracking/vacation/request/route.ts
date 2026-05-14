import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

interface VacationRequestBody {
  date_start: string;
  date_end: string;
  half_day?: boolean;
  note?: string;
  substitute_staff_id?: number;
}

export const POST = createPostHandler<unknown, VacationRequestBody>(
  async (_request: NextRequest, body: VacationRequestBody, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/time-tracking/vacation/request",
      token,
      body,
    );
    return response.data;
  },
);
