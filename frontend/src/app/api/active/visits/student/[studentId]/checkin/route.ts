// API route for immediate student check-in

import { createPostHandler } from "~/lib/route-wrapper";

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

    // Use internal Docker network URL for server-side calls
    const apiUrl =
      process.env.NODE_ENV === "production" || process.env.DOCKER_ENV
        ? "http://server:8080"
        : (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080");

    const response = await fetch(
      `${apiUrl}/api/active/visits/student/${studentId}/checkin`,
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
      throw new Error(error || "Failed to check in student");
    }

    const data = (await response.json()) as {
      status: string;
      message: string;
      data: unknown;
    };
    return data.data;
  },
);
