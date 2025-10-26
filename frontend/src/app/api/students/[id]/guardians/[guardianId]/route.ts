import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";

interface StudentGuardianResponse {
  id: number;
  student_id: number;
  guardian_id: number;
  relationship_type: string;
  is_primary: boolean;
  [key: string]: unknown;
}

export const PUT = createPutHandler<StudentGuardianResponse>(async (_request, body, token, params) => {
  const id = params.id as string;
  const guardianId = params.guardianId as string;

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

  return response.json() as Promise<StudentGuardianResponse>;
});

export const DELETE = createDeleteHandler<StudentGuardianResponse>(async (_request, token, params) => {
  const id = params.id as string;
  const guardianId = params.guardianId as string;

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

  return response.json() as Promise<StudentGuardianResponse>;
});
