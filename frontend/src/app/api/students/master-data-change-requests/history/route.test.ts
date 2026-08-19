import { describe, expect, it, vi } from "vitest";

import { GET } from "./route";
import { apiGet } from "~/lib/api-helpers.server";

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
}));

vi.mock("~/lib/route-wrapper.server", () => ({
  createGetHandler:
    (handler: (request: Request, token: string) => Promise<unknown>) =>
    (request: Request) =>
      handler(request, "staff-token"),
}));

describe("staff master-data change request history route", () => {
  it("forwards only the allowlisted cursor and limit params", async () => {
    vi.mocked(apiGet).mockResolvedValue({
      data: { items: [{ id: "100" }], next_cursor: "abc" },
    });

    const out = await GET(
      new Request(
        "http://test.local/api/students/master-data-change-requests/history?cursor=abc&limit=5&evil=1",
      ) as never,
      {} as never,
    );

    expect(out).toEqual({ items: [{ id: "100" }], next_cursor: "abc" });
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/master-data-change-requests/history?cursor=abc&limit=5",
      "staff-token",
    );
  });

  it("forwards no query string when none is given", async () => {
    vi.mocked(apiGet).mockResolvedValue({ data: { items: [] } });

    await GET(new Request("http://test.local") as never, {} as never);

    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/master-data-change-requests/history",
      "staff-token",
    );
  });
});
