import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  CareCostShare,
  CareCostShareSet,
} from "~/lib/traeger-care-cost-shares-api";
import TraegerKostentraegerPage from "./page";

const { fetchCareCostShares, saveCareCostShare } = vi.hoisted(() => ({
  fetchCareCostShares: vi.fn(),
  saveCareCostShare: vi.fn(),
}));
vi.mock("~/lib/traeger-care-cost-shares-api", () => ({
  fetchCareCostShares,
  saveCareCostShare,
  deleteCareCostShare: vi.fn(),
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  usePathname: () => "/abrechnung/kostentraeger",
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

const share = (overrides: Partial<CareCostShare>): CareCostShare => ({
  id: "1",
  contractId: "1",
  kind: "youth_office",
  costBearerId: "7",
  costBearerName: "Jugendamt Enzkreis",
  shareBasisPoints: 10000,
  shareCents: null,
  scope: "care",
  validFrom: "2026-08-01",
  validUntil: "2027-07-31",
  reference: "Bescheid 4-12",
  checkedOn: "2026-08-20",
  status: "active",
  ...overrides,
});

const data: CareCostShareSet = {
  costBearers: [
    { id: "7", name: "Jugendamt Enzkreis", active: true },
    { id: "8", name: "Jobcenter Enzkreis", active: true },
  ],
  expiring: 1,
  warningDays: 30,
  contracts: [
    {
      contractId: "1",
      schoolId: "11",
      schoolName: "Schule am Berg",
      childFirstName: "Mia",
      childLastName: "Beispiel",
      shares: [share({})],
    },
    {
      contractId: "2",
      schoolId: "11",
      schoolName: "Schule am Berg",
      childFirstName: "Noah",
      childLastName: "Muster",
      shares: [
        share({
          id: "2",
          contractId: "2",
          kind: "education_participation",
          costBearerId: "8",
          costBearerName: "Jobcenter Enzkreis",
          shareBasisPoints: null,
          shareCents: 2500,
          validUntil: "2026-09-30",
          status: "expiring",
        }),
      ],
    },
    {
      contractId: "3",
      schoolId: "11",
      schoolName: "Schule am Berg",
      childFirstName: "Lea",
      childLastName: "Ohne",
      shares: [],
    },
  ],
};

function renderPage() {
  fetchCareCostShares.mockResolvedValue(data);
  render(
    <ToastProvider>
      <TraegerKostentraegerPage />
    </ToastProvider>,
  );
}

describe("TraegerKostentraegerPage", () => {
  it("shows each share with its amount and warns before a decision ends", async () => {
    renderPage();
    expect((await screen.findAllByText("100 %"))[0]).toBeInTheDocument();
    expect(screen.getAllByText("25,00 € im Monat")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Endet am 30.09.2026")[0]).toBeInTheDocument();
    expect(
      screen.getByText(/Ein Bescheid endet in den nächsten 30 Tagen/),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Kein Anteil, die Familie zahlt alles")[0],
    ).toBeInTheDocument();
  });

  it("narrows the list to the shares that end soon", async () => {
    renderPage();
    await screen.findAllByText("100 %");
    fireEvent.click(screen.getByRole("button", { name: "Nur diese zeigen" }));
    expect(screen.getAllByText("Noah Muster")[0]).toBeInTheDocument();
    expect(screen.queryAllByText("Mia Beispiel")).toHaveLength(0);
  });

  it("sends a fixed monthly amount in cent with its decision", async () => {
    renderPage();
    await screen.findAllByText("100 %");
    saveCareCostShare.mockResolvedValue(data);
    fireEvent.click(
      screen.getAllByRole("button", { name: "Anteile: Lea Ohne" })[0]!,
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Anteil eintragen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Bildung und Teilhabe" }),
    );
    fireEvent.click(
      within(dialog).getByRole("combobox", { name: /Kostenträger/ }),
    );
    fireEvent.click(
      await screen.findByRole("option", { name: "Jobcenter Enzkreis" }),
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Fester Betrag im Monat" }),
    );
    fireEvent.change(within(dialog).getByLabelText("Betrag in Euro"), {
      target: { value: "25,50" },
    });
    fireEvent.change(
      within(dialog).getByLabelText(/Bescheid/, { selector: "input" }),
      {
        target: { value: "BuT-Bescheid 2026-117" },
      },
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Anteil speichern" }),
    );
    await vi.waitFor(() => expect(saveCareCostShare).toHaveBeenCalled());
    expect(saveCareCostShare).toHaveBeenCalledWith(
      null,
      expect.objectContaining({
        contractId: "3",
        kind: "education_participation",
        costBearerId: "8",
        shareBasisPoints: null,
        shareCents: 2550,
        scope: "meal",
        reference: "BuT-Bescheid 2026-117",
      }),
    );
  });

  it("lets a share cover the care and the Mensa (#4686)", async () => {
    renderPage();
    await screen.findAllByText("100 %");
    saveCareCostShare.mockResolvedValue(data);
    fireEvent.click(
      screen.getAllByRole("button", { name: "Anteile: Lea Ohne" })[0]!,
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Anteil eintragen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Jugendamt" }));
    expect(
      within(dialog).getByRole("button", { name: "Betreuung", pressed: true }),
    ).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Betreuung und Mensa" }),
    );
    // A chosen scope stays when the kind changes afterwards.
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Bildung und Teilhabe" }),
    );
    fireEvent.click(
      within(dialog).getByRole("combobox", { name: /Kostenträger/ }),
    );
    fireEvent.click(
      await screen.findByRole("option", { name: "Jobcenter Enzkreis" }),
    );
    fireEvent.change(
      within(dialog).getByLabelText(/Bescheid/, { selector: "input" }),
      { target: { value: "BuT-Bescheid 2026-118" } },
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Anteil speichern" }),
    );
    await vi.waitFor(() =>
      expect(saveCareCostShare).toHaveBeenCalledWith(
        null,
        expect.objectContaining({
          kind: "education_participation",
          scope: "both",
        }),
      ),
    );
  });
});
