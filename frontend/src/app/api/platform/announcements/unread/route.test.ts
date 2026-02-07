import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { Session } from "next-auth";
import { mockSessionData } from "~/test/mocks/next-auth";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { GET } from "./route";

function mockValidSession(): void {
  mockAuth.mockResolvedValue(mockSessionData() as ExtendedSession);
}

const mockContext = { params: Promise.resolve({}) };

describe("GET /api/platform/announcements/unread", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches unread announcements successfully", async () => {
    mockValidSession();
    const announcements = [
      {
        id: 1,
        title: "Update Available",
        content: "New version released",
        type: "update",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        title: "Maintenance Notice",
        content: "Scheduled downtime",
        type: "maintenance",
        severity: "warning",
        published_at: "2024-01-02T00:00:00Z",
      },
    ];

    mockApiGet.mockResolvedValue({ data: announcements });

    const request = new NextRequest(
      "http://localhost:3000/api/platform/announcements/unread",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown; error?: string };
    expect(json.data).toEqual(announcements);
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/platform/announcements/unread",
      "test-token",
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/platform/announcements/unread",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string; data?: unknown };
    expect(json.error).toBe("Unauthorized");
  });

  it("returns empty array when no unread announcements", async () => {
    mockValidSession();
    mockApiGet.mockResolvedValue({ data: [] });

    const request = new NextRequest(
      "http://localhost:3000/api/platform/announcements/unread",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown; error?: string };
    expect(json.data).toEqual([]);
  });
});
