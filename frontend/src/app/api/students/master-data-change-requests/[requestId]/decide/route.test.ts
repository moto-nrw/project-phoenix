import { describe, expect, it, vi } from "vitest";

import { POST } from "./route";
import { apiPost } from "~/lib/api-helpers.server";

vi.mock("~/lib/api-helpers.server", () => ({
  apiPost: vi.fn(),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createPostHandler:
    (
      handler: (
        request: Request,
        body: { approve: boolean; reason?: string },
        token: string,
        params: Record<string, unknown>,
      ) => Promise<unknown>,
    ) =>
    (
      request: Request,
      context: {
        params: Record<string, unknown>;
        body: { approve: boolean; reason?: string };
      },
    ) =>
      handler(request, context.body, "staff-token", context.params),
}));

describe("staff master-data change request decision route", () => {
  it("forwards decisions to the encoded backend request endpoint", async () => {
    const body = { approve: true, reason: "passt" };
    vi.mocked(apiPost).mockResolvedValue({ data: { id: "id/with space" } });

    const out = await POST(
      new Request("http://test.local") as never,
      {
        params: { requestId: "id/with space" },
        body,
      } as never,
    );

    expect(out).toEqual({ id: "id/with space" });
    expect(apiPost).toHaveBeenCalledWith(
      "/api/students/master-data-change-requests/id%2Fwith%20space/decide",
      "staff-token",
      body,
    );
  });
});
