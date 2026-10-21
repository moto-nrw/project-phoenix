import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST, PUT } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiGet, mockApiPost, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
}));

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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
      data: Array<{ id: string; student_id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("1");
    expect(json.data[0]!.student_id).toBe("1");
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

  it("does not turn a failed backend request into an empty enrollment list", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/activities/5/students");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Backend error");
  });
});

describe("POST /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("enrolls single student successfully", async () => {
    const mockUpdatedEnrollments = [
      { id: 10, first_name: "Eve", last_name: "Brown", school_class: "3D" },
    ];
    mockApiPost.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_id: "10" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/5/enroll/10",
      "test-token",
      {},
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; student_id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]!.id).toBe("10");
    expect(json.data[0]!.student_id).toBe("10");
    expect(json.data[0]!.name).toBe("Eve Brown");
  });

  it("batch updates multiple students", async () => {
    const mockUpdatedEnrollments = [
      { id: 10, first_name: "Eve", last_name: "Brown", school_class: "3D" },
      { id: 11, first_name: "Frank", last_name: "Green", school_class: "3E" },
    ];
    mockApiPut.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_ids: ["10", "11"] },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/5/students",
      "test-token",
      { student_ids: [10, 11] },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("10");
    expect(json.data[1]!.id).toBe("11");
  });

  it("throws error when neither student_id nor student_ids provided", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: {},
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "must provide student_id or student_ids",
    );
  });
});

describe("PUT /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
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
      data: Array<{ id: string; student_id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("20");
    expect(json.data[0]!.student_id).toBe("20");
    expect(json.data[0]!.name).toBe("Grace Lee");
  });

  it("throws error when student_ids not provided", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: {},
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("student_ids array is required");
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
