import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST, PUT } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const {
  mockAuth,
  mockApiGet,
  mockApiPut,
  mockUpdateGroupEnrollments,
  mockEnrollStudent,
} = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockUpdateGroupEnrollments: vi.fn(),
  mockEnrollStudent: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
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
  updateGroupEnrollments: mockUpdateGroupEnrollments,
  enrollStudent: mockEnrollStudent,
}));

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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5/students");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("fetches enrolled students for activity", async () => {
    const mockEnrollments = [
      { id: 1, first_name: "Alice", last_name: "Smith", school_class: "3A" },
      { id: 2, first_name: "Bob", last_name: "Jones", school_class: "3B" },
    ];

    mockApiGet.mockResolvedValueOnce({ data: mockEnrollments });

    const request = createMockRequest("/api/activities/5/students");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/5/students",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string; school_class: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("1");
    expect(json.data[0]!.name).toBe("Alice Smith");
  });

  it("fetches available students when available=true query param", async () => {
    const mockEnrolled = [{ id: 1, first_name: "Alice", last_name: "Smith" }];
    const mockAllStudents = [
      { id: 1, first_name: "Alice", last_name: "Smith", school_class: "3A" },
      { id: 2, first_name: "Bob", last_name: "Jones", school_class: "3B" },
      { id: 3, first_name: "Carol", last_name: "White", school_class: "3C" },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: mockEnrolled })
      .mockResolvedValueOnce({ data: mockAllStudents });

    const request = createMockRequest(
      "/api/activities/5/students?available=true",
    );
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledTimes(2);
    expect(mockApiGet).toHaveBeenNthCalledWith(
      1,
      "/api/activities/5/students",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?",
      "test-token",
    );

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("2");
    expect(json.data[0]!.name).toBe("Bob Jones");
    expect(json.data[1]!.id).toBe("3");
    expect(json.data[1]!.name).toBe("Carol White");
  });

  it("filters available students by search param", async () => {
    const mockEnrolled = [{ id: 1, first_name: "Alice", last_name: "Smith" }];
    const mockAllStudents = [
      { id: 2, first_name: "Bob", last_name: "Jones", school_class: "3B" },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: mockEnrolled })
      .mockResolvedValueOnce({ data: mockAllStudents });

    const request = createMockRequest(
      "/api/activities/5/students?available=true&search=Bob",
    );
    await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?search=Bob",
      "test-token",
    );
  });

  it("returns empty array on error", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/activities/5/students");
    const response = await GET(request, createMockContext({ id: "5" }));

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<unknown>;
    }>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_id: "10" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("enrolls single student successfully", async () => {
    mockEnrollStudent.mockResolvedValueOnce({ success: true });

    const mockUpdatedEnrollments = [
      { id: 10, first_name: "Eve", last_name: "Brown", school_class: "3D" },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_id: "10" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockEnrollStudent).toHaveBeenCalledWith("5", { studentId: "10" });
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]!.id).toBe("10");
  });

  it("batch updates multiple students", async () => {
    mockUpdateGroupEnrollments.mockResolvedValueOnce(true);

    const mockUpdatedEnrollments = [
      { id: 10, first_name: "Eve", last_name: "Brown", school_class: "3D" },
      { id: 11, first_name: "Frank", last_name: "Green", school_class: "3E" },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_ids: ["10", "11"] },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockUpdateGroupEnrollments).toHaveBeenCalledWith("5", {
      student_ids: ["10", "11"],
    });
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
  });

  it("throws error when batch update fails", async () => {
    mockUpdateGroupEnrollments.mockResolvedValueOnce(false);

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_ids: ["10"] },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to update enrollments");
  });

  it("throws error when single enrollment fails", async () => {
    mockEnrollStudent.mockResolvedValueOnce({ success: false });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_id: "10" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to enroll student");
  });

  it("throws error when neither student_id nor student_ids provided", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: {},
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("must provide student_id or student_ids");
  });
});

describe("PUT /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: { student_ids: ["10", "11"] },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(401);
  });

  it("replaces enrolled students successfully", async () => {
    const mockUpdatedEnrollments = [
      { id: 20, first_name: "Grace", last_name: "Lee", school_class: "3F" },
      { id: 21, first_name: "Henry", last_name: "King", school_class: "3G" },
    ];

    mockApiPut.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: { student_ids: ["20", "21"] },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/5/students",
      "test-token",
      { student_ids: [20, 21] },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("20");
  });

  it("throws error when student_ids not provided", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: {},
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("student_ids array is required");
  });

  it("throws error when student_ids is not an array", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: { student_ids: "not-an-array" },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
  });
});
