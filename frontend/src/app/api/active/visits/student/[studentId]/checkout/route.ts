// API route for immediate student checkout

import { createPostHandler } from "~/lib/route-wrapper";
import { apiPost } from "~/lib/api-helpers";

export const POST = createPostHandler<unknown, Record<string, never>>(
  async (_request, _body, token, params) => {
    const studentId = params.studentId as string;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    const data = await apiPost<{
      status: string;
      message: string;
      data: unknown;
    }>(
      `/api/active/visits/student/${studentId}/checkout`,
      token,
      {},
    );

    return data.data;
  },
);
