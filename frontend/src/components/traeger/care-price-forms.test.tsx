import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CareTariffs } from "~/lib/traeger-care-billing-api";
import { ToastProvider } from "~/contexts/ToastContext";
import {
  freeMonthsAt,
  freeMonthsLabel,
  missingPrices,
  priceRowsAt,
} from "./care-billing-format";
import { CarePriceListModal } from "./care-price-list-modal";

const saveCarePriceList = vi.fn();

vi.mock("~/lib/traeger-care-billing-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-care-billing-api")>()),
  saveCarePriceList: (...args: unknown[]) =>
    saveCarePriceList(...args) as unknown,
}));

// Birkenfeld (#6196): Modul 4 by days per week, the meal on top; Modul 1
// changed from weekday prices to a scale in January.
const data: CareTariffs = {
  schools: [{ id: "12", name: "Grundschule Birkenfeld" }],
  modules: [
    {
      schoolId: "12",
      schoolName: "Grundschule Birkenfeld",
      careOfferingId: "704",
      moduleName: "Modul 4 mit Mittagessen",
      endsAt: "15:30",
      bookedWeekdays: ["mon", "tue", "wed"],
      contractCount: 2,
    },
    {
      schoolId: "12",
      schoolName: "Grundschule Birkenfeld",
      careOfferingId: "701",
      moduleName: "Modul 1 ohne Mittagessen",
      endsAt: "12:00",
      bookedWeekdays: ["mon", "tue"],
      contractCount: 1,
    },
  ],
  tariffs: [
    ...[2000, 4000, 6000, 8000, 10000].map((cents, i) => ({
      id: `4${i}`,
      schoolId: "12",
      careOfferingId: "704",
      moduleName: "Modul 4 mit Mittagessen",
      weekday: "" as const,
      daysPerWeek: i + 1,
      validFrom: "2027-01-01",
      validUntil: null,
      monthlyCents: cents,
      siblingCents: i === 2 ? 4500 : null,
      mealExtra: true,
    })),
    {
      id: "10",
      schoolId: "12",
      careOfferingId: "701",
      moduleName: "Modul 1 ohne Mittagessen",
      weekday: "mon",
      daysPerWeek: null,
      validFrom: "2026-09-01",
      validUntil: null,
      monthlyCents: 700,
      siblingCents: null,
      mealExtra: false,
    },
    {
      id: "11",
      schoolId: "12",
      careOfferingId: "701",
      moduleName: "Modul 1 ohne Mittagessen",
      weekday: "",
      daysPerWeek: 1,
      validFrom: "2027-01-01",
      validUntil: null,
      monthlyCents: 500,
      siblingCents: null,
      mealExtra: false,
    },
  ],
  lists: [{ schoolId: "12", validFrom: "2027-01-01", freeMonths: [8] }],
  billedUntil: {},
};

describe("price forms of the price list", () => {
  it("reads a scale with its sibling steps and the meal note", () => {
    const [module4] = priceRowsAt(data, "12", "2027-01-01");
    expect(module4?.form).toBe("days_per_week");
    expect(module4?.mealExtra).toBe(true);
    expect(module4?.steps[3]).toBe(6000);
    expect(module4?.siblingSteps[3]).toBe(4500);
    expect(module4?.prices).toEqual({});
  });

  it("takes the form of the latest price and drops the other form", () => {
    const december = priceRowsAt(data, "12", "2026-12-01")[1];
    expect(december?.form).toBe("weekday");
    expect(december?.prices).toEqual({ mon: 700 });
    const january = priceRowsAt(data, "12", "2027-01-01")[1];
    expect(january?.form).toBe("days_per_week");
    expect(january?.prices).toEqual({});
    // Modul 1 is booked on two days: the step for 2 days is missing.
    expect(january && missingPrices(january)).toEqual(["2 Tage"]);
  });

  it("names the free months of the list in force", () => {
    expect(freeMonthsAt(data, "12", "2026-12-01")).toEqual([]);
    expect(freeMonthsAt(data, "12", "2027-03-01")).toEqual([8]);
    expect(freeMonthsLabel([12, 7, 8])).toBe("Juli, August und Dezember");
  });
});

describe("CarePriceListModal", () => {
  beforeEach(() => {
    saveCarePriceList.mockReset();
    saveCarePriceList.mockResolvedValue(data);
  });

  function open() {
    render(
      <ToastProvider>
        <CarePriceListModal
          data={data}
          school={{ id: "12", name: "Grundschule Birkenfeld" }}
          currentMonth="2027-01-01"
          onClose={() => undefined}
          onSaved={() => undefined}
        />
      </ToastProvider>,
    );
  }

  it("saves the scale with sibling steps, the meal note and the free months", async () => {
    open();
    const sibling = await screen.findByRole("textbox", {
      name: "Modul 4 mit Mittagessen, 3 Tage je Woche, Jedes weitere Kind, Monatspreis in Euro",
    });
    expect(sibling).toHaveValue("45,00");
    fireEvent.click(screen.getByRole("button", { name: "Juli beitragsfrei" }));
    fireEvent.click(screen.getByRole("button", { name: "Preise speichern" }));
    await waitFor(() => expect(saveCarePriceList).toHaveBeenCalledTimes(1));
    const [schoolId, month, prices, freeMonths] = saveCarePriceList.mock
      .calls[0] as [string, string, unknown[], number[]];
    expect(schoolId).toBe("12");
    expect(month).toBe("2027-01-01");
    expect(freeMonths).toEqual([7, 8]);
    expect(prices).toContainEqual({
      careOfferingId: "704",
      moduleName: "Modul 4 mit Mittagessen",
      daysPerWeek: 3,
      monthlyCents: 6000,
      siblingCents: 4500,
      mealExtra: true,
    });
  });

  it("refuses a sibling price above the first child's", async () => {
    open();
    const sibling = await screen.findByRole("textbox", {
      name: "Modul 4 mit Mittagessen, 1 Tag je Woche, Jedes weitere Kind, Monatspreis in Euro",
    });
    fireEvent.change(sibling, { target: { value: "25,00" } });
    fireEvent.click(screen.getByRole("button", { name: "Preise speichern" }));
    expect(
      await screen.findByText("Höchstens so hoch wie beim 1. Kind."),
    ).toBeInTheDocument();
    expect(saveCarePriceList).not.toHaveBeenCalled();
  });
});
