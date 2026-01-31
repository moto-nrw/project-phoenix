// Time tracking API service for check-in/out and history management

import { getSession } from "next-auth/react";
import type {
  WorkSession,
  WorkSessionBreak,
  WorkSessionHistory,
} from "./time-tracking-helpers";
import {
  mapWorkSessionResponse,
  mapWorkSessionBreakResponse,
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
}

/**
 * Service class for time tracking API operations
 */
class TimeTrackingService {
  private readonly baseUrl = "/api/time-tracking";

  /**
   * Get authentication token from session
   */
  private async getToken(): Promise<string | undefined> {
    const session = await getSession();
    return session?.user?.token;
  }

  /**
   * Check in for work
   * @param status - Work status (present or home_office)
   * @returns Created work session
   */
  async checkIn(status: "present" | "home_office"): Promise<WorkSession> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/check-in`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token && { Authorization: `Bearer ${token}` }),
      },
      body: JSON.stringify({ status }),
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? "Failed to check in");
    }

    const result = (await response.json()) as ApiResponse<WorkSession>;
    return mapWorkSessionResponse(result.data as never);
  }

  /**
   * Check out from work
   * @returns Updated work session
   */
  async checkOut(): Promise<WorkSession> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/check-out`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token && { Authorization: `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? "Failed to check out");
    }

    const result = (await response.json()) as ApiResponse<WorkSession>;
    return mapWorkSessionResponse(result.data as never);
  }

  /**
   * Get current active work session
   * @returns Current session or null if not checked in
   */
  async getCurrentSession(): Promise<WorkSession | null> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/current`, {
      method: "GET",
      headers: {
        ...(token && { Authorization: `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(
        error.error ?? error.message ?? "Failed to get current session",
      );
    }

    const result = (await response.json()) as ApiResponse<WorkSession | null>;
    return result.data ? mapWorkSessionResponse(result.data as never) : null;
  }

  /**
   * Get work session history
   * @param from - Start date (YYYY-MM-DD)
   * @param to - End date (YYYY-MM-DD)
   * @returns Array of work sessions with calculated fields
   */
  async getHistory(from: string, to: string): Promise<WorkSessionHistory[]> {
    const token = await this.getToken();
    const params = new URLSearchParams({ from, to });
    const response = await fetch(`${this.baseUrl}/history?${params}`, {
      method: "GET",
      headers: {
        ...(token && { Authorization: `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? "Failed to get history");
    }

    const result = (await response.json()) as ApiResponse<WorkSessionHistory[]>;
    return result.data.map((session) =>
      mapWorkSessionHistoryResponse(session as never),
    );
  }

  /**
   * Update a work session
   * @param id - Session ID
   * @param updates - Fields to update
   * @returns Updated work session
   */
  async updateSession(
    id: string,
    updates: UpdateSessionRequest,
  ): Promise<WorkSession> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/${id}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        ...(token && { Authorization: `Bearer ${token}` }),
      },
      body: JSON.stringify(updates),
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(
        error.error ?? error.message ?? "Failed to update session",
      );
    }

    const result = (await response.json()) as ApiResponse<WorkSession>;
    return mapWorkSessionResponse(result.data as never);
  }

  /**
   * Start a new break for the current session
   * @returns Created break
   */
  async startBreak(): Promise<WorkSessionBreak> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/break/start`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token && { Authorization: `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? "Failed to start break");
    }

    const result = (await response.json()) as ApiResponse<WorkSessionBreak>;
    return mapWorkSessionBreakResponse(result.data as never);
  }

  /**
   * End the current active break
   * @returns Updated work session
   */
  async endBreak(): Promise<WorkSession> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/break/end`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token && { Authorization: `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? "Failed to end break");
    }

    const result = (await response.json()) as ApiResponse<WorkSession>;
    return mapWorkSessionResponse(result.data as never);
  }

  /**
   * Get breaks for a specific session
   * @param sessionId - Session ID
   * @returns Array of breaks
   */
  async getSessionBreaks(sessionId: string): Promise<WorkSessionBreak[]> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/breaks/${sessionId}`, {
      method: "GET",
      headers: {
        ...(token && { Authorization: `Bearer ${token}` }),
      },
    });

    if (!response.ok) {
      const error = (await response.json()) as ErrorResponse;
      throw new Error(error.error ?? error.message ?? "Failed to get breaks");
    }

    const result = (await response.json()) as ApiResponse<WorkSessionBreak[]>;
    return result.data.map((brk) => mapWorkSessionBreakResponse(brk as never));
  }
}

export const timeTrackingService = new TimeTrackingService();
