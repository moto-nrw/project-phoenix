import { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (request, token) => {
  const { searchParams } = new URL(request.url);

  // Forward all query parameters to backend
  const params = new URLSearchParams();
  searchParams.forEach((value, key) => {
    params.append(key, value);
  });

  const url = `/api/guardians${params.toString() ? `?${params.toString()}` : ""}`;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}${url}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});

export const POST = createPostHandler(async (request, token) => {
  const body = await request.json();

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/guardians`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});
