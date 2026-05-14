// Time tracking API service for check-in/out and history management

import { getSession } from "next-auth/react";
import { buildApiError } from "./auth-api";
import type {
  StaffAbsence,
  WeeklySummary,
  WorkSession,
  WorkSessionBreak,
  WorkSessionEdit,
  WorkSessionHistory,
} from "./time-tracking-helpers";
import {
  mapHistoryResponse,
  mapStaffAbsenceResponse,
  mapWorkSessionResponse,
  mapWorkSessionBreakResponse,
  mapWorkSessionEditResponse,
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
 * Stable error code surfaced by the backend when a CheckIn would silently
 * change the status of an existing checked-out session. The page handles
 * this by prompting for a reason and routing the change through
 * UpdateSession (Issue #1368).
 */
export const REOPEN_STATUS_CONFLICT_CODE = "reopen_status_conflict";

/**
 * Update session request body
 */
export interface UpdateSessionRequest {
  date?: string;
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
      throw await buildApiError(response, errorMessage);
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
      throw await buildApiError(response, errorMessage);
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

  async getHistory(
    from: string,
    to: string,
  ): Promise<{
    sessions: WorkSessionHistory[];
    weeklySummaries: WeeklySummary[];
  }> {
    const params = new URLSearchParams({ from, to });
    const result = await this.request<{
      sessions: unknown[];
      weekly_summaries: unknown[];
    }>(`/history?${params}`, "GET", "Failed to get history");
    return mapHistoryResponse(result.data as never);
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
    return (result.data ?? []).map((brk) =>
      mapWorkSessionBreakResponse(brk as never),
    );
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

  async requestVacation(req: VacationRequestPayload): Promise<StaffAbsence> {
    const result = await this.request<StaffAbsence>(
      "/vacation/request",
      "POST",
      "Urlaubsantrag konnte nicht erstellt werden",
      req,
    );
    return mapStaffAbsenceResponse(result.data as never);
  }

  async cancelAbsence(id: string): Promise<void> {
    await this.requestVoid(
      `/absences/${id}/cancel`,
      "POST",
      "Antrag konnte nicht storniert werden",
    );
  }

  async getVacationQuota(year?: number): Promise<VacationQuotaSummary> {
    const params = year ? `?year=${year}` : "";
    const result = await this.request<VacationQuotaSummary>(
      `/vacation/quota${params}`,
      "GET",
      "Urlaubskontingent konnte nicht geladen werden",
    );
    return result.data;
  }
}

export interface VacationRequestPayload {
  date_start: string;
  date_end: string;
  start_half_day: boolean;
  end_half_day: boolean;
  note?: string;
  substitute_staff_id?: number;
}

export interface VacationQuotaSummary {
  staff_id: number;
  year: number;
  entitled_days: number;
  carryover_days: number;
  taken_days: number;
  reserved_days: number;
  remaining_days: number;
}

export const timeTrackingService = new TimeTrackingService();
