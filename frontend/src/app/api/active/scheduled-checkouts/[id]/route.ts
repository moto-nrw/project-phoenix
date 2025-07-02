// API route for individual scheduled checkouts

import { createGetHandler, createDeleteHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (_request, token, params: Record<string, unknown>) => {
  const id = params.id as string;

  if (!id) {
    throw new Error("Checkout ID is required");
  }

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/active/scheduled-checkouts/${id}`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || "Failed to fetch scheduled checkout");
  }

  return await response.json() as unknown;
});

export const DELETE = createDeleteHandler(async (_request, token, params: Record<string, unknown>) => {
  const id = params.id as string;

  if (!id) {
    throw new Error("Checkout ID is required");
  }

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/active/scheduled-checkouts/${id}`,
    {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    const error = await response.text();
    throw new Error(error || "Failed to cancel scheduled checkout");
  }

  return { success: true };
});