import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/activities/[id]/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockReset().mockResolvedValue(defaultSession);
    mockApiGet.mockReset();
    mockApiPost.mockReset();
  });

  it("fetches supervisors for activity", async () => {
    const mockSupervisors = [
      {
        id: 10,
        staff_id: 110,
        first_name: "Mr.",
        last_name: "Smith",
        is_primary: true,
      },
      {
        id: 11,
        staff_id: 111,
        first_name: "Ms.",
        last_name: "Johnson",
        is_primary: false,
      },
    ];

    mockApiGet.mockResolvedValueOnce({ data: mockSupervisors });

    const request = createMockRequest("/api/activities/5/supervisors");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/5/supervisors",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{
        id: string;
        staff_id: string;
        name: string;
        is_primary: boolean;
      }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]?.id).toBe("10");
    expect(json.data[0]?.staff_id).toBe("110");
    expect(json.data[0]?.name).toBe("Mr. Smith");
    expect(json.data[0]?.is_primary).toBe(true);
  });

  it("throws error when activity ID is missing", async () => {
    const request = createMockRequest("/api/activities//supervisors");
    const response = await GET(request, createMockContext({ id: "" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Activity ID is required");
  });
});

describe("POST /api/activities/[id]/supervisors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockReset().mockResolvedValue(defaultSession);
    mockApiGet.mockReset();
    mockApiPost.mockReset();
  });

  it("assigns supervisor successfully", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "15", is_primary: true },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/5/supervisors",
      "test-token",
      {
        staff_id: 15,
        is_primary: true,
      },
    );
    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();

    const json = await parseJsonResponse<{
      success: boolean;
    }>(response);
    expect(json.success).toBe(true);
  });

  it("assigns supervisor without is_primary flag", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "20" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/5/supervisors",
      "test-token",
      {
        staff_id: 20,
        is_primary: undefined,
      },
    );
    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("does not refetch after a successful assignment", async () => {
    mockApiPost.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: { staff_id: "20" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(200);
    expect(mockApiGet).not.toHaveBeenCalled();

    const json = await parseJsonResponse<{
      success: boolean;
    }>(response);
    expect(json.success).toBe(true);
  });

  it("throws error when activity ID is missing", async () => {
    const request = createMockRequest("/api/activities//supervisors", {
      method: "POST",
      body: { staff_id: "15" },
    });
    const response = await POST(request, createMockContext({ id: "" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Activity ID is required");
  });

  it("throws error when staff_id is missing", async () => {
    const request = createMockRequest("/api/activities/5/supervisors", {
      method: "POST",
      body: {},
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Staff ID is required");
  });
});
