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

describe("GET /api/school/class-day/absence-list/export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("downloads the absence list of a class with the school session", async () => {
    mockAuth.mockResolvedValue(session("school-tok"));
    global.fetch = vi.fn(
      async () =>
        new Response("xlsx", {
          headers: {
            "content-type":
              "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            "content-disposition": 'attachment; filename="fehlzeiten.xlsx"',
          },
        }),
    ) as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/school/class-day/absence-list/export?class=3a&from=2026-08-01&to=2027-01-31&format=xlsx&extra=1",
      ),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-disposition")).toBe(
      'attachment; filename="fehlzeiten.xlsx"',
    );
    const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0]! as [string, RequestInit];
    expect(url).toBe(
      "http://backend:8080/school/class-day/absence-list/export?class=3a&from=2026-08-01&to=2027-01-31&format=xlsx",
    );
    expect(init.headers).toMatchObject({ Authorization: "Bearer school-tok" });
  });

  it("refuses an unknown format before calling the backend", async () => {
    mockAuth.mockResolvedValue(session("school-tok"));
    global.fetch = vi.fn() as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/school/class-day/absence-list/export?class=3a&format=docx",
      ),
    );

    expect(response.status).toBe(400);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("forwards a refusal of the backend", async () => {
    mockAuth.mockResolvedValue(session("school-tok"));
    global.fetch = vi.fn(
      async () =>
        new Response(
          JSON.stringify({ status: 403, code: "general.forbidden" }),
          {
            status: 403,
            headers: { "content-type": "application/problem+json" },
          },
        ),
    ) as unknown as typeof fetch;

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/school/class-day/absence-list/export?class=9z&format=pdf",
      ),
    );

    expect(response.status).toBe(403);
  });
});
