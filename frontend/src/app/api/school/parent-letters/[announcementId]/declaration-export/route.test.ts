import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth/school", () => ({
  schoolAuth: mockAuth,
  uncachedSchoolAuth: mockUncachedAuth,
  withSchoolAuth: <T>(handler: T) => handler,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: vi.fn(async () => ({})),
}));

const { GET } = await import("./route");

function session(token: string): ExtendedSession {
  return { user: { token } } as ExtendedSession;
}

describe("GET /api/school/parent-letters/[announcementId]/declaration-export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("downloads the proof of an own class letter with the school session", async () => {
    mockAuth.mockResolvedValue(session("school-tok"));
    global.fetch = vi.fn(
      async () =>
        new Response("pdf", {
          headers: { "content-type": "application/pdf" },
        }),
    ) as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/school/parent-letters/42/declaration-export?format=pdf",
      ),
      { params: Promise.resolve({ announcementId: "42" }) },
    );

    expect(response.status).toBe(200);
    const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0]! as [string, RequestInit];
    expect(url).toBe(
      "http://backend:8080/school/parent-letters/42/declaration-export?format=pdf",
    );
    expect(init.headers).toMatchObject({ Authorization: "Bearer school-tok" });
  });

  it("refuses an unknown format before calling the backend", async () => {
    mockAuth.mockResolvedValue(session("school-tok"));
    global.fetch = vi.fn() as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/school/parent-letters/42/declaration-export?format=docx",
      ),
      { params: Promise.resolve({ announcementId: "42" }) },
    );

    expect(response.status).toBe(400);
    expect(global.fetch).not.toHaveBeenCalled();
  });
});
