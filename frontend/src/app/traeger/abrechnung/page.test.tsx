import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingPeriod, BillingRun } from "~/lib/traeger-billing-api";

const { api, mockPush, mockReplace, route } = vi.hoisted(() => ({
  api: {
    fetchBillingRuns: vi.fn(),
    fetchBillingPeriods: vi.fn(),
    startBillingRun: vi.fn(),
  },
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
  route: { search: "" },
}));

vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  usePathname: () => "/abrechnung",
  useSearchParams: () => new URLSearchParams(route.search),
}));

import TraegerAbrechnungPage from "./page";

const PERIOD: BillingPeriod = {
  id: "5",
  name: "Winterferien 2027",
  startsOn: "2027-01-04",
  endsOn: "2027-01-15",
};

const RELEASED: BillingRun = {
  id: "9",
  holidayPeriodId: "5",
  periodName: "Herbstferien 2026",
  serviceFrom: "2026-10-12",
  serviceUntil: "2026-10-23",
  status: "released",
  createdAt: "2026-09-01T08:00:00Z",
  releasedAt: "2026-09-02T08:00:00Z",
  issuedOn: "2026-09-02",
  documentCount: 2,
  totalCents: 18000,
};

function renderPage() {
  render(
    <ToastProvider>
      <TraegerAbrechnungPage />
    </ToastProvider>,
  );
}

describe("TraegerAbrechnungPage", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
    mockPush.mockReset();
    mockReplace.mockReset();
    route.search = "";
    api.fetchBillingRuns.mockResolvedValue([RELEASED]);
    api.fetchBillingPeriods.mockResolvedValue([PERIOD]);
  });

  it("lists the runs with their invoices", async () => {
    renderPage();

    expect(
      (await screen.findAllByText("Herbstferien 2026"))[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("2 Rechnungen · 180,00 €")[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("Freigegeben")[0]).toBeInTheDocument();
  });

  it("starts a run for a period and opens it", async () => {
    api.startBillingRun.mockResolvedValue({
      ...RELEASED,
      id: "12",
      status: "calculated",
    });
    renderPage();
    await screen.findAllByText("Herbstferien 2026");

    fireEvent.click(screen.getByRole("button", { name: /Lauf starten/ }));
    const buttons = await screen.findAllByRole("button", {
      name: "Lauf starten",
    });
    fireEvent.click(buttons[buttons.length - 1]!);

    await waitFor(() => expect(api.startBillingRun).toHaveBeenCalledWith("5"));
    await waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith("/abrechnung/12"),
    );
  });

  it("opens the run in preview instead of starting a second one", async () => {
    api.fetchBillingRuns.mockResolvedValue([
      { ...RELEASED, id: "14", status: "calculated", releasedAt: null },
    ]);
    renderPage();
    await screen.findAllByText("Herbstferien 2026");

    fireEvent.click(screen.getByRole("button", { name: /Lauf starten/ }));
    expect(
      await screen.findByText(
        /Für diesen Ferienzeitraum ist schon ein Lauf in der Vorschau/,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Lauf öffnen" }));

    await waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith("/abrechnung/14"),
    );
    expect(api.startBillingRun).not.toHaveBeenCalled();
  });

  it("states that a run needs a published period", async () => {
    api.fetchBillingPeriods.mockResolvedValue([]);
    renderPage();

    expect(
      await screen.findByText(
        /Es gibt noch keinen veröffentlichten Ferienzeitraum/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Lauf starten/ }),
    ).toHaveAttribute("aria-disabled", "true");
  });

  describe("filtered to a holiday period (#5688)", () => {
    const OTHER: BillingRun = {
      ...RELEASED,
      id: "11",
      holidayPeriodId: "6",
      periodName: "Osterferien 2027",
      documentCount: 1,
      totalCents: 5000,
    };

    it("shows only the runs of the period from the address, as a removable filter", async () => {
      api.fetchBillingRuns.mockResolvedValue([RELEASED, OTHER]);
      route.search = "ferienzeitraum=6";
      renderPage();

      // Anker: der Lauf des gefilterten Zeitraums steht in der Liste.
      expect(
        (await screen.findAllByText("1 Rechnung · 50,00 €"))[0],
      ).toBeInTheDocument();
      expect(screen.queryAllByText("2 Rechnungen · 180,00 €")).toHaveLength(0);
      expect(screen.getByText("Abrechnungslauf")).toBeInTheDocument();

      fireEvent.click(
        screen.getAllByRole("button", {
          name: /Osterferien 2027 entfernen/,
        })[0]!,
      );
      expect(mockReplace).toHaveBeenCalledWith("/abrechnung", {
        scroll: false,
      });
    });

    it("explains how to start the missing run and preselects the period", async () => {
      api.fetchBillingRuns.mockResolvedValue([OTHER]);
      api.startBillingRun.mockResolvedValue({ ...RELEASED, id: "12" });
      api.fetchBillingPeriods.mockResolvedValue([
        { ...PERIOD, id: "8", name: "Sommerferien 2027" },
        PERIOD,
      ]);
      route.search = "ferienzeitraum=5";
      renderPage();

      expect(
        (
          await screen.findAllByText(
            "Noch kein Abrechnungslauf für Winterferien 2027.",
          )
        )[0],
      ).toBeInTheDocument();
      expect(
        screen.getAllByText(/Starten Sie ihn oben mit „Lauf starten“/)[0],
      ).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: /Lauf starten/ }));
      const buttons = await screen.findAllByRole("button", {
        name: "Lauf starten",
      });
      fireEvent.click(buttons[buttons.length - 1]!);
      await waitFor(() =>
        expect(api.startBillingRun).toHaveBeenCalledWith("5"),
      );
    });
  });
});
