import { describe, expect, it, vi } from "vitest";

import { GET, POST } from "./route";
import { parentApiGet, parentApiPost } from "~/lib/parent/route-wrapper.server";

vi.mock("~/lib/parent/route-wrapper.server", () => ({
  createParentGetHandler:
    (
      handler: (
        request: Request,
        token: string,
        params: Record<string, unknown>,
      ) => Promise<unknown>,
    ) =>
    (request: Request, context: { params: Record<string, unknown> }) =>
      handler(request, "parent-token", context.params),
  createParentPostHandler:
    (
      handler: (
        request: Request,
        body: { changes: unknown[] },
        token: string,
        params: Record<string, unknown>,
      ) => Promise<unknown>,
    ) =>
    (
      request: Request,
      context: {
        params: Record<string, unknown>;
        body: { changes: unknown[] };
      },
    ) =>
      handler(request, context.body, "parent-token", context.params),
  parentApiGet: vi.fn(),
  parentApiPost: vi.fn(),
}));

describe("parent child master-data requests route", () => {
  it("forwards GET to the encoded backend request list endpoint", async () => {
    vi.mocked(parentApiGet).mockResolvedValue([{ id: "100" }]);

    const out = await GET(
      new Request("http://test.local") as never,
      {
        params: { studentId: "42/unsafe" },
      } as never,
    );

    expect(out).toEqual([{ id: "100" }]);
    expect(parentApiGet).toHaveBeenCalledWith(
      "/parent/me/children/42%2Funsafe/master-data/requests",
      "parent-token",
    );
  });

  it("forwards POST bodies to the encoded backend request endpoint", async () => {
    const body = {
      changes: [{ target: "person", field_key: "first_name", value: "Lea" }],
    };
    vi.mocked(parentApiPost).mockResolvedValue([{ id: "101" }]);

    const out = await POST(
      new Request("http://test.local") as never,
      {
        params: { studentId: "42/unsafe" },
        body,
      } as never,
    );

    expect(out).toEqual([{ id: "101" }]);
    expect(parentApiPost).toHaveBeenCalledWith(
      "/parent/me/children/42%2Funsafe/master-data/requests",
      "parent-token",
      body,
    );
  });
});
