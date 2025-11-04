import { createGetHandler } from "~/lib/route-wrapper";

interface StudentResponse {
  id: number;
  person_id: number;
  first_name: string;
  last_name: string;
  [key: string]: unknown;
}

export const GET = createGetHandler<StudentResponse[]>(async (_request, token, params) => {
  const id = String(params.id);

  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/guardians/${id}/students`,
    {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    throw new Error(`Backend error: ${response.statusText}`);
  }

  return response.json() as Promise<StudentResponse[]>;
});
