// API route for immediate student checkout

import { createPostHandler } from "~/lib/route-wrapper.server";
import { apiPost } from "~/lib/api-helpers.server";

export const POST = createPostHandler<unknown, Record<string, never>>(
  async (_request, _body, token, params) => {
    const studentId = params.studentId as string;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    const response = await apiPost<{ data: unknown }>(
      `/api/active/visits/student/${encodeURIComponent(studentId)}/checkout`,
      token,
      {},
    );
    return response.data;
  },
);
