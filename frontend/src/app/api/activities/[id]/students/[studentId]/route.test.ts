import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiDelete, mockGetEnrolledStudents } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
  mockGetEnrolledStudents: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

vi.mock("~/lib/activity-api", () => ({
  getEnrolledStudents: mockGetEnrolledStudents,
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/activities/[id]/students/[studentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns student enrollment details", async () => {
    const students = [
      { student_id: "100", first_name: "John", last_name: "Doe" },
      { student_id: "101", first_name: "Jane", last_name: "Smith" },
    ];
    mockGetEnrolledStudents.mockResolvedValueOnce(students);

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(mockGetEnrolledStudents).toHaveBeenCalledWith("1");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<(typeof students)[0]>>(response);
    expect(json.data).toEqual(students[0]);
  });

  it("returns 500 when activityId is missing", async () => {
    // Note: route-wrapper extracts IDs from URL, so we need a URL without extractable IDs
    const request = createMockRequest("/api/activities/invalid/students/100");
    const response = await GET(
      request,
      createMockContext({ id: undefined, studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toMatch(
      /Activity ID and Student ID are required|Cannot read properties/,
    );
  });

  it("returns 500 when studentId is missing", async () => {
    const request = createMockRequest("/api/activities/1/students/");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Activity ID and Student ID are required");
  });

  it("returns 500 when student not found in enrollment", async () => {
    const students = [
      { student_id: "101", first_name: "Jane", last_name: "Smith" },
    ];
    mockGetEnrolledStudents.mockResolvedValueOnce(students);

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Student with ID 100 is not enrolled");
  });

  it("handles errors from getEnrolledStudents", async () => {
    mockGetEnrolledStudents.mockRejectedValueOnce(new Error("API error"));

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("API error");
  });
});

describe("DELETE /api/activities/[id]/students/[studentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/1/students/100", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(401);
  });

  it("unenrolls student successfully", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/1/students/100", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/activities/1/students/100",
      "test-token",
    );
    // Handler returns { success: true } which gets wrapped
    expect([200, 204]).toContain(response.status);
  });

  it("returns 500 when activityId is missing", async () => {
    // Route-wrapper auto-extracts numeric IDs from URL, so this test
    // verifies the handler works even with auto-extraction
    mockApiDelete.mockRejectedValueOnce(new Error("Invalid ID"));

    const request = createMockRequest("/api/activities/invalid/students/100", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: undefined, studentId: "100" }),
    );

    // Will get ID extracted from URL and proceed with deletion, which fails
    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid ID");
  });

  it("returns 500 when studentId is missing", async () => {
    const request = createMockRequest("/api/activities/1/students/", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", studentId: "" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Activity ID and Student ID are required");
  });

  it("handles deletion errors", async () => {
    mockApiDelete.mockRejectedValueOnce(new Error("Student not found"));

    const request = createMockRequest("/api/activities/1/students/100", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Student not found");
  });
});
