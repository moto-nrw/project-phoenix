import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiDelete, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: vi.fn(),
    apiPut: vi.fn(),
    apiDelete: mockApiDelete,
    handleApiError: vi.fn(mockHandleApiError),
  };
});

// ============================================================================
// Test Helpers
// ============================================================================

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
      { id: 100, first_name: "John", last_name: "Doe" },
      { id: 101, first_name: "Jane", last_name: "Smith" },
    ];
    mockApiGet.mockResolvedValueOnce({ data: students });

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/1/students",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        id: string;
        student_id: string;
        name: string;
      }>
    >(response);
    expect(json.data.id).toBe("100");
    expect(json.data.student_id).toBe("100");
    expect(json.data.name).toBe("John Doe");
  });

  it("returns 500 when activityId is missing", async () => {
    // Note: route-wrapper extracts IDs from URL, so we need a URL without extractable IDs
    const request = createMockRequest("/api/activities/invalid/students/100");
    const response = await GET(
      request,
      createMockContext({ id: undefined, studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toMatch(
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Activity ID and Student ID are required",
    );
  });

  it("returns 500 when student not found in enrollment", async () => {
    const students = [{ id: 101, first_name: "Jane", last_name: "Smith" }];
    mockApiGet.mockResolvedValueOnce({ data: students });

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Student with ID 100 is not enrolled",
    );
  });

  it("handles errors from getEnrolledStudents", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error"));

    const request = createMockRequest("/api/activities/1/students/100");
    const response = await GET(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("API error");
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid ID");
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Activity ID and Student ID are required",
    );
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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Student not found");
  });
});
