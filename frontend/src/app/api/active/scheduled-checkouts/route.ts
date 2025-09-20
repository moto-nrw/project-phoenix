// API route for scheduled checkouts

import type { NextRequest } from "next/server";
import { createPostHandler, createGetHandler } from "~/lib/route-wrapper";

interface ScheduledCheckoutBody {
  student_id: number;
  scheduled_for: string;
  reason?: string;
}

export const POST = createPostHandler<unknown, ScheduledCheckoutBody>(
  async (_request: NextRequest, body: ScheduledCheckoutBody, token: string, _params: Record<string, unknown>) => {

  // Use internal Docker network URL for server-side calls
  const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV 
    ? 'http://server:8080' 
    : (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080');

  const response = await fetch(
    `${apiUrl}/api/active/scheduled-checkouts`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(body),
    }
  );

  if (!response.ok) {
    const error = await response.text();
    // Check if it's a 401 error to trigger token refresh
    if (response.status === 401) {
      throw new Error("API error (401): " + (error || "Unauthorized"));
    }
    throw new Error(String(error) || "Failed to create scheduled checkout");
  }

  const responseData = await response.json() as { status: string; data: unknown; message: string };
  
  // The backend returns { status: "success", data: checkout, message: "..." }
  // We return just the data for the route wrapper to format
  return responseData.data;
});

export const GET = createGetHandler(async (_request: NextRequest, token: string) => {
  // Use internal Docker network URL for server-side calls
  const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV 
    ? 'http://server:8080' 
    : (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080');

  const response = await fetch(
    `${apiUrl}/api/active/scheduled-checkouts/pending`,
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
    throw new Error(error || "Failed to fetch pending checkouts");
  }

  const responseData = await response.json() as { status: string; data: unknown };
  
  // The backend returns { status: "success", data: checkouts[] }
  // We return just the data for the route wrapper to format
  return responseData.data;
});