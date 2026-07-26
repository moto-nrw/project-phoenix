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

describe("staff master-data change request route", () => {
  it("forwards the queue request to the backend", async () => {
    vi.mocked(apiGet).mockResolvedValue({ data: [{ id: "100" }] });

    const out = await GET(
      new Request("http://test.local") as never,
      {} as never,
    );

    expect(out).toEqual([{ id: "100" }]);
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/master-data-change-requests",
      "staff-token",
    );
  });
});
