// API route for fetching scheduled checkouts by student

import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (_request, token, params: Record<string, unknown>) => {
  const studentId = params.studentId as string;

  if (!studentId) {
    throw new Error("Student ID is required");
  }

  // Use internal Docker network URL for server-side calls
  const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV 
    ? 'http://server:8080' 
    : (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080');

  const response = await fetch(
    `${apiUrl}/api/active/scheduled-checkouts/student/${studentId}`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    const error = await response.text();
    // Check if it's a 401 error to trigger token refresh
    if (response.status === 401) {
      throw new Error("API error (401): " + (error || "Unauthorized"));
    }
    throw new Error(error || "Failed to fetch student scheduled checkouts");
  }

  const responseData = await response.json() as { status: string; data: unknown };
  
  // The backend returns { status: "success", data: checkouts[] }
  // We need to return just the data array for the route wrapper
  return responseData.data;
});