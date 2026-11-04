import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockFetch, mockAuth } = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockAuth: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth, uncachedAuth: mockAuth }));
vi.mock("~/server/auth/parent", () => ({
  parentAuth: mockAuth,
  uncachedParentAuth: mockAuth,
}));
vi.mock("~/server/auth/traeger", () => ({
  traegerAuth: mockAuth,
  uncachedTraegerAuth: mockAuth,
}));
vi.mock("~/server/auth/school", () => ({
  schoolAuth: mockAuth,
  uncachedSchoolAuth: mockAuth,
  withSchoolAuth: <T>(handler: T) => handler,
}));
vi.mock("~/server/auth/caterer", () => ({
  catererAuth: mockAuth,
  uncachedCatererAuth: mockAuth,
}));
vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
  withOperatorAuth: <T>(handler: T) => handler,
}));
vi.mock("~/server/auth/school-route", () => ({
  withSchoolAuth: <T>(handler: T) => handler,
}));
vi.mock("~/server/auth/caterer-route", () => ({
  withCatererAuth: <T>(handler: T) => handler,
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
vi.mock("~/lib/analytics-session-header.server", () => ({
  incomingAnalyticsSessionHeaders: async () => ({}),
}));
vi.mock("~/lib/client-headers.server", () => ({
  incomingClientForwardHeaders: async () => ({}),
  clientIdentityHeaders: () => ({}),
}));

type Route = (
  request: NextRequest,
  context: { params: Promise<typeof downloadParams> },
) => Promise<Response>;

const downloadParams = {
  id: "1",
  documentId: "2",
  organizationId: "3",
  announcementId: "4",
  token: "status-token",
};

const routes: Array<[string, string, () => Promise<Route>]> = [
  [
    "student export",
    "POST",
    async () => (await import("./students/export/route")).POST,
  ],
  [
    "planning export",
    "POST",
    async () => (await import("./rooms/export/route")).POST,
  ],
  [
    "staff streaming export",
    "GET",
    async () => (await import("./staff/time-tracking/export/route")).GET,
  ],
  [
    "student document",
    "GET",
    async () =>
      (await import("./students/[id]/documents/[documentId]/download/route"))
        .GET,
  ],
  [
    "parent document",
    "GET",
    async () =>
      (
        await import("./parent/me/documents/[organizationId]/[documentId]/pdf/route")
      ).GET,
  ],
  [
    "school declaration",
    "GET",
    async () =>
      (
        await import("./school/parent-letters/[announcementId]/declaration-export/route")
      ).GET,
  ],
  [
    "caterer export",
    "GET",
    async () => (await import("./caterer/orders/export/route")).GET,
  ],
  [
    "operator export",
    "GET",
    async () =>
      (await import("./operator/billing/key-date-counts/export/route")).GET,
  ],
  [
    "Träger export",
    "GET",
    async () => (await import("./traeger/access-log/export/route")).GET,
  ],
  [
    "token-bound proof",
    "GET",
    async () =>
      (await import("./enrollment/requests/[token]/signature-proof/route")).GET,
  ],
];

function call(route: Route, method: string) {
  return route(
    new NextRequest("http://localhost:3000/api/export?format=pdf", {
      method,
      ...(method === "POST" ? { body: "{}" } : {}),
    }),
    {
      params: Promise.resolve(downloadParams),
    },
  );
}

beforeEach(() => {
  mockFetch.mockReset();
  mockAuth.mockReset().mockResolvedValue({ user: { token: "access-token" } });
  vi.stubGlobal("fetch", mockFetch);
});

afterEach(() => vi.unstubAllGlobals());

describe("private file responses (#5119)", () => {
  it.each(routes)(
    "%s: protects real file bytes and download headers",
    async (_name, method, load) => {
      const bytes = new Uint8Array([0, 255, 37, 80, 68, 70]);
      mockFetch.mockResolvedValue(
        new Response(bytes, {
          headers: {
            "Content-Type": "application/pdf",
            "Content-Disposition": 'attachment; filename="report.pdf"',
            "Content-Length": String(bytes.length),
            // Even a mistaken upstream cache policy cannot expose private files.
            "Cache-Control": "public, max-age=3600",
          },
        }),
      );

      const response = await call(await load(), method);

      expect(response.status).toBe(200);
      expect(mockFetch).toHaveBeenCalledTimes(1);
      expect(response.headers.get("Cache-Control")).toBe("private, no-store");
      expect(response.headers.get("Content-Type")).toBe("application/pdf");
      expect(response.headers.get("Content-Disposition")).toBe(
        'attachment; filename="report.pdf"',
      );
      expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes);
    },
  );

  it.each([
    "text/csv; charset=utf-8",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    "application/octet-stream",
  ])("protects %s as well as PDF", async (contentType) => {
    const bytes = new Uint8Array([0, 255, 1, 2]);
    mockFetch.mockResolvedValue(
      new Response(bytes, { headers: { "Content-Type": contentType } }),
    );
    const { POST } = await import("./students/export/route");
    const response = await call(POST, "POST");
    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    expect(response.headers.get("Content-Type")).toBe(contentType);
    expect(response.headers.get("Content-Length")).toBe(String(bytes.length));
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes);
  });

  it.each(routes)(
    "%s: protects backend errors",
    async (_name, method, load) => {
      for (const status of [401, 403, 503]) {
        mockFetch.mockImplementation(() =>
          Promise.resolve(
            new Response("denied", {
              status,
              headers: { "Content-Type": "text/plain" },
            }),
          ),
        );
        const response = await call(await load(), method);
        expect(response.status).toBe(status);
        expect(response.headers.get("Cache-Control")).toBe("private, no-store");
      }
    },
  );

  it("returns a streaming download before the upstream finishes", async () => {
    let controller!: ReadableStreamDefaultController<Uint8Array>;
    const stream = new ReadableStream<Uint8Array>({
      start(value) {
        controller = value;
      },
    });
    mockFetch.mockResolvedValue(
      new Response(stream, {
        headers: { "Content-Type": "application/pdf", "Content-Length": "4" },
      }),
    );
    const { GET } = await import("./staff/time-tracking/export/route");
    const response = await call(GET, "GET");
    expect(response.status).toBe(200);
    expect(response.body).toBe(stream);
    expect(response.headers.get("Content-Length")).toBe("4");
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    const reader = response.body!.getReader();
    controller.enqueue(new Uint8Array([1, 2]));
    expect(await reader.read()).toEqual({
      done: false,
      value: new Uint8Array([1, 2]),
    });
    controller.enqueue(new Uint8Array([3, 4]));
    controller.close();
    expect(await reader.read()).toEqual({
      done: false,
      value: new Uint8Array([3, 4]),
    });
    expect((await reader.read()).done).toBe(true);
  });
});
