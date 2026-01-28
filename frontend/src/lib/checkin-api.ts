// lib/checkin-api.ts
// API client for student check-in functionality

import axios from "axios";

// Create axios instance for frontend API routes
const api = axios.create({
  baseURL: "", // Use relative URLs to call Next.js API routes
});

// Perform immediate check-in of a student who is at home
// activeGroupId is required - specifies which room session to check into
export async function performImmediateCheckin(
  studentId: number,
  activeGroupId: number,
  token?: string,
): Promise<void> {
  await api.post(
    `/api/active/visits/student/${studentId}/checkin`,
    { active_group_id: activeGroupId },
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    },
  );
}
