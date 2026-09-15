import { describe, expect, it, vi } from "vitest";

import { apiGet } from "~/lib/api-helpers.server";
import { GET } from "./route";

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createGetHandler:
    (handler: (request: Request, token: string) => Promise<unknown>) =>
    (request: Request) =>
      handler(request, "staff-token"),
}));

describe("change request access route", () => {
  it("reicht die effektive Backend-Capability unverändert durch", async () => {
    vi.mocked(apiGet).mockResolvedValue({
      data: { review_access: "group_leader" },
    });

    const result = await GET(new Request("http://test.local") as never, {
      params: Promise.resolve({}),
    });

    expect(result).toEqual({ review_access: "group_leader" });
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests/access",
      "staff-token",
    );
  });
});
