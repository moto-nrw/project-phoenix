import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";

// Mock next-auth
vi.mock("~/server/auth", () => ({
  auth: vi.fn(),
}));

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Import after mocking to get mocked version
import { auth } from "~/server/auth";
const mockAuth = vi.mocked(auth);

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

describe("POST /api/active/visits/student/[studentId]/checkin", () => {
  const mockToken = "test-jwt-token";

  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { token: mockToken },
      expires: new Date(Date.now() + 3600000).toISOString(),
    });
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
    mockFetch.mockResolvedValue({
      ok: false,
      text: async () => "Backend error",
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

    expect(response.status).toBe(500);
    const data = (await response.json()) as ErrorResponse;
    expect(data.error).toContain("Backend error");
  });

  it("returns success with visit data on successful checkin", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        message: "Student checked in successfully",
        data: {
          visit_id: 789,
          student_id: 123,
          action: "checked_in",
        },
      }),
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

    // Verify fetch was called correctly
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/active/visits/student/123/checkin"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: `Bearer ${mockToken}`,
          "Content-Type": "application/json",
        }) as Record<string, string>,
        body: JSON.stringify({ active_group_id: 456 }),
      }),
    );
  });

  it("passes through correct student ID from params", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        message: "Student checked in",
        data: { visit_id: 1, student_id: 999, action: "checked_in" },
      }),
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

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/active/visits/student/999/checkin"),
      expect.anything(),
    );
  });

  it("passes through correct active_group_id in request body", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        message: "Student checked in",
        data: { visit_id: 1, student_id: 123, action: "checked_in" },
      }),
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

    expect(mockFetch).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        body: JSON.stringify({ active_group_id: 9999 }),
      }),
    );
  });
});
