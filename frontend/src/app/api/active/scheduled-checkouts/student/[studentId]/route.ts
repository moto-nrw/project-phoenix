// API route for fetching scheduled checkouts by student

import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (_request, token, params: Record<string, unknown>) => {
  const studentId = params.studentId as string;

  if (!studentId) {
    throw new Error("Student ID is required");
  }

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/active/scheduled-checkouts/student/${studentId}`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || "Failed to fetch student scheduled checkouts");
  }

  return await response.json() as unknown;
});