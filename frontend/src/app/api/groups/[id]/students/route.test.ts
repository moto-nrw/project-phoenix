import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return { ...actual, apiGet: mockApiGet };
});

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/groups/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when group ID is missing", async () => {
    const request = createMockRequest("/api/groups//students");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Group ID is required");
  });

  it("fetches students in group from backend", async () => {
    const mockStudents = [
      { id: 1, first_name: "Alice", second_name: "A" },
      { id: 2, first_name: "Bob", second_name: "B" },
    ];
    mockApiGet.mockResolvedValueOnce(mockStudents);

    const request = createMockRequest("/api/groups/123/students");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/groups/123/students",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<typeof mockStudents>(response);
    expect(json).toEqual(mockStudents);
  });

  it("returns array directly if response is already an array", async () => {
    const mockStudents = [{ id: 1, first_name: "Charlie", second_name: "C" }];
    mockApiGet.mockResolvedValueOnce(mockStudents);

    const request = createMockRequest("/api/groups/123/students");
    const response = await GET(request, createMockContext({ id: "123" }));

    const json = await parseJsonResponse<typeof mockStudents>(response);
    expect(json).toEqual(mockStudents);
  });
});
