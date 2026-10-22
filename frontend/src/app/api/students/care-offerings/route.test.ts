import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

function createMockRequest(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"));
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/students/care-offerings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("proxies the active care offerings with string ids", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: [{ id: 12, name: "OGS Ganztag" }],
    });

    const response = await GET(
      createMockRequest("/api/students/care-offerings"),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/care-offerings",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toEqual([{ id: "12", name: "OGS Ganztag" }]);
  });

  it("answers an empty list for an unexpected backend shape", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const response = await GET(
      createMockRequest("/api/students/care-offerings"),
      createMockContext(),
    );

    const json = await parseJsonResponse<{ data: unknown[] }>(response);
    expect(json.data).toEqual([]);
  });
});
