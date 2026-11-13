import { describe, expect, it } from "vitest";
import {
  blankOfferingDraft,
  moduleDraftOf,
  offeringDraftFromForm,
  type OfferingFormDraft,
} from "./catalog-offering-form";
import { validatePhaseInput, blankPhaseInput } from "./catalog-phase-form";

function form(patch: Partial<OfferingFormDraft> = {}): OfferingFormDraft {
  const blank = blankOfferingDraft();
  return {
    ...blank,
    fields: {
      ...blank.fields,
      name: "Frühbetreuung",
      availableDays: ["mon", "tue"],
      pickupTimes: { mon: "08:30", tue: "08:30" },
    },
    ...patch,
  };
}

describe("offeringDraftFromForm (#4704)", () => {
  it("turns weekday prices with a sibling price into the backend's prices", () => {
    const { draft, errors } = offeringDraftFromForm(
      form({
        capacity: "20",
        priceInForm: "32,5",
        prices: {
          form: "weekday",
          mealExtra: true,
          first: { mon: "6,50", tue: "7" },
          further: { mon: "5,00" },
        },
      }),
    );
    expect(errors).toEqual({});
    expect(draft?.fields).toMatchObject({ capacity: 20, priceCents: 3250 });
    expect(draft?.prices).toEqual([
      {
        weekday: "mon",
        daysPerWeek: null,
        monthlyCents: 650,
        siblingCents: 500,
        mealExtra: true,
      },
      {
        weekday: "tue",
        daysPerWeek: null,
        monthlyCents: 700,
        siblingCents: null,
        mealExtra: true,
      },
    ]);
  });

  it("keeps a scale by days per week", () => {
    const { draft } = offeringDraftFromForm(
      form({
        prices: {
          form: "days_per_week",
          mealExtra: false,
          first: { "3": "90", "5": "120" },
          further: {},
        },
      }),
    );
    expect(
      draft?.prices.map((price) => [price.daysPerWeek, price.monthlyCents]),
    ).toEqual([
      [3, 9000],
      [5, 12000],
    ]);
    expect(draft?.prices.every((price) => price.weekday === null)).toBe(true);
  });

  it("refuses a sibling price above the first child's and a care day without pickup time", () => {
    const { draft, errors } = offeringDraftFromForm(
      form({
        fields: { ...form().fields, pickupTimes: { mon: "08:30" } },
        prices: {
          form: "weekday",
          mealExtra: false,
          first: { mon: "6,50" },
          further: { mon: "8,00" },
        },
      }),
    );
    expect(draft).toBeNull();
    expect(errors).toEqual({
      "pickup_times.tue": "Bitte eine Gehzeit eintragen.",
      "angebot:mon:weitere": "Höchstens so hoch wie beim 1. Kind.",
    });
  });

  it("reads stored prices back into the price table", () => {
    expect(
      moduleDraftOf([
        {
          weekday: null,
          daysPerWeek: 2,
          monthlyCents: 4000,
          siblingCents: 3000,
          mealExtra: true,
        },
      ]),
    ).toEqual({
      form: "days_per_week",
      mealExtra: true,
      first: { "2": "40,00" },
      further: { "2": "30,00" },
    });
  });
});

describe("validatePhaseInput", () => {
  it("suggests the next school year and asks for a name and a school", () => {
    const blank = blankPhaseInput("2026-09-09");
    expect(blank.serviceStartDate).toBe("2027-08-01");
    expect(blank.serviceEndDate).toBe("2028-07-31");
    expect(validatePhaseInput({ ...blank, allSchools: false })).toEqual({
      name: "Bitte geben Sie einen Namen ein.",
      school_ids: "Bitte wählen Sie mindestens eine Schule.",
    });
    expect(
      validatePhaseInput({
        ...blank,
        name: "Schuljahr",
        serviceEndDate: "2027-07-01",
      }),
    ).toEqual({ service_end_date: "Das Ende liegt vor dem Beginn." });
  });
});
