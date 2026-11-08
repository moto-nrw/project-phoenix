import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const { GET } = await import("./route");

function session(token: string): ExtendedSession {
  return { user: { token } } as ExtendedSession;
}

describe("GET /api/parent-announcements/[announcementId]/declaration-export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards the active analytics session with the export request", async () => {
    mockAuth.mockResolvedValue(session("tok"));
    global.fetch = vi.fn(
      async () =>
        new Response("csv", {
          headers: { "content-type": "text/csv" },
        }),
    ) as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/parent-announcements/42/declaration-export?format=csv",
        {
          headers: {
            "x-posthog-session-id": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
          },
        },
      ),
      { params: Promise.resolve({ announcementId: "42" }) },
    );

    expect(response.status).toBe(200);
    const init = (global.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0]![1] as RequestInit;
    expect(init.headers).toMatchObject({
      Authorization: "Bearer tok",
      "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
    });
  });
});
