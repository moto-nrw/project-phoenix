import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockParentAuth, mockUncachedParentAuth, mockAnalyticsHeaders } =
  vi.hoisted(() => ({
    mockParentAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockUncachedParentAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
    mockAnalyticsHeaders: vi.fn(),
  }));

vi.mock("~/server/auth/parent", () => ({
  parentAuth: mockParentAuth,
  uncachedParentAuth: mockUncachedParentAuth,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: mockAnalyticsHeaders,
}));

const { GET } = await import("./route");

function session(token: string): ExtendedSession {
  return { user: { token } } as ExtendedSession;
}

describe("GET /api/parent/me/news/[announcementId]/declaration/proof/pdf", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards the active analytics session with the proof download", async () => {
    mockParentAuth.mockResolvedValue(session("tok"));
    mockAnalyticsHeaders.mockResolvedValue({
      "X-POSTHOG-SESSION-ID": "4f79f286-6f14-49cb-bb84-16fd4f906eba",
    });
    global.fetch = vi.fn(
      async () =>
        new Response("pdf", {
          headers: { "content-type": "application/pdf" },
        }),
    ) as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/parent/me/news/42/declaration/proof/pdf?student_id=5",
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
