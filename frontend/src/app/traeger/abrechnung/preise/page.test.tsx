import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareTariffs } from "~/lib/traeger-care-billing-api";
import type { CareRule } from "~/lib/traeger-care-reductions-api";
import { setTestClock } from "~/test/clock";

const { fetchCareTariffs, fetchCareRules } = vi.hoisted(() => ({
  fetchCareTariffs: vi.fn(),
  fetchCareRules: vi.fn(),
}));
vi.mock("~/lib/traeger-care-billing-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-care-billing-api")>()),
  fetchCareTariffs,
}));
vi.mock("~/lib/traeger-care-reductions-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-care-reductions-api")
  >()),
  fetchCareRules,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  usePathname: () => "/abrechnung/preise",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

import TraegerPreislistePage from "./page";

const data: CareTariffs = {
  schools: [{ id: "11", name: "Sonnenhof" }],
  modules: ["A", "B", "C"].map((name, index) => ({
    schoolId: "11",
    schoolName: "Sonnenhof",
    careOfferingId: String(index + 101),
    moduleName: `Modul ${name}`,
    endsAt: "14:00",
    bookedWeekdays: ["mon"],
    bookedDaysPerWeek: [1],
    contractCount: 1,
  })),
  tariffs: ["A", "B", "C"].map((name, index) => ({
    id: String(index + 1),
    schoolId: "11",
    careOfferingId: String(index + 101),
    moduleName: `Modul ${name}`,
    weekday: index === 2 ? "" : "mon",
    daysPerWeek: index === 2 ? 1 : null,
    validFrom: "2026-01-01",
    validUntil: null,
    monthlyCents: 1000,
    siblingCents: 500,
  })),
  billedUntil: {},
};

const rule: CareRule = {
  id: "1",
  kind: "sibling",
  name: "Geschwisterpreise",
  validFrom: "2026-01-01",
  validUntil: null,
  schoolIds: [],
  careOfferingIds: [],
  countSchoolIds: [],
  siblingFromChild: 2,
  discountBasisPoints: null,
  discountCents: null,
  priceCents: null,
  capCents: null,
  tariffSiblingPrice: true,
  countsFamilyChildren: false,
};

function renderPage(rules: readonly CareRule[]) {
  fetchCareRules.mockResolvedValue({ rules });
  return render(
    <ToastProvider>
      <TraegerPreislistePage />
    </ToastProvider>,
  );
}

function expectInactive(moduleName: string, inactive: boolean) {
  const row = screen
    .getAllByRole("row")
    .find((entry) => within(entry).queryByText(moduleName));
  expect(row).toBeDefined();
  const cells = within(row!);
  expect(cells.getByText("5,00 €")).toBeInTheDocument();
  expect(cells.queryByText("(gilt zurzeit nicht)") !== null).toBe(inactive);
}

describe("sibling prices of the displayed price list", () => {
  beforeEach(() => {
    setTestClock("2026-11-12T12:00:00+01:00");
    fetchCareTariffs.mockResolvedValue(data);
  });

  it.each(["101", "103"])(
    "applies a module-restricted rule only to module %s, for either price form",
    async (careOfferingId) => {
      renderPage([{ ...rule, careOfferingIds: [careOfferingId] }]);
      await screen.findAllByText("Modul A");
      await waitFor(() => {
        expectInactive("Modul A", careOfferingId !== "101");
        expectInactive("Modul B", true);
        expectInactive("Modul C", careOfferingId !== "103");
      });
      expect(screen.getByText(/„Modul B“/)).toHaveTextContent(/gelten.*nicht/);
    },
  );

  it.each([
    { validFrom: "2026-09-01", validUntil: null, inactive: false },
    { validFrom: "2026-01-01", validUntil: "2026-08-31", inactive: true },
  ])(
    "uses November for the list that started in January: %j",
    async (dates) => {
      renderPage([{ ...rule, ...dates }]);
      await screen.findAllByText("Modul A");
      await waitFor(() => expectInactive("Modul A", dates.inactive));
    },
  );

  it("keeps historical and future lists scoped to their own months", async () => {
    fetchCareTariffs.mockResolvedValue({
      ...data,
      tariffs: data.tariffs.map((tariff) => ({
        ...tariff,
        validFrom: "2025-01-01",
      })),
      lists: [
        { schoolId: "11", validFrom: "2025-01-01", freeMonths: [] },
        { schoolId: "11", validFrom: "2026-01-01", freeMonths: [] },
        { schoolId: "11", validFrom: "2027-01-01", freeMonths: [] },
      ],
    });
    renderPage([{ ...rule, validUntil: "2026-12-31" }]);
    await screen.findAllByText("Modul A");
    await waitFor(() => expectInactive("Modul A", false));
    fireEvent.click(screen.getByRole("combobox", { name: "Preise anzeigen" }));
    fireEvent.click(screen.getByRole("option", { name: /Januar 2025/ }));
    await waitFor(() => expectInactive("Modul A", true));
    fireEvent.click(screen.getByRole("combobox", { name: "Preise anzeigen" }));
    fireEvent.click(screen.getByRole("option", { name: /Januar 2027/ }));
    await waitFor(() => expectInactive("Modul A", true));
    fireEvent.click(screen.getByRole("combobox", { name: "Preise anzeigen" }));
    fireEvent.click(screen.getByRole("option", { name: /Januar 2026/ }));
    await waitFor(() => expectInactive("Modul A", false));
  });
});
