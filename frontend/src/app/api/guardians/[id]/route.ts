import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (request, token, params) => {
  const { id } = await params;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/guardians/${id}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});

export const PUT = createPutHandler(async (request, token, params) => {
  const { id } = await params;
  const body = await request.json();

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/guardians/${id}`, {
    method: "PUT",
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

export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id } = await params;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/guardians/${id}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});
