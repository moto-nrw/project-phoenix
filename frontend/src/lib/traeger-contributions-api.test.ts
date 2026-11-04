import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cancelHolidayRegistration,
  fetchHolidayContribution,
} from "./traeger-contributions-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("traeger-contributions-api", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("cancels a registration through the route of the Elternabrechnung (#5662)", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, { data: { id: 7, status: "cancelled" } }),
    );

    await cancelHolidayRegistration("5", "7");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(String(url)).toBe(
      "/api/traeger/holiday-periods/5/registrations/7/cancel",
    );
    expect(init?.method).toBe("POST");
  });

  it("reads that an invoice may need a correction (#5662)", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        data: {
          children: [],
          total_cents: 2250,
          exemption_available: false,
          benefits_exemption: false,
          frozen: true,
          confirmed_at: "2026-09-08T10:00:00Z",
          invoiced: true,
          correction_needed: true,
        },
      }),
    );

    const contribution = await fetchHolidayContribution("5", "7");

    expect(contribution.invoiced).toBe(true);
    expect(contribution.correctionNeeded).toBe(true);
  });
});
