import { describe, expect, it, vi } from "vitest";

import { GET } from "./route";
import { parentApiGet } from "~/lib/parent/route-wrapper.server";

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
  parentApiGet: vi.fn(),
}));

describe("parent child master-data route", () => {
  it("forwards to the encoded backend master-data endpoint", async () => {
    vi.mocked(parentApiGet).mockResolvedValue({ student_id: "42" });

    const out = await GET(
      new Request("http://test.local") as never,
      {
        params: { studentId: "42/unsafe" },
      } as never,
    );

    expect(out).toEqual({ student_id: "42" });
    expect(parentApiGet).toHaveBeenCalledWith(
      "/parent/me/children/42%2Funsafe/master-data",
      "parent-token",
    );
  });
});
