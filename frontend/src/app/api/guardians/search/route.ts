import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (request, token) => {
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

  return response.json();
});
