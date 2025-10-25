import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (request, token, params) => {
  const { id } = await params;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/students/${id}/guardians`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});

export const POST = createPostHandler(async (request, token, params) => {
  const { id } = await params;
  const body = await request.json();

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/students/${id}/guardians`, {
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
