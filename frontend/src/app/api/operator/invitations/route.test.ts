import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockOperatorApiGet, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockOperatorApiGet: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockUncachedAuth,
}));

vi.mock("~/lib/operator/route-wrapper", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/operator/route-wrapper")>();
  return {
    ...actual,
    operatorApiGet: mockOperatorApiGet,
  };
});

import { GET } from "./route";

const mockSession = { user: { token: "access-token-123" } };

function createMockContext() {
  return { params: Promise.resolve({}) };
}

describe("GET /api/operator/invitations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(mockSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/invitations",
    );
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns invitation data on success", async () => {
    const mockData = {
      invitations: [{ id: 1, email: "test@example.com" }],
      operators: [{ id: 2, email: "admin@example.com" }],
    };
    mockOperatorApiGet.mockResolvedValue(mockData);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/invitations",
    );
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: unknown;
      status: string;
    };
    expect(json.data).toEqual(mockData);
    expect(mockOperatorApiGet).toHaveBeenCalledWith(
      "/operator/invitations",
      "access-token-123",
    );
  });
});
