import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { BookingAccounts } from "~/lib/traeger-finance-exports-api";
import TraegerBookingAccountsPage from "./page";

const api = vi.hoisted(() => ({
  fetchBookingAccounts: vi.fn(),
  saveRevenueAccount: vi.fn(),
  addCostCenter: vi.fn(),
  deleteCostCenter: vi.fn(),
}));
vi.mock("~/lib/traeger-finance-exports-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

// The test clock stands on 2026-09-09.
const PAGE: BookingAccounts = {
  standardAccount: "4400",
  costCenterRequired: false,
  schools: [
    { id: "11", name: "GS Ispringen" },
    { id: "12", name: "GS Birkenfeld" },
  ],
  offerings: [{ schoolId: "11", offeringId: "501", moduleNames: ["Modul 1"] }],
  revenueAccounts: [{ scope: "meals", scopeId: "0", account: "4420" }],
  costCenters: [
    {
      id: "1",
      scope: "school",
      scopeId: "11",
      validFrom: "2026-01-01",
      code: "ISP",
    },
    {
      id: "2",
      scope: "school",
      scopeId: "11",
      validFrom: "2027-01-01",
      code: "ISP27",
    },
  ],
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerBookingAccountsPage />
    </ToastProvider>,
  );
}

beforeEach(() => {
  vi.resetAllMocks();
  api.fetchBookingAccounts.mockResolvedValue(PAGE);
});

it("shows where each Leistung books and which cost center holds today", async () => {
  renderPage();
  const meals = (await screen.findByText("Mensa")).closest("li")!;
  expect(within(meals).getByText("4420")).toBeInTheDocument();
  const care = screen.getByText("Monatsbeiträge").closest("li")!;
  expect(within(care).getByText("4400")).toBeInTheDocument();
  expect(within(care).getByText("Standard-Erlöskonto")).toBeInTheDocument();
  const offering = screen.getAllByText("Modul 1")[0]!.closest("li")!;
  expect(within(offering).getByText("Standard-Erlöskonto")).toBeInTheDocument();

  const school = screen
    .getByText("GS Ispringen", { selector: "p.font-semibold" })
    .closest("li")!;
  expect(within(school).getByText("ISP")).toBeInTheDocument();
  expect(within(school).getByText("seit 01.01.2026")).toBeInTheDocument();
  expect(within(school).getByText("ISP27")).toBeInTheDocument();
  const other = screen
    .getByText("GS Birkenfeld", { selector: "p.font-semibold" })
    .closest("li")!;
  expect(within(other).getByText("Keine Kostenstelle")).toBeInTheDocument();
  expect(screen.getByText("1 von 2")).toBeInTheDocument();
});

it("saves the revenue account of a Leistung after checking it like the export", async () => {
  api.saveRevenueAccount.mockResolvedValue({
    ...PAGE,
    revenueAccounts: [
      ...PAGE.revenueAccounts,
      { scope: "care", scopeId: "0", account: "4410" },
    ],
  });
  renderPage();
  fireEvent.click(
    await screen.findByRole("button", {
      name: "Erlöskonto für Monatsbeiträge ändern",
    }),
  );
  const field = screen.getByLabelText("Erlöskonto für Monatsbeiträge");
  fireEvent.change(field, { target: { value: "0000" } });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  expect(
    await screen.findByText("Nur Ziffern, nicht nur Nullen."),
  ).toBeInTheDocument();
  expect(api.saveRevenueAccount).not.toHaveBeenCalled();

  fireEvent.change(field, { target: { value: " 4410 " } });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  await waitFor(() =>
    expect(api.saveRevenueAccount).toHaveBeenCalledWith("care", "0", "4410"),
  );
  const care = (await screen.findByText("Monatsbeiträge")).closest("li")!;
  expect(await within(care).findByText("4410")).toBeInTheDocument();
});

it("adds a cost center from a day and keeps a refused one in the form", async () => {
  api.addCostCenter.mockRejectedValueOnce(
    new ApiError("taken", 409, {
      code: "parent_billing.cost_center_date_taken",
    }),
  );
  api.addCostCenter.mockResolvedValueOnce(PAGE);
  renderPage();
  fireEvent.click(
    await screen.findByRole("button", { name: "Neue KOST1 für GS Birkenfeld" }),
  );
  fireEvent.change(screen.getByLabelText("Kostenstelle"), {
    target: { value: "BIR-1" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  expect(
    await screen.findByText(
      "Nur Buchstaben ohne Umlaute, Ziffern, Leerzeichen und _.",
    ),
  ).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Kostenstelle"), {
    target: { value: "BIR 1" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  await waitFor(() =>
    expect(api.addCostCenter).toHaveBeenCalledWith({
      scope: "school",
      scopeId: "12",
      validFrom: "2026-09-09",
      code: "BIR 1",
    }),
  );
  expect(
    await screen.findByText(/Ab diesem Tag gibt es schon eine Kostenstelle/),
  ).toBeInTheDocument();
  expect(screen.getByLabelText("Kostenstelle")).toHaveValue("BIR 1");
  fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
  await waitFor(() => expect(api.addCostCenter).toHaveBeenCalledTimes(2));
  await waitFor(() =>
    expect(screen.queryByLabelText("Kostenstelle")).toBeNull(),
  );
  expect(
    screen.getByRole("button", { name: "Neue KOST1 für GS Birkenfeld" }),
  ).toBeEnabled();
});

it("names a missing standard account with the way to the DATEV values", async () => {
  api.fetchBookingAccounts.mockResolvedValue({
    ...PAGE,
    standardAccount: "",
    revenueAccounts: [],
  });
  renderPage();
  expect(
    await screen.findByText("Das Standard-Erlöskonto fehlt"),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "DATEV-Werte eintragen" }),
  ).toHaveAttribute("href", "/traeger/einstellungen?bereich=datev");
  const meals = screen.getByText("Mensa").closest("li")!;
  expect(within(meals).getByText("Kein Konto")).toBeInTheDocument();
});
