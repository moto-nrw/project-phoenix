import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";

export const PUT = createPutHandler(async (request, token, params) => {
  const { id, guardianId } = await params;
  const body = await request.json();

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/students/${id}/guardians/${guardianId}`,
    {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    }
  );

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});

export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { id, guardianId } = await params;

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/students/${id}/guardians/${guardianId}`,
    {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json();
});
