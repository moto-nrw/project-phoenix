// API route for immediate student check-in

import { createPostHandler } from "~/lib/route-wrapper.server";
import { apiPost } from "~/lib/api-helpers.server";

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

    const response = await apiPost<{ data: unknown }>(
      `/api/active/visits/student/${encodeURIComponent(studentId)}/checkin`,
      token,
      { active_group_id: body.active_group_id },
    );
    return response.data;
  },
);
