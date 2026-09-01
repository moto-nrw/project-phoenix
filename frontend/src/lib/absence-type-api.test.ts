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
              overrun_policy: "block",
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
        overrunPolicy: "block",
      }),
    ]);
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
