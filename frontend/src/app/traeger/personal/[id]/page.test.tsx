import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { StaffRecordDetail } from "~/lib/traeger-staff-api";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchStaffRecord: vi.fn(),
    updateStaffPersonnelNumber: vi.fn(),
    revealStaffFinancial: vi.fn(),
    updateStaffFinancial: vi.fn(),
    dissolveStaffRecord: vi.fn(),
  },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-staff-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-staff-api")>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useParams: () => ({ id: "5" }),
  usePathname: () => "/personal/5",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerStaffRecordPage from "./page";

const RECORD: StaffRecordDetail = {
  id: "5",
  firstName: "Mira",
  lastName: "Sommer-Berg",
  birthday: "1990-04-02",
  personnelNumber: "70001",
  hasFinancial: true,
  employments: [
    { schoolId: "1", schoolName: "OGS Nord", staffId: "11", active: true },
    { schoolId: "2", schoolName: "OGS Süd", staffId: "22", active: false },
  ],
  financial: { iban: "•••• 3000", taxId: null, socialSecurityNumber: null },
  changes: [
    {
      kind: "not_chosen",
      field: "last_name",
      value: "Sommer",
      schoolName: "OGS Nord",
      occurredAt: "2026-11-14T10:00:00Z",
    },
  ],
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerStaffRecordPage />
    </ToastProvider>,
  );
}

describe("TraegerStaffRecordPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchStaffRecord.mockResolvedValue(RECORD);
  });

  it("shows the Einsatzorte, the masked bank values and the trail", async () => {
    renderPage();
    expect(await screen.findByText("Einsatzorte")).toBeInTheDocument();
    expect(screen.getByText("Beendet")).toBeInTheDocument();
    expect(screen.getByText("•••• 3000")).toBeInTheDocument();
    expect(
      screen.getByText("Nachname „Sommer“ von OGS Nord nicht übernommen"),
    ).toBeInTheDocument();
  });

  it("saves the Träger's personnel number and refuses letters", async () => {
    api.updateStaffPersonnelNumber.mockResolvedValue({
      ...RECORD,
      personnelNumber: "70005",
    });
    renderPage();
    const card = (
      await screen.findByText("Personalnummer", {
        selector: "h2",
      })
    ).closest("section");
    expect(card).not.toBeNull();
    fireEvent.click(within(card!).getByRole("button", { name: /Ändern/ }));
    const input = within(card!).getByLabelText("Personalnummer");
    fireEvent.change(input, { target: { value: "70A" } });
    fireEvent.click(within(card!).getByRole("button", { name: "Speichern" }));
    expect(
      await within(card!).findByText("Die Personalnummer hat 1 bis 9 Ziffern."),
    ).toBeInTheDocument();
    expect(api.updateStaffPersonnelNumber).not.toHaveBeenCalled();

    fireEvent.change(input, { target: { value: "70005" } });
    fireEvent.click(within(card!).getByRole("button", { name: "Speichern" }));
    await vi.waitFor(() =>
      expect(api.updateStaffPersonnelNumber).toHaveBeenCalledWith("5", "70005"),
    );
    expect(await within(card!).findByText("70005")).toBeInTheDocument();
  });

  it("reveals the bank values on request", async () => {
    api.revealStaffFinancial.mockResolvedValue({
      iban: "DE89370400440532013000",
      taxId: null,
      socialSecurityNumber: null,
    });
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /Anzeigen/ }));
    expect(
      await screen.findByText("DE89370400440532013000"),
    ).toBeInTheDocument();
    expect(api.revealStaffFinancial).toHaveBeenCalledWith("5");
  });
});
