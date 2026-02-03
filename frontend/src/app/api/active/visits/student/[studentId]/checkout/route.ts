// API route for immediate student checkout

import { createPostHandler } from "~/lib/route-wrapper";
import { getServerApiUrl } from "~/lib/server-api-url";

export const POST = createPostHandler<unknown, Record<string, never>>(
  async (_request, _body, token, params) => {
    const studentId = params.studentId as string;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    const response = await fetch(
      `${getServerApiUrl()}/api/active/visits/student/${studentId}/checkout`,
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
