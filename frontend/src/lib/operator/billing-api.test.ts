import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  billingMonth,
  mapBillingKeyDateCount,
  operatorBillingService,
} from "./billing-api";

const { mockDownloadBlob } = vi.hoisted(() => ({ mockDownloadBlob: vi.fn() }));

vi.mock("~/lib/file-download", async () => {
  const actual = await vi.importActual<typeof import("~/lib/file-download")>(
    "~/lib/file-download",
  );
  return { ...actual, downloadBlob: mockDownloadBlob };
});

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify({ success: true, data }), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("operator billing api", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockReset();
    mockDownloadBlob.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("maps a captured row and turns the school ID into a string", () => {
    expect(
      mapBillingKeyDateCount({
        school_id: 42,
        school_name: "OGS Am Berg",
        organization_name: "Träger Nord",
        period: "2026-08-01",
        key_date: "2026-08-15",
        active_students: 120,
        active_terminals: 3,
        recorded_at: "2026-08-15T04:00:00Z",
      }),
    ).toEqual({
      schoolId: "42",
      schoolName: "OGS Am Berg",
      organizationName: "Träger Nord",
      period: "2026-08-01",
      keyDate: "2026-08-15",
      activeStudents: 120,
      activeTerminals: 3,
      recordedAt: "2026-08-15T04:00:00Z",
    });
    expect(billingMonth("2026-08-01")).toBe("2026-08");
  });

  it("sends the new key day as key_day", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({
        key_day: 10,
        next_key_date: "2026-10-10",
        updated_at: "2026-09-22T10:00:00Z",
      }),
    );

    const result = await operatorBillingService.updateKeyDay(10);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/operator/billing/key-day",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ key_day: 10 }),
      }),
    );
    expect(result).toEqual({
      keyDay: 10,
      nextKeyDate: "2026-10-10",
      updatedAt: "2026-09-22T10:00:00Z",
    });
  });

  it("treats a missing list as no captured months", async () => {
    fetchMock.mockResolvedValue(jsonResponse(null));
    await expect(operatorBillingService.listKeyDateCounts()).resolves.toEqual(
      [],
    );
  });

  it("downloads one month under the backend's file name", async () => {
    fetchMock.mockResolvedValue(
      new Response("csv", {
        status: 200,
        headers: {
          "Content-Disposition":
            'attachment; filename="abrechnung-stichtag-2026-08.csv"',
        },
      }),
    );

    await operatorBillingService.downloadKeyDateCounts("2026-08");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/operator/billing/key-date-counts/export?month=2026-08",
      { credentials: "include" },
    );
    expect(mockDownloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      "abrechnung-stichtag-2026-08.csv",
    );
  });

  it("fails instead of saving an error page as CSV", async () => {
    fetchMock.mockResolvedValue(new Response("nope", { status: 500 }));

    await expect(
      operatorBillingService.downloadKeyDateCounts(),
    ).rejects.toThrow();
    expect(mockDownloadBlob).not.toHaveBeenCalled();
  });
});
