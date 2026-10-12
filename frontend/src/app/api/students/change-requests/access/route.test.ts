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

function requestFor(url: string) {
  const request = new Request(url);
  return Object.assign(request, { nextUrl: new URL(url) });
}

describe("change request access route", () => {
  it("reicht die effektive Backend-Capability unverändert durch", async () => {
    vi.mocked(apiGet).mockResolvedValue({
      data: { review_access: "group_leader" },
    });

    const result = await GET(requestFor("http://test.local") as never, {
      params: Promise.resolve({}),
    });

    expect(result).toEqual({ review_access: "group_leader" });
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests/access",
      "staff-token",
    );
  });

  it("reicht student_id an das Backend weiter (#3886)", async () => {
    vi.mocked(apiGet).mockResolvedValue({
      data: {
        review_access: "group_leader",
        student: { requests: false, absences: true },
      },
    });

    const result = await GET(
      requestFor(
        "http://test.local/api/students/change-requests/access?student_id=42",
      ) as never,
      { params: Promise.resolve({}) },
    );

    expect(result).toEqual({
      review_access: "group_leader",
      student: { requests: false, absences: true },
    });
    expect(apiGet).toHaveBeenCalledWith(
      "/api/students/change-requests/access?student_id=42",
      "staff-token",
    );
  });
});
