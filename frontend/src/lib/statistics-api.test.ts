import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import { fetchStatisticsReport } from "./statistics-api";

describe("fetchStatisticsReport errors", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("carries the backend code, details, field errors and request ID", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        Response.json(
          {
            error: "forbidden",
            code: "statistics.report_forbidden",
            details: { report: "attendance" },
            errors: [{ field: "group_id", reason: "not_permitted" }],
            instance: "request-statistics-403",
          },
          { status: 403 },
        ),
      ),
    );

    await expect(
      fetchStatisticsReport("2026-09-01", "2026-09-09"),
    ).rejects.toMatchObject({
      name: "ApiError",
      code: "statistics.report_forbidden",
      status: 403,
      details: { report: "attendance" },
      errors: [{ field: "group_id", reason: "not_permitted" }],
      requestId: "request-statistics-403",
    });
  });

  it("uses the status class code when no backend code is present", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          Response.json({ error: "bad range" }, { status: 400 }),
        ),
    );

    const failure = fetchStatisticsReport("2026-09-01", "2026-09-09");
    await expect(failure).rejects.toBeInstanceOf(ApiError);
    await expect(failure).rejects.toMatchObject({
      code: "general.input",
      status: 400,
    });
  });

  it("turns a request that never reached the API into general.unavailable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
    );

    await expect(
      fetchStatisticsReport("2026-09-01", "2026-09-09"),
    ).rejects.toMatchObject({ code: "general.unavailable" });
  });
});
