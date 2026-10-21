import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { type ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
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
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

const { GET } = await import("./route");

const request = () =>
  new NextRequest(new URL("/api/staff/former", "http://localhost:3000"));
const context = { params: Promise.resolve({}) };

describe("GET /api/staff/former", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token", name: "Test User" },
      expires: "2099-01-01",
    });
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(request(), context);

    expect(response.status).toBe(401);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("reads the backend's former employments and maps them", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: [
        {
          id: "42",
          first_name: "Grace",
          last_name: "Hopper",
          started_on: "2024-09-02",
          ended_on: "2026-07-31",
        },
      ],
    });

    const response = await GET(request(), context);

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff?employment=former",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: unknown };
    expect(json.data).toEqual([
      {
        id: "42",
        firstName: "Grace",
        lastName: "Hopper",
        startedOn: "2024-09-02",
        endedOn: "2026-07-31",
      },
    ]);
  });

  it("keeps a bigint identifier exact", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: [
        {
          id: "9223372036854775807",
          first_name: "Ada",
          last_name: "Lovelace",
          started_on: "2024-09-02",
          ended_on: "2026-07-31",
        },
      ],
    });

    const response = await GET(request(), context);

    const json = (await response.json()) as {
      data: Array<{ id: string }>;
    };
    expect(json.data[0]?.id).toBe("9223372036854775807");
  });

  it("answers an empty list when the backend has none", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const response = await GET(request(), context);

    const json = (await response.json()) as { data: unknown };
    expect(json.data).toEqual([]);
  });
});
