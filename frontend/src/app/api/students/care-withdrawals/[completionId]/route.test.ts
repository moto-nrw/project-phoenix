import { NextRequest } from "next/server";
import { describe, expect, it, vi } from "vitest";
import { createMockContext, defaultSession } from "~/test/route-test-helpers";

const { mockFetch } = vi.hoisted(() => ({ mockFetch: vi.fn<typeof fetch>() }));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: async () => defaultSession,
    withTenantAuth: (handler: unknown) => handler,
  }),
);
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));
vi.stubGlobal("fetch", mockFetch);

import { DELETE } from "./route";

describe("DELETE /api/students/care-withdrawals/[completionId]", () => {
  it("forwards the confirmation body to the encoded withdrawal and answers 204", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));
    const confirmation = { fingerprint: "f1", name: "Mia", reason: "Umzug" };

    const response = await DELETE(
      new NextRequest(
        "http://localhost:3000/api/students/care-withdrawals/7%2F1",
        {
          method: "DELETE",
          body: JSON.stringify(confirmation),
          headers: { "content-type": "application/json" },
        },
      ),
      createMockContext({ completionId: "7/1" }),
    );

    expect(response.status).toBe(204);
    const [url, init] = mockFetch.mock.calls[0]!;
    expect(url).toBe("http://backend:8080/api/students/care-withdrawals/7%2F1");
    expect(init?.method).toBe("DELETE");
    expect(JSON.parse(String(init?.body))).toEqual(confirmation);
  });
});
