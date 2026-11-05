import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingPeriod, BillingRun } from "~/lib/traeger-billing-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchBillingRuns: vi.fn(),
    fetchBillingPeriods: vi.fn(),
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
    api.fetchBillingRuns.mockResolvedValue([RELEASED]);
    api.fetchBillingPeriods.mockResolvedValue([PERIOD]);
  });

  it("lists the runs with their invoices", async () => {
    renderPage();

    expect(
      (await screen.findAllByText("Herbstferien 2026"))[0],
    ).toBeInTheDocument();
    expect(screen.getAllByText("2 · 180,00 €")[0]).toBeInTheDocument();
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
});
