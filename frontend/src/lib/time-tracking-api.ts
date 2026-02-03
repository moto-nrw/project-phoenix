// Time tracking API service for check-in/out and history management

import { getSession } from "next-auth/react";
import type {
  StaffAbsence,
  WorkSession,
  WorkSessionBreak,
  WorkSessionEdit,
  WorkSessionHistory,
} from "./time-tracking-helpers";
import {
  mapStaffAbsenceResponse,
  mapWorkSessionResponse,
  mapWorkSessionBreakResponse,
  mapWorkSessionEditResponse,
  mapWorkSessionHistoryResponse,
} from "./time-tracking-helpers";

/**
 * Standard API response wrapper
 */
interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

/**
 * Error response structure
 */
interface ErrorResponse {
  error?: string;
  message?: string;
}

/**
 * Check-in request body
 */
export interface CheckInRequest {
  status: "present" | "home_office";
}

/**
 * Update session request body
 */
export interface UpdateSessionRequest {
  status?: "present" | "home_office";
  checkInTime?: string;
  checkOutTime?: string;
  breakMinutes?: number;
  notes?: string;
  breaks?: Array<{ id: string; durationMinutes: number }>;
}

/**
 * Create absence request body
 */
export interface CreateAbsenceRequest {
  absence_type: string;
  date_start: string;
  date_end: string;
  half_day?: boolean;
  note?: string;
}

/**
 * Update absence request body
 */
export interface UpdateAbsenceRequest {
  absence_type?: string;
  date_start?: string;
  date_end?: string;
  half_day?: boolean;
  note?: string;
}

/**
 * Service class for time tracking API operations
 */
class TimeTrackingService {
  private readonly baseUrl = "/api/time-tracking";

  private async getToken(): Promise<string | undefined> {
    const session = await getSession();
    return session?.user?.token;
  }

  private buildHeaders(
    token: string | undefined,
    withBody: boolean,
  ): HeadersInit {
    return {
      ...(withBody && { "Content-Type": "application/json" }),
      ...(token && { Authorization: `Bearer ${token}` }),
    };
  }

  private async request<T>(
    path: string,
    method: string,
    errorMessage: string,
    body?: unknown,
  ): Promise<ApiResponse<T>> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: this.buildHeaders(token, body !== undefined),
      ...(body !== undefined && { body: JSON.stringify(body) }),
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? errorMessage);
    }

    return (await response.json()) as ApiResponse<T>;
  }

  private async requestVoid(
    path: string,
    method: string,
    errorMessage: string,
  ): Promise<void> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: this.buildHeaders(token, false),
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? errorMessage);
    }
  }

  async checkIn(status: "present" | "home_office"): Promise<WorkSession> {
    const result = await this.request<WorkSession>(
      "/check-in",
      "POST",
      "Failed to check in",
      { status },
    );
    return mapWorkSessionResponse(result.data as never);
  }

  async checkOut(): Promise<WorkSession> {
    const result = await this.request<WorkSession>(
      "/check-out",
      "POST",
      "Failed to check out",
    );
    return mapWorkSessionResponse(result.data as never);
  }

  async getCurrentSession(): Promise<WorkSession | null> {
    const result = await this.request<WorkSession | null>(
      "/current",
      "GET",
      "Failed to get current session",
    );
    return result.data ? mapWorkSessionResponse(result.data as never) : null;
  }

  async getHistory(from: string, to: string): Promise<WorkSessionHistory[]> {
    const params = new URLSearchParams({ from, to });
    const result = await this.request<WorkSessionHistory[]>(
      `/history?${params}`,
      "GET",
      "Failed to get history",
    );
    return result.data.map((session) =>
      mapWorkSessionHistoryResponse(session as never),
    );
  }

  async updateSession(
    id: string,
    updates: UpdateSessionRequest,
  ): Promise<WorkSession> {
    const result = await this.request<WorkSession>(
      `/${id}`,
      "PUT",
      "Failed to update session",
      updates,
    );
    return mapWorkSessionResponse(result.data as never);
  }

  async startBreak(durationMinutes?: number): Promise<WorkSessionBreak> {
    const body = durationMinutes
      ? { planned_duration_minutes: durationMinutes }
      : undefined;
    const result = await this.request<WorkSessionBreak>(
      "/break/start",
      "POST",
      "Failed to start break",
      body,
    );
    return mapWorkSessionBreakResponse(result.data as never);
  }

  async endBreak(): Promise<WorkSession> {
    const result = await this.request<WorkSession>(
      "/break/end",
      "POST",
      "Failed to end break",
    );
    return mapWorkSessionResponse(result.data as never);
  }

  async getSessionBreaks(sessionId: string): Promise<WorkSessionBreak[]> {
    const result = await this.request<WorkSessionBreak[]>(
      `/breaks/${sessionId}`,
      "GET",
      "Failed to get breaks",
    );
    return result.data.map((brk) => mapWorkSessionBreakResponse(brk as never));
  }

  async getSessionEdits(sessionId: string): Promise<WorkSessionEdit[]> {
    const result = await this.request<WorkSessionEdit[]>(
      `/${sessionId}/edits`,
      "GET",
      "Failed to get edits",
    );
    return (result.data ?? []).map((edit) =>
      mapWorkSessionEditResponse(edit as never),
    );
  }

  async getAbsences(from: string, to: string): Promise<StaffAbsence[]> {
    const params = new URLSearchParams({ from, to });
    const result = await this.request<StaffAbsence[]>(
      `/absences?${params}`,
      "GET",
      "Failed to get absences",
    );
    return (result.data ?? []).map((a) => mapStaffAbsenceResponse(a as never));
  }

  async createAbsence(req: CreateAbsenceRequest): Promise<StaffAbsence> {
    const result = await this.request<StaffAbsence>(
      "/absences",
      "POST",
      "Failed to create absence",
      req,
    );
    return mapStaffAbsenceResponse(result.data as never);
  }

  async updateAbsence(
    id: string,
    req: UpdateAbsenceRequest,
  ): Promise<StaffAbsence> {
    const result = await this.request<StaffAbsence>(
      `/absences/${id}`,
      "PUT",
      "Failed to update absence",
      req,
    );
    return mapStaffAbsenceResponse(result.data as never);
  }

  async deleteAbsence(id: string): Promise<void> {
    await this.requestVoid(
      `/absences/${id}`,
      "DELETE",
      "Failed to delete absence",
    );
  }
}

export const timeTrackingService = new TimeTrackingService();
