// API route for individual scheduled checkouts

import { createGetHandler, createDeleteHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (_request, token, params: Record<string, unknown>) => {
  const id = params.id as string;

  if (!id) {
    throw new Error("Checkout ID is required");
  }

  // Use internal Docker network URL for server-side calls
  const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV 
    ? 'http://server:8080' 
    : (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080');

  const response = await fetch(
    `${apiUrl}/api/active/scheduled-checkouts/${id}`,
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
    throw new Error(error || "Failed to fetch scheduled checkout");
  }

  const responseData = await response.json() as { status: string; data: unknown };
  
  // The backend returns { status: "success", data: checkout }
  // We return just the data for the route wrapper to format
  return responseData.data;
});

export const DELETE = createDeleteHandler(async (_request, token, params: Record<string, unknown>) => {
  const id = params.id as string;

  if (!id) {
    throw new Error("Checkout ID is required");
  }

  // Use internal Docker network URL for server-side calls
  const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV 
    ? 'http://server:8080' 
    : (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080');

  const response = await fetch(
    `${apiUrl}/api/active/scheduled-checkouts/${id}`,
    {
      method: "DELETE",
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
    throw new Error(error || "Failed to cancel scheduled checkout");
  }

  return { success: true };
});