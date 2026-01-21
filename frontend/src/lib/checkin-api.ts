// lib/checkin-api.ts
// API client for student check-in functionality

import axios from "axios";

// Create axios instance for frontend API routes
// BetterAuth: Cookies are automatically included on same-origin requests
const api = axios.create({
  baseURL: "", // Use relative URLs to call Next.js API routes
  withCredentials: true, // Ensure cookies are sent
});

// Perform immediate check-in of a student who is at home
// activeGroupId is required - specifies which room session to check into
// BetterAuth: No token parameter needed - cookies are sent automatically
export async function performImmediateCheckin(
  studentId: number,
  activeGroupId: number,
): Promise<void> {
  await api.post(`/api/active/visits/student/${studentId}/checkin`, {
    active_group_id: activeGroupId,
  });
}
