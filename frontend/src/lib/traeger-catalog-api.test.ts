import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createTraegerOffering,
  fetchTraegerPhase,
  previewTraegerPhase,
  rolloutTraegerPhase,
  updateTraegerPhase,
} from "./traeger-catalog-api";

const request = vi.hoisted(() => vi.fn());
vi.mock("./traeger-api", () => ({ traegerRequest: request }));
afterEach(() => request.mockReset());

const backendPhase = {
  id: 7,
  organization_id: 2,
  draft: {
    name: "Schuljahr 2027/28",
    kind: "school_year",
    service_start_date: "2027-08-01",
    service_end_date: "2028-07-31",
    enrollment_open_at: null,
    enrollment_close_at: null,
    audience: "",
    care_overflow_mode: "",
    care_offering_selection_mode: "",
  },
  all_schools: false,
  released_version: 2,
  schools: [
    { school_id: 11, selected: true, applied_version: 2, outcome: "applied" },
    {
      school_id: 12,
      selected: true,
      outcome: "rejected",
      error_code: "enrollment.traeger_copy_rejected",
    },
  ],
  updated_at: "2026-11-11T10:00:00Z",
  offerings: [
    {
      id: 31,
      traeger_phase_id: 7,
      draft: {
        fields: {
          name: "Frühbetreuung",
          days_of_week_mode: "fixed",
          available_days: ["mon", "tue"],
          includes_holiday_care: false,
          includes_lunch: false,
          is_required: false,
          counts_as_care: true,
          pickup_times: { mon: "08:30", tue: "08:30" },
        },
        prices: [
          {
            weekday: "mon",
            monthly_cents: 650,
            sibling_cents: 500,
            meal_extra: false,
          },
          { weekday: "tue", monthly_cents: 650, meal_extra: false },
        ],
        contract_template: {
          term_kind: "school_year",
          notice_months: 1,
          notice_to: "month_end",
          special_termination: false,
          special_termination_note: "",
          statute_required: false,
          statute_title: "",
          statute_url: "",
          contract_partner: "",
        },
      },
      schools: [{ school_id: 11, applied_version: 1, outcome: "applied" }],
    },
  ],
};

describe("Träger catalog requests (#4704)", () => {
  it("maps IDs to strings, fills defaults and keeps a school's refusal code", async () => {
    request.mockResolvedValue(backendPhase);
    const phase = await fetchTraegerPhase("7");

    expect(request).toHaveBeenCalledWith("/api/traeger/catalog/phases/7");
    expect(phase.id).toBe("7");
    expect(phase.draft.audience).toBe("open");
    expect(phase.draft.careOverflowMode).toBe("waitlist");
    expect(phase.draft.careOfferingSelectionMode).toBe("optional");
    expect(phase.schools).toEqual([
      expect.objectContaining({
        schoolId: "11",
        outcome: "applied",
        errorCode: null,
      }),
      expect.objectContaining({
        schoolId: "12",
        outcome: "rejected",
        appliedVersion: null,
        errorCode: "enrollment.traeger_copy_rejected",
      }),
    ]);
    const offering = phase.offerings[0];
    expect(offering?.id).toBe("31");
    expect(offering?.draft.fields.selectionRule).toBe("optional");
    expect(offering?.draft.prices).toEqual([
      {
        weekday: "mon",
        daysPerWeek: null,
        monthlyCents: 650,
        siblingCents: 500,
        mealExtra: false,
      },
      {
        weekday: "tue",
        daysPerWeek: null,
        monthlyCents: 650,
        siblingCents: null,
        mealExtra: false,
      },
    ]);
    expect(offering?.draft.contractTemplate?.termMonths).toBeNull();
  });

  it("sends school IDs as numbers and none with „Alle Schulen“", async () => {
    request.mockResolvedValue(backendPhase);
    const input = {
      name: " Schuljahr ",
      kind: "school_year" as const,
      serviceStartDate: "2027-08-01",
      serviceEndDate: "2028-07-31",
      enrollmentOpenAt: null,
      enrollmentCloseAt: null,
      audience: "open" as const,
      careOverflowMode: "waitlist" as const,
      careOfferingSelectionMode: "optional" as const,
      allSchools: false,
      schoolIds: ["11", "12"],
    };
    await updateTraegerPhase("7", input);
    await updateTraegerPhase("7", { ...input, allSchools: true });

    const [first, second] = request.mock.calls.map(([, init]) => init.body);
    expect(first).toMatchObject({
      name: "Schuljahr",
      all_schools: false,
      school_ids: [11, 12],
    });
    expect(second).toMatchObject({ all_schools: true, school_ids: [] });
  });

  it("sends a scale price without weekday and leaves out an empty sibling price", async () => {
    request.mockResolvedValue(backendPhase.offerings[0]);
    await createTraegerOffering("7", {
      fields: {
        name: "Ganztag",
        description: "  ",
        daysOfWeekMode: "parent_choice",
        availableDays: ["mon"],
        includesHolidayCare: false,
        includesLunch: true,
        capacity: null,
        priceCents: null,
        isRequired: false,
        countsAsCare: true,
        selectionGroup: "",
        selectionRule: "optional",
        sortOrder: 0,
        pickupTimes: { mon: "16:00" },
      },
      prices: [
        {
          weekday: null,
          daysPerWeek: 3,
          monthlyCents: 9000,
          siblingCents: null,
          mealExtra: true,
        },
      ],
      contractTemplate: null,
    });

    const [path, init] = request.mock.calls[0]!;
    expect(path).toBe("/api/traeger/catalog/phases/7/offerings");
    expect(init.body.fields.description).toBeNull();
    expect(init.body.prices).toEqual([
      { days_per_week: 3, monthly_cents: 9000, meal_extra: true },
    ]);
    expect(init.body).not.toHaveProperty("contract_template");
  });

  it("reads the preview per school and confirms it with its fingerprint", async () => {
    request.mockResolvedValueOnce({
      traeger_phase_id: 7,
      version: 3,
      fingerprint: "abc",
      schools: [
        {
          school_id: 11,
          action: "update",
          outcome: "ready",
          phase: [
            { field: "name", state: "keeps", traeger: "Neu", school: "Eigen" },
          ],
          offerings: [
            {
              traeger_offering_id: 31,
              name: "Frühbetreuung",
              action: "update",
              fields: null,
              prices_from: "2027-09-01",
              contract_template: false,
            },
          ],
        },
      ],
    });
    const preview = await previewTraegerPhase("7");
    expect(preview.schools[0]).toMatchObject({
      schoolId: "11",
      errorCode: null,
      phase: [
        { field: "name", state: "keeps", traeger: "Neu", school: "Eigen" },
      ],
      offerings: [
        { traegerOfferingId: "31", fields: [], pricesFrom: "2027-09-01" },
      ],
    });

    request.mockResolvedValueOnce({
      traeger_phase_id: 7,
      version: 3,
      schools: [{ school_id: 11, outcome: "stale" }],
    });
    const rollout = await rolloutTraegerPhase("7", preview.fingerprint);
    expect(request).toHaveBeenLastCalledWith(
      "/api/traeger/catalog/phases/7/rollout",
      {
        method: "POST",
        body: { fingerprint: "abc" },
      },
    );
    expect(rollout.schools).toEqual([
      { schoolId: "11", outcome: "stale", errorCode: null },
    ]);
  });
});
