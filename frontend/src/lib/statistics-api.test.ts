import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchStatisticsReport, StatisticsError } from "./statistics-api";

describe("fetchStatisticsReport errors", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("separates backend code from the existing forbidden display code", async () => {
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
      name: "StatisticsError",
      legacyCode: "forbidden",
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
    await expect(failure).rejects.toBeInstanceOf(StatisticsError);
    await expect(failure).rejects.toMatchObject({
      legacyCode: "invalid_request",
      code: "general.input",
      status: 400,
    });
  });
});
