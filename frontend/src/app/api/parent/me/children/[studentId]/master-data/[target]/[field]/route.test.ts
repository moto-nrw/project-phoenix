import { describe, expect, it, vi } from "vitest";

import { PATCH } from "./route";
import { parent } from "~/lib/parent/route-wrapper.server";

vi.mock("~/lib/parent/route-wrapper.server", () => ({
  createParentPatchHandler:
    (
      handler: (
        request: Request,
        body: { value: unknown },
        token: string,
        params: Record<string, unknown>,
      ) => Promise<unknown>,
    ) =>
    (
      request: Request,
      context: { params: Record<string, unknown>; body: { value: unknown } },
    ) =>
      handler(request, context.body, "parent-token", context.params),
  parent: { api: vi.fn() },
}));

describe("parent child master-data field route", () => {
  it("forwards PATCH bodies to the encoded backend field endpoint", async () => {
    vi.mocked(parent.api).mockResolvedValue({ data: { ok: true } });

    const out = await PATCH(
      new Request("http://test.local") as never,
      {
        params: {
          studentId: "42/unsafe",
          target: "guardian/profile",
          field: "email address",
        },
        body: { value: "new@example.test" },
      } as never,
    );

    expect(out).toEqual({ ok: true });
    expect(parent.api).toHaveBeenCalledWith(
      "PATCH",
      "/parent/me/children/42%2Funsafe/master-data/guardian%2Fprofile/email%20address",
      "parent-token",
      { value: "new@example.test" },
    );
  });
});
