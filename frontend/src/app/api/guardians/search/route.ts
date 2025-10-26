import { createGetHandler } from "~/lib/route-wrapper";

interface GuardianSearchResult {
  id: number;
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  [key: string]: unknown;
}

export const GET = createGetHandler<GuardianSearchResult[]>(async (request, token) => {
  const { searchParams } = new URL(request.url);

  // Forward all query parameters to backend
  const params = new URLSearchParams();
  searchParams.forEach((value, key) => {
    params.append(key, value);
  });

  const url = `/api/guardians/search${params.toString() ? `?${params.toString()}` : ""}`;

  const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}${url}`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json() as Promise<GuardianSearchResult[]>;
});
