import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/api-helpers.server")>();
  return {
    ...actual,
    apiGet: mockApiGet,
  };
});

const session: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

describe("GET /api/students/[id]/delete-impact", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(session);
  });

  it("forwards the request and returns the impact without a double envelope", async () => {
    const impact = {
      confirmation_name: "Mia Muster",
      fingerprint: "abc",
      total: 2,
      counts: { timetable_assignments: 2 },
      preserved: { shared_instances: true },
    };
    mockApiGet.mockResolvedValueOnce({ data: impact });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/students/42/delete-impact"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/42/delete-impact",
      "test-token",
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({ data: impact });
  });

  it("requires authentication", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      new NextRequest("http://localhost:3000/api/students/42/delete-impact"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(401);
    expect(mockApiGet).not.toHaveBeenCalled();
  });
});
