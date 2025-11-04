import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

interface GuardianWithRelationship {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  relationship_type?: string;
  is_primary?: boolean;
  [key: string]: unknown;
}

export const GET = createGetHandler<GuardianWithRelationship[]>(async (_request, token, params) => {
  const id = params.id as string;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/students/${id}/guardians`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json() as Promise<GuardianWithRelationship[]>;
});

export const POST = createPostHandler<GuardianWithRelationship>(async (_request, body, token, params) => {
  const id = params.id as string;

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

  return response.json() as Promise<GuardianWithRelationship>;
});
