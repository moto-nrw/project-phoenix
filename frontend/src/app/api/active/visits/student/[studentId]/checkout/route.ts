// API route for immediate student checkout

import { createPostHandler } from "~/lib/route-wrapper";

export const POST = createPostHandler<unknown, Record<string, never>>(
  async (_request, _body, token, params) => {
    const studentId = params.studentId as string;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    // Use internal Docker network URL for server-side calls
    const apiUrl =
      process.env.NODE_ENV === "production" || process.env.DOCKER_ENV
        ? "http://server:8080"
        : (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080");

    const response = await fetch(
      `${apiUrl}/api/active/visits/student/${studentId}/checkout`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({}),
      },
    );

    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || "Failed to checkout student");
    }

    const data = (await response.json()) as {
      status: string;
      message: string;
      data: unknown;
    };
    return data.data;
  },
);
