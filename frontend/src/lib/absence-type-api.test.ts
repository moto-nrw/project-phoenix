import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getSession } from "next-auth/react";

import { absenceTypeService } from "./absence-type-api";

vi.mock("next-auth/react", () => ({ getSession: vi.fn() }));

describe("absenceTypeService allowances", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.mocked(getSession).mockResolvedValue({
      user: { token: "token" },
      expires: "2027-01-01",
    } as never);
  });

  afterEach(() => {
    global.fetch = originalFetch;
    vi.clearAllMocks();
  });

  it("maps allowance configuration from the type response", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: [
            {
              id: "12",
              name: "Regenerationstag",
              base_type: "other",
              is_active: true,
              allowance_enabled: true,
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(absenceTypeService.getAbsenceTypes()).resolves.toEqual([
      expect.objectContaining({
        id: "12",
        allowanceEnabled: true,
      }),
    ]);
  });

  it("creates an absence type with its allowance configuration", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {
            id: "12",
            name: "Regenerationstag",
            base_type: "other",
            is_active: true,
            allowance_enabled: true,
          },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(
      absenceTypeService.createAbsenceType("Regenerationstag", {
        allowanceEnabled: true,
      }),
    ).resolves.toMatchObject({
      allowanceEnabled: true,
    });
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/staff/absence-types",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          name: "Regenerationstag",
          allowance_enabled: true,
        }),
      }),
    );
  });

  it("sends and reads the carryover rule (#3257)", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {
            id: "12",
            name: "Krank-Urlaubstag",
            base_type: "other",
            is_active: true,
            allowance_enabled: true,
            carryover_until: "03-31",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(
      absenceTypeService.updateAbsenceType("12", { carryoverUntil: null }),
    ).resolves.toMatchObject({ carryoverUntil: "03-31" });
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/staff/absence-types/12",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ carryover_until: "" }),
      }),
    );
  });

  it("reads the yearly split of a planned booking", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {
            years: [
              {
                staff_id: "42",
                absence_type_id: "12",
                year: 2026,
                entitled_days: 2,
                taken_days: 2,
                reserved_days: 0,
                remaining_days: 0,
                expires_on: "2027-03-31",
                expired_days: 0,
                booking_days: 1,
                carried_in: null,
              },
            ],
            blocked: true,
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(
      absenceTypeService.previewAllowance("12", "42", {
        dateStart: "2027-01-11",
        dateEnd: "2027-01-12",
        halfDay: false,
      }),
    ).resolves.toEqual({
      years: [
        expect.objectContaining({
          year: 2026,
          bookingDays: 1,
          expiresOn: "2027-03-31",
          carriedIn: null,
        }),
      ],
      blocked: true,
    });
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/staff/absence-types/12/allowances/42/preview?date_start=2027-01-11&date_end=2027-01-12&half_day=false",
      expect.anything(),
    );
  });

  it("saves a yearly claim with its reason", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: {
            staff_id: "42",
            absence_type_id: "12",
            year: 2026,
            entitled_days: 2.5,
            taken_days: 0.5,
            reserved_days: 0,
            remaining_days: 2,
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    const result = await absenceTypeService.setAllowance("12", "42", {
      year: 2026,
      entitledDays: 2.5,
      reason: "Tariflicher Anspruch",
    });

    expect(result).toEqual(
      expect.objectContaining({
        staffId: "42",
        absenceTypeId: "12",
        remainingDays: 2,
      }),
    );
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/staff/absence-types/12/allowances/42",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({
          year: 2026,
          entitled_days: 2.5,
          reason: "Tariflicher Anspruch",
        }),
      }),
    );
  });
});
