// lib/scheduled-checkout-api.ts
// API client for scheduled checkout functionality

import axios from "axios";
import { env } from "~/env";

// Create axios instance
const api = axios.create({
  baseURL: env.NEXT_PUBLIC_API_URL,
});

// Types for scheduled checkouts
export interface ScheduledCheckout {
  id: number;
  student_id: number;
  scheduled_for: string;
  reason?: string;
  scheduled_by: string;
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
  token?: string
): Promise<ScheduledCheckout> {
  const response = await api.post<{ data: ScheduledCheckout }>(
    "/active/scheduled-checkouts",
    data,
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }
  );
  return response.data.data;
}

// Get scheduled checkouts for a student
export async function getStudentScheduledCheckouts(
  studentId: string,
  token?: string
): Promise<ScheduledCheckout[]> {
  const response = await api.get<{ data: ScheduledCheckout[] }>(
    `/active/scheduled-checkouts/student/${studentId}`,
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }
  );
  return response.data.data;
}

// Get a specific scheduled checkout
export async function getScheduledCheckout(
  checkoutId: string,
  token?: string
): Promise<ScheduledCheckout> {
  const response = await api.get<{ data: ScheduledCheckout }>(
    `/active/scheduled-checkouts/${checkoutId}`,
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }
  );
  return response.data.data;
}

// Cancel a scheduled checkout
export async function cancelScheduledCheckout(
  checkoutId: string,
  token?: string
): Promise<void> {
  await api.delete(`/active/scheduled-checkouts/${checkoutId}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
}

// Get all pending scheduled checkouts
export async function getPendingScheduledCheckouts(
  token?: string
): Promise<ScheduledCheckout[]> {
  const response = await api.get<{ data: ScheduledCheckout[] }>(
    "/active/scheduled-checkouts/pending",
    {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }
  );
  return response.data.data;
}