import { sessionFetch } from "~/lib/session-cache";

export interface StaffGroupLeaderCandidate {
  id: string;
  teacherId?: string;
  fullName: string;
}

interface BackendStaffGroupLeaderCandidate {
  id: number;
  teacher_id?: number;
  full_name: string;
}

export async function fetchGroupLeaderCandidates(): Promise<
  StaffGroupLeaderCandidate[]
> {
  const response = await sessionFetch("/api/staff/by-role?role=user", {
    method: "GET",
  });
  if (!response.ok) {
    throw new Error("Fachkräfte konnten nicht geladen werden.");
  }
  const data = (await response.json()) as {
    data: BackendStaffGroupLeaderCandidate[] | null;
  };
  if (!Array.isArray(data.data)) {
    throw new Error("Ungültige Antwort für verfügbare Fachkräfte.");
  }
  return data.data.map((staff) => ({
    id: staff.id.toString(),
    teacherId: staff.teacher_id?.toString(),
    fullName: staff.full_name,
  }));
}
