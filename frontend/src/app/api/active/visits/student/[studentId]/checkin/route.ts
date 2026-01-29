// API route for immediate student check-in

import { createPostHandler } from "~/lib/route-wrapper";
import { apiPost } from "~/lib/api-helpers";

interface CheckinBody {
  active_group_id: number;
}

export const POST = createPostHandler<unknown, CheckinBody>(
  async (_request, body, token, params) => {
    const studentId = params.studentId as string;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    if (!body.active_group_id) {
      throw new Error("active_group_id is required");
    }

    const data = await apiPost<{
      status: string;
      message: string;
      data: unknown;
    }>(
      `/api/active/visits/student/${studentId}/checkin`,
      token,
      { active_group_id: body.active_group_id },
    );

    return data.data;
  },
);
