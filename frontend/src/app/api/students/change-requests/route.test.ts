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

describe("aggregated change request list route", () => {
  it("forwards only the allowlisted params", async () => {
    vi.mocked(apiGet).mockResolvedValue({
      data: { items: [{ request_type: "excused", data: { id: "1" } }] },
    });

    const out = await GET(
      new Request(
        "http://test.local/api/students/change-requests?view=history&search=Emma&types=excused,offering&status=approved&from=2026-08-01&to=2026-08-19&cursor=abc&limit=5&evil=1",
      ) as never,
      {} as never,
    );

    expect(out).toEqual({
      items: [{ request_type: "excused", data: { id: "1" } }],
    });
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests?view=history&search=Emma&types=excused%2Coffering&status=approved&from=2026-08-01&to=2026-08-19&cursor=abc&limit=5",
      "staff-token",
    );
  });

  it("forwards the child filter of the Änderungsprotokoll", async () => {
    vi.mocked(apiGet).mockResolvedValue({ data: { items: [] } });

    await GET(
      new Request(
        "http://test.local/api/students/change-requests?view=history&student_id=42",
      ) as never,
      {} as never,
    );

    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests?view=history&student_id=42",
      "staff-token",
    );
  });

  it("forwards no query string when none is given", async () => {
    vi.mocked(apiGet).mockResolvedValue({ data: { items: [] } });

    await GET(new Request("http://test.local") as never, {} as never);

    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests",
      "staff-token",
    );
  });
});
