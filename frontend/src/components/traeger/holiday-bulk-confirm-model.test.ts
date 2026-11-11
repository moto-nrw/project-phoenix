import { describe, expect, it } from "vitest";
import { ApiError, unavailableApiError } from "~/lib/api-error";
import type { HolidayRegistrationSummary } from "~/lib/traeger-holiday-registrations-api";
import type { HolidaySiblingGroup } from "~/lib/traeger-sibling-groups-api";
import { HOLIDAY_REGISTRATION } from "~/test/fixtures/holiday-registrations";
import {
  bulkConfirmOutcome,
  planHolidayBulkConfirmation,
} from "./holiday-bulk-confirm-model";

const BASE: HolidayRegistrationSummary = {
  ...HOLIDAY_REGISTRATION,
  children: HOLIDAY_REGISTRATION.children.map((child) => ({
    ...child,
    duplicateRegistrationIds: [],
  })),
};

function registration(
  id: string,
  change: Partial<HolidayRegistrationSummary> = {},
): HolidayRegistrationSummary {
  return { ...BASE, id, ...change };
}

const PENDING_CHILD = {
  ...BASE.children[0]!,
  bookings: [{ ...BASE.children[0]!.bookings[0]!, placement: "pending" }],
} as const;

const NO_GROUPS: readonly HolidaySiblingGroup[] = [];

describe("planHolidayBulkConfirmation", () => {
  it("confirms what needs no further input and names why the rest stays open", () => {
    const plan = planHolidayBulkConfirmation(
      [
        registration("1"),
        registration("2", { status: "confirmed" }),
        registration("3", { status: "cancelled" }),
        registration("4", { children: [PENDING_CHILD] }),
        registration("5", { benefitsDeclared: true }),
        registration("6", {
          children: [
            { ...BASE.children[0]!, duplicateRegistrationIds: ["11"] },
          ],
        }),
      ],
      { debtorNumberRequired: false, groups: NO_GROUPS, known: [] },
    );

    expect(plan.confirmable.map((entry) => entry.id)).toEqual(["1"]);
    expect(
      plan.skipped.map((entry) => [entry.registration.id, entry.reason]),
    ).toEqual([
      ["2", "not_received"],
      ["3", "not_received"],
      ["4", "place_open"],
      ["5", "payer_open"],
      ["6", "duplicate"],
    ]);
  });

  it("names the first reason when several apply", () => {
    const plan = planHolidayBulkConfirmation(
      [
        registration("1", {
          benefitsDeclared: true,
          children: [PENDING_CHILD],
        }),
      ],
      { debtorNumberRequired: true, groups: NO_GROUPS, known: [] },
    );

    expect(plan.skipped).toEqual([
      expect.objectContaining({ reason: "place_open" }),
    ]);
  });

  it("asks for the debtor number only while the Träger asks for it", () => {
    const registrations = [registration("1"), registration("2")];
    const known = [registration("9", { debtorNumber: "10 4711" })];

    const off = planHolidayBulkConfirmation(registrations, {
      debtorNumberRequired: false,
      groups: NO_GROUPS,
      known,
    });
    expect(off.confirmable).toHaveLength(2);

    const on = planHolidayBulkConfirmation(
      [...registrations, registration("3", { debtorNumber: "10 0815" })],
      { debtorNumberRequired: true, groups: NO_GROUPS, known },
    );
    expect(on.confirmable.map((entry) => entry.id)).toEqual(["3"]);
    expect(on.skipped.map((entry) => entry.reason)).toEqual([
      "debtor_number_missing",
      "debtor_number_missing",
    ]);
  });

  it("lets a sibling take the number of its group (#5628)", () => {
    const groups = [{ id: "g", registrationIds: ["1", "9"] }];
    const known = [
      registration("1"),
      registration("9", { status: "confirmed", debtorNumber: "10 4711" }),
    ];

    const plan = planHolidayBulkConfirmation([registration("1")], {
      debtorNumberRequired: true,
      groups,
      known,
    });
    expect(plan.confirmable.map((entry) => entry.id)).toEqual(["1"]);
  });

  it("tries a sibling whose group number the list does not show", () => {
    // The backend knows the group's number; it refuses the confirmation
    // itself when there is none.
    const plan = planHolidayBulkConfirmation([registration("1")], {
      debtorNumberRequired: true,
      groups: [{ id: "g", registrationIds: ["1", "9"] }],
      known: [registration("1")],
    });
    expect(plan.confirmable.map((entry) => entry.id)).toEqual(["1"]);
  });
});

describe("bulkConfirmOutcome", () => {
  it("reads the refusal of one registration from its code", () => {
    expect(
      bulkConfirmOutcome(
        new ApiError("", 409, {
          code: "enrollment.holiday_allocation_pending",
        }),
      ),
    ).toEqual({ reason: "place_open" });
    expect(
      bulkConfirmOutcome(
        new ApiError("", 400, { code: "parent_billing.debtor_number_invalid" }),
      ),
    ).toEqual({ reason: "debtor_number_missing" });
    expect(
      bulkConfirmOutcome(
        new ApiError("", 409, {
          code: "enrollment.holiday_registration_state_conflict",
        }),
      ),
    ).toEqual({ reason: "changed" });
    expect(
      bulkConfirmOutcome(
        new ApiError("", 409, {
          code: "parent_billing.benefits_exemption_unavailable",
        }),
      ),
    ).toEqual({ reason: "failed" });
    expect(bulkConfirmOutcome(new ApiError("", 500))).toEqual({
      reason: "failed",
    });
  });

  it("stops the run when no further confirmation can succeed", () => {
    expect(bulkConfirmOutcome(new ApiError("", 401))).toBe("stop");
    expect(bulkConfirmOutcome(new ApiError("", 403))).toBe("stop");
    expect(bulkConfirmOutcome(unavailableApiError())).toBe("stop");
    expect(bulkConfirmOutcome(new Error("boom"))).toBe("stop");
  });
});
