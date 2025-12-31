// lib/scheduled-checkout-api.ts
// API client for scheduled checkout functionality

import axios from "axios";

// Create axios instance for frontend API routes
const api = axios.create({
  baseURL: "", // Use relative URLs to call Next.js API routes
});

// Types for scheduled checkouts
export interface ScheduledCheckout {
  id: number;
  student_id: number;
  scheduled_for: string;
  reason?: string;
  scheduled_by: number; // Staff ID
  status: "pending" | "executed" | "cancelled";
  created_at: string;
  updated_at: string;
}

export interface CreateScheduledCheckoutRequest {
  student_id: number;
  scheduled_for: string;
  reason?: string;
}

// Create a scheduled checkout
export async function createScheduledCheckout(
  data: CreateScheduledCheckoutRequest,
  token?: string,
): Promise<ScheduledCheckout> {
  const response = await api.post<{
    success: boolean;
    message: string;
    data: ScheduledCheckout;
  }>("/api/active/scheduled-checkouts", data, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  // Route wrapper returns { success: true, message: "Success", data: checkout }
  return response.data.data;
}

// Get scheduled checkouts for a student
export async function getStudentScheduledCheckouts(
  studentId: string,
  token?: string,
): Promise<ScheduledCheckout[]> {
  const response = await api.get<{
    success: boolean;
    message: string;
    data: ScheduledCheckout[];
  }>(`/api/active/scheduled-checkouts/student/${studentId}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  // Route wrapper returns { success: true, message: "Success", data: checkouts[] }
  return response.data.data || [];
}

// Get a specific scheduled checkout
export async function getScheduledCheckout(
  checkoutId: string,
  token?: string,
): Promise<ScheduledCheckout> {
  const response = await api.get<{
    success: boolean;
    message: string;
    data: ScheduledCheckout;
  }>(`/api/active/scheduled-checkouts/${checkoutId}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  return response.data.data;
}

// Cancel a scheduled checkout
export async function cancelScheduledCheckout(
  checkoutId: string,
  token?: string,
): Promise<void> {
  await api.delete(`/api/active/scheduled-checkouts/${checkoutId}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
}

// Get all pending scheduled checkouts
export async function getPendingScheduledCheckouts(
  token?: string,
): Promise<ScheduledCheckout[]> {
  const response = await api.get<{
    success: boolean;
    message: string;
    data: ScheduledCheckout[];
  }>("/api/active/scheduled-checkouts/pending", {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  return response.data.data || [];
}

// Perform immediate checkout of a student
export async function performImmediateCheckout(
  studentId: number,
  token?: string,
): Promise<void> {
  await api.post(
    `/api/active/visits/student/${studentId}/checkout`,
    {},
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    },
  );
}
