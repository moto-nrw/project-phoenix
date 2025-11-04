import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";

interface GuardianResponse {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  [key: string]: unknown;
}

interface GuardiansListResponse {
  data: GuardianResponse[];
  total: number;
  page: number;
  per_page: number;
}

export const GET = createGetHandler<GuardiansListResponse>(async (request, token) => {
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

  return response.json() as Promise<GuardiansListResponse>;
});

export const POST = createPostHandler<GuardianResponse>(async (_request, body, token) => {
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

  return response.json() as Promise<GuardianResponse>;
});
