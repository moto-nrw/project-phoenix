// API route for scheduled checkouts

import type { NextRequest } from "next/server";
import { createPostHandler, createGetHandler } from "~/lib/route-wrapper";

export const POST = createPostHandler(async (request: NextRequest, token: string) => {
  const body = await request.json() as {
    student_id: number;
    scheduled_for: string;
    reason?: string;
  };

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/active/scheduled-checkouts`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(body as Record<string, unknown>),
    }
  );

  if (!response.ok) {
    const error = await response.text();
    throw new Error(String(error) || "Failed to create scheduled checkout");
  }

  return await response.json() as unknown;
});

export const GET = createGetHandler(async (_request: NextRequest, token: string) => {
  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/active/scheduled-checkouts/pending`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || "Failed to fetch pending checkouts");
  }

  return await response.json() as unknown;
});