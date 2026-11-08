import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
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
  fingerprint: "m3",
  version: 0,
  releases: [],
  current: null,
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

function monthLoadError() {
  return new ApiError("month unavailable", 503, {
    code: "general.unavailable",
  });
}

describe("MealBillingReleasePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchMealBillingMonths.mockResolvedValue(MONTHS);
    fetchMealBillingMonth.mockResolvedValue(MARCH);
  });

  it("opens the month that waits for its release with each child's absences", async () => {
    renderPage();
    expect((await screen.findAllByText("Albers, Mia")).length).toBeGreaterThan(
      0,
    );
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

  it.each(["data", "error"])(
    "ignores stale month %s after selecting another month",
    async (response) => {
      let resolve!: (value: MealBillingMonth) => void;
      let reject!: (reason: Error) => void;
      const promise = new Promise<MealBillingMonth>((done, fail) => {
        resolve = done;
        reject = fail;
      });
      const april = {
        ...MARCH,
        month: "2026-04-01",
        children: [{ ...MARCH.children[0]!, lastName: "April" }],
      };
      fetchMealBillingMonth.mockImplementation((value: string) =>
        value === MARCH.month ? promise : Promise.resolve(april),
      );
      releaseMealBillingMonth.mockResolvedValue({
        ...april,
        status: "released",
      });
      renderPage();
      await waitFor(() =>
        expect(fetchMealBillingMonth).toHaveBeenCalledWith(MARCH.month),
      );
      fireEvent.click(screen.getByRole("combobox", { name: "Monat" }));
      fireEvent.click(
        await screen.findByRole("option", { name: "April 2026 · Läuft noch" }),
      );
      expect((await screen.findAllByText("April, Mia")).length).toBeGreaterThan(
        0,
      );
      await act(async () => {
        if (response === "data") resolve(MARCH);
        else reject(monthLoadError());
        await promise.catch(() => undefined);
      });
      // Anker: Die Daten des aktuell gewählten Monats bleiben sichtbar.
      expect(screen.getAllByText("April, Mia").length).toBeGreaterThan(0);
      expect(
        screen.queryByText(
          catalogText("general.unavailable", "den Mensa-Monat"),
        ),
      ).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Monat freigeben" }));
      fireEvent.click(await screen.findByRole("button", { name: "Freigeben" }));
      await waitFor(() =>
        expect(releaseMealBillingMonth).toHaveBeenCalledWith(april),
      );
    },
  );

  it("shows errors for the active month", async () => {
    fetchMealBillingMonth.mockRejectedValue(monthLoadError());
    renderPage();
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "den Mensa-Monat"),
      ),
    ).toBeInTheDocument();
  });

  it("names a month past the attendance retention as no longer billable, without a release", async () => {
    fetchMealBillingMonths.mockResolvedValue([
      { ...MONTHS[1], status: "expired" },
    ]);
    fetchMealBillingMonth.mockResolvedValue({
      ...MARCH,
      status: "expired",
      childCount: 0,
      mealCount: 0,
      absenceCount: 0,
      children: [],
    });
    renderPage();
    expect(
      await screen.findByText(
        "Nicht mehr abrechenbar: Die Anwesenheiten dieses Monats sind nach der Aufbewahrungsfrist gelöscht. Eine Freigabe ist nicht mehr möglich.",
      ),
    ).toBeInTheDocument();
    expect(fetchMealBillingMonth).toHaveBeenCalledWith("2026-03-01");
    expect(
      screen.queryByRole("button", { name: "Monat freigeben" }),
    ).not.toBeInTheDocument();
  });

  it("releases a corrected month again with the reason and the new counts", async () => {
    const corrected: MealBillingMonth = {
      ...MARCH,
      mealCount: 6,
      absenceCount: 0,
      fingerprint: "m3b",
    };
    const released: MealBillingMonth = {
      ...MARCH,
      status: "released",
      releasedAt: "2026-04-02T09:00:00Z",
      fingerprint: "",
      version: 1,
      releases: [
        {
          version: 1,
          releasedAt: "2026-04-02T09:00:00Z",
          reason: "",
          childCount: 2,
          mealCount: 5,
        },
      ],
      current: corrected,
    };
    fetchMealBillingMonth.mockResolvedValue(released);
    releaseMealBillingMonth.mockResolvedValue({
      ...released,
      current: null,
      version: 2,
      mealCount: 6,
    });
    renderPage();
    expect(
      await screen.findByText(
        "Seit der Freigabe hat sich die Anwesenheit geändert: jetzt 6 Essen statt 5. Prüfen Sie die Liste und geben Sie den Monat erneut frei.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Erneut freigeben" }));
    const confirm = await screen.findByRole("button", { name: "Freigeben" });
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/Grund/), {
      target: { value: "Anwesenheit nachgetragen" },
    });
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(releaseMealBillingMonth).toHaveBeenCalledWith(
        corrected,
        "Anwesenheit nachgetragen",
      ),
    );
  });
});
