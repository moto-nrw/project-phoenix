// API route for immediate student check-in

import { createPostHandler } from "~/lib/route-wrapper";
import { getServerApiUrl } from "~/lib/server-api-url";

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

    const response = await fetch(
      `${getServerApiUrl()}/api/active/visits/student/${studentId}/checkin`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ active_group_id: body.active_group_id }),
      },
    );

    if (!response.ok) {
      const error = await response.text();
      // Include status code in error message so handleApiError can extract and propagate it
      throw new Error(
        `API error (${response.status}): ${error || "Failed to check in student"}`,
      );
    }

    const data = (await response.json()) as {
      status: string;
      message: string;
      data: unknown;
    };
    return data.data;
  },
);
