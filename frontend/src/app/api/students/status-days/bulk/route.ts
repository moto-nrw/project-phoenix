import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface BulkStatusDaysBody {
  student_ids: number[];
  status: string;
  from: string;
  to: string;
  reason?: string;
}

export const POST = createPostHandler<unknown, BulkStatusDaysBody>(
  async (_request: NextRequest, body: BulkStatusDaysBody, token: string) => {
    const response = await apiPost<{ data: unknown }, BulkStatusDaysBody>(
      "/api/students/status-days/bulk",
      token,
      body,
    );
    return response.data;
  },
);
