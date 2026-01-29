import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { Session } from "next-auth";

// Type definitions for API responses
interface ErrorResponse {
  error: string;
}

interface SuccessResponse {
  success: boolean;
  data: {
    visit_id: number;
    student_id: number;
    action: string;
  };
}

// Mock apiPost from api-helpers
const mockApiPost = vi.fn();
vi.mock("~/lib/api-helpers", () => ({
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  apiPost: (...args: unknown[]) => mockApiPost(...args),
  handleApiError: (error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal server error";
    return new Response(JSON.stringify({ error: message }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    });
  },
}));

// Create typed mock for auth
const mockAuth = vi.fn<() => Promise<Session | null>>();

// Mock next-auth
vi.mock("~/server/auth", () => ({
  auth: () => mockAuth(),
}));

// Import after mocking
import { POST } from "./route";

describe("POST /api/active/visits/student/[studentId]/checkin", () => {
  const mockToken = "test-jwt-token";

  const createMockSession = (token: string): Session => ({
    user: {
      id: "1",
      token,
      name: "Test User",
      email: "test@example.com",
    },
    expires: new Date(Date.now() + 3600000).toISOString(),
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(createMockSession(mockToken));
  });

  it("returns 401 when no session", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/123/checkin",
      {
        method: "POST",
        body: JSON.stringify({ active_group_id: 456 }),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: "123" }),
    };

    const response = await POST(request, context);

    expect(response.status).toBe(401);
    const data = (await response.json()) as ErrorResponse;
    expect(data.error).toBe("Unauthorized");
  });

  it("returns error when studentId is missing", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/undefined/checkin",
      {
        method: "POST",
        body: JSON.stringify({ active_group_id: 456 }),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: undefined as unknown as string }),
    };

    const response = await POST(request, context);

    expect(response.status).toBe(500);
    const data = (await response.json()) as ErrorResponse;
    expect(data.error).toContain("Student ID is required");
  });

  it("returns error when active_group_id is missing in body", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/123/checkin",
      {
        method: "POST",
        body: JSON.stringify({}),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: "123" }),
    };

    const response = await POST(request, context);

    expect(response.status).toBe(500);
    const data = (await response.json()) as ErrorResponse;
    expect(data.error).toContain("active_group_id is required");
  });

  it("returns error when backend fetch fails", async () => {
    mockApiPost.mockRejectedValue(new Error("Backend error"));

    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/123/checkin",
      {
        method: "POST",
        body: JSON.stringify({ active_group_id: 456 }),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: "123" }),
    };

    const response = await POST(request, context);

    expect(response.status).toBe(500);
    const data = (await response.json()) as ErrorResponse;
    expect(data.error).toContain("Backend error");
  });

  it("returns success with visit data on successful checkin", async () => {
    mockApiPost.mockResolvedValue({
      status: "success",
      message: "Student checked in successfully",
      data: {
        visit_id: 789,
        student_id: 123,
        action: "checked_in",
      },
    });

    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/123/checkin",
      {
        method: "POST",
        body: JSON.stringify({ active_group_id: 456 }),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: "123" }),
    };

    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const data = (await response.json()) as SuccessResponse;
    expect(data.success).toBe(true);
    expect(data.data).toEqual({
      visit_id: 789,
      student_id: 123,
      action: "checked_in",
    });

    // Verify apiPost was called with correct endpoint, token, and body
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits/student/123/checkin",
      mockToken,
      { active_group_id: 456 },
    );
  });

  it("passes through correct student ID from params", async () => {
    mockApiPost.mockResolvedValue({
      status: "success",
      message: "Student checked in",
      data: { visit_id: 1, student_id: 999, action: "checked_in" },
    });

    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/999/checkin",
      {
        method: "POST",
        body: JSON.stringify({ active_group_id: 100 }),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: "999" }),
    };

    await POST(request, context);

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits/student/999/checkin",
      mockToken,
      { active_group_id: 100 },
    );
  });

  it("passes through correct active_group_id in request body", async () => {
    mockApiPost.mockResolvedValue({
      status: "success",
      message: "Student checked in",
      data: { visit_id: 1, student_id: 123, action: "checked_in" },
    });

    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/student/123/checkin",
      {
        method: "POST",
        body: JSON.stringify({ active_group_id: 9999 }),
        headers: { "Content-Type": "application/json" },
      },
    );

    const context = {
      params: Promise.resolve({ studentId: "123" }),
    };

    await POST(request, context);

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits/student/123/checkin",
      mockToken,
      { active_group_id: 9999 },
    );
  });
});
