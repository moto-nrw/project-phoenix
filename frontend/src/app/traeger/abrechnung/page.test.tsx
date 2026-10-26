import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingPeriod, BillingRun } from "~/lib/traeger-billing-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchBillingRuns: vi.fn(),
    fetchBillingPeriods: vi.fn(),
    fetchHolidayNumberRange: vi.fn(),
    saveHolidayNumberRange: vi.fn(),
    startBillingRun: vi.fn(),
  },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-billing-api", () => api);
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
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
    api.fetchBillingRuns.mockResolvedValue([RELEASED]);
    api.fetchBillingPeriods.mockResolvedValue([PERIOD]);
    api.fetchHolidayNumberRange.mockResolvedValue({
      service: "holiday",
      pattern: "FE-{JJJJ}-{NNNN}",
      example: "FE-2026-0001",
    });
  });

  it("lists the runs with their invoices and the number range", async () => {
    renderPage();

    expect(
      (await screen.findAllByText("Herbstferien 2026"))[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("2 · 180,00 €")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Freigegeben")[0]).toBeInTheDocument();
    expect(screen.getByText("FE-2026-0001")).toBeInTheDocument();
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

  it("states that a run needs a published period", async () => {
    api.fetchBillingPeriods.mockResolvedValue([]);
    renderPage();

    expect(
      await screen.findByText(
        /Es gibt noch keinen veröffentlichten Ferienzeitraum/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Lauf starten/ })).toBeDisabled();
  });

  it("asks for a pattern before the first release and saves it", async () => {
    api.fetchHolidayNumberRange.mockResolvedValue({
      service: "holiday",
      pattern: "",
      example: "",
    });
    api.saveHolidayNumberRange.mockResolvedValue({
      service: "holiday",
      pattern: "R-{NNN}",
      example: "R-001",
    });
    renderPage();

    expect(
      await screen.findByText(/Ohne Muster können Sie keinen Lauf freigeben/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Muster festlegen" }));
    fireEvent.change(
      await screen.findByLabelText("Muster der Rechnungsnummer"),
      { target: { value: "R-{NNN}" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.saveHolidayNumberRange).toHaveBeenCalledWith("R-{NNN}"),
    );
    await waitFor(() =>
      expect(api.fetchHolidayNumberRange).toHaveBeenCalledTimes(2),
    );
  });
});
