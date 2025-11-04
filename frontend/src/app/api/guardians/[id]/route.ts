import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";

interface GuardianResponse {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  [key: string]: unknown;
}

export const GET = createGetHandler<GuardianResponse>(async (_request, token, params) => {
  const id = params.id as string;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/guardians/${id}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json() as Promise<GuardianResponse>;
});

export const PUT = createPutHandler<GuardianResponse>(async (_request, body, token, params) => {
  const id = params.id as string;

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

  return response.json() as Promise<GuardianResponse>;
});

export const DELETE = createDeleteHandler<GuardianResponse>(async (_request, token, params) => {
  const id = params.id as string;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/guardians/${id}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json() as Promise<GuardianResponse>;
});
