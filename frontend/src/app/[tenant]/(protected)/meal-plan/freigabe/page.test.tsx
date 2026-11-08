import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type {
  MealBillingMonth,
  MealBillingSummary,
} from "~/lib/meal-billing-api";

import MealBillingReleasePage from "./page";

const {
  fetchMealBillingMonths,
  fetchMealBillingMonth,
  releaseMealBillingMonth,
} = vi.hoisted(() => ({
  fetchMealBillingMonths: vi.fn(),
  fetchMealBillingMonth: vi.fn(),
  releaseMealBillingMonth: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/schule/meal-plan/freigabe",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/meal-billing-api", () => ({
  fetchMealBillingMonths,
  fetchMealBillingMonth,
  releaseMealBillingMonth,
}));

const MARCH: MealBillingMonth = {
  month: "2026-03-01",
  status: "open",
  schoolDays: 20,
  releasedAt: null,
  childCount: 2,
  mealCount: 5,
  absenceCount: 1,
  children: [
    {
      studentId: "1",
      firstName: "Mia",
      lastName: "Albers",
      schoolClass: "2a",
      registeredDays: 4,
      mealDays: 3,
      absentDates: ["2026-03-04"],
    },
    {
      studentId: "2",
      firstName: "Ben",
      lastName: "Brandt",
      schoolClass: "1b",
      registeredDays: 2,
      mealDays: 2,
      absentDates: [],
    },
  ],
};

const MONTHS: MealBillingSummary[] = [
  {
    month: "2026-04-01",
    status: "running",
    releasedAt: null,
    childCount: null,
    mealCount: null,
  },
  {
    month: "2026-03-01",
    status: "open",
    releasedAt: null,
    childCount: null,
    mealCount: null,
  },
];

function renderPage() {
  return render(
    <ToastProvider>
      <MealBillingReleasePage />
    </ToastProvider>,
  );
}

describe("MealBillingReleasePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchMealBillingMonths.mockResolvedValue(MONTHS);
    fetchMealBillingMonth.mockResolvedValue(MARCH);
  });

  it("opens the month that waits for its release with each child's absences", async () => {
    renderPage();
    expect((await screen.findAllByText("Albers, Mia")).length).toBeGreaterThan(0);
    expect(fetchMealBillingMonth).toHaveBeenCalledWith("2026-03-01");
    expect(screen.getAllByText("Brandt, Ben").length).toBeGreaterThan(0);
    expect(screen.getAllByText(/04\.03\./).length).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Monat freigeben" }),
    ).toBeInTheDocument();
  });

  it("releases with the children and meals the Leitung saw", async () => {
    releaseMealBillingMonth.mockResolvedValue({
      ...MARCH,
      status: "released",
      releasedAt: "2026-04-02T09:00:00Z",
    });
    renderPage();
    fireEvent.click(
      await screen.findByRole("button", { name: "Monat freigeben" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Freigeben" }));
    await waitFor(() =>
      expect(releaseMealBillingMonth).toHaveBeenCalledWith(MARCH),
    );
    expect(
      await screen.findByText(/Der Träger rechnet diesen Monat ab/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Monat freigeben" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByText("Albers, Mia").length).toBeGreaterThan(0);
  });

  it("explains that a running month waits for its end", async () => {
    fetchMealBillingMonths.mockResolvedValue([MONTHS[0]]);
    fetchMealBillingMonth.mockResolvedValue({
      ...MARCH,
      month: "2026-04-01",
      status: "running",
    });
    renderPage();
    expect(
      await screen.findByText(
        "Der Monat läuft noch. Sie können ihn ab dem 1. Mai 2026 freigeben.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Albers, Mia").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Monat freigeben" }),
    ).not.toBeInTheDocument();
  });
});
