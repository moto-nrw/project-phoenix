import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { SepaMandate } from "~/lib/traeger-sepa-api";
import { traegerRequest } from "~/lib/traeger-api";
import { SepaMandatesCard } from "./sepa-mandates-card";

const api = vi.hoisted(() => ({
  fetchSepaMandates: vi.fn(),
  fetchSepaMandateHistory: vi.fn(),
  createSepaMandate: vi.fn(),
  revokeSepaMandate: vi.fn(),
}));
vi.mock("~/lib/traeger-sepa-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));
vi.mock("~/lib/traeger-api", () => ({
  traegerRequest: vi.fn(),
  fetchTraegerBuero: () => Promise.resolve({ members: [] }),
}));

const MANDATE: SepaMandate = {
  id: "9",
  contractId: "42",
  familyEmail: "eva@example.org",
  partyFirstName: "Eva",
  partyLastName: "Beispiel",
  reference: "SJR-2026-0001",
  signedOn: "2026-09-01",
  kind: "recurring",
  sequence: "recurring",
  creditorId: "DE98ZZZ09999999999",
  creditorCurrent: true,
  accountHolder: "Eva Beispiel",
  ibanMasked: "DE•• •••• •••• •••• ••30 00",
  bic: "",
  source: "office",
  revokedOn: null,
  lastCollectedOn: null,
  expiresOn: "2029-09-01",
  createdAt: "2026-09-01T10:00:00+02:00",
};

function renderCard() {
  render(
    <ToastProvider>
      <SepaMandatesCard contractId="42" holder="Eva Beispiel" />
    </ToastProvider>,
  );
}

describe("SepaMandatesCard (#4695)", () => {
  beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset();
  });

  it("offers a new mandate while none is in force and sends the compacted IBAN", async () => {
    api.fetchSepaMandates.mockResolvedValue([]);
    api.createSepaMandate.mockResolvedValue(MANDATE);
    renderCard();
    expect(await screen.findByText("Kein gültiges Mandat")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Mandat erfassen" }));
    fireEvent.change(screen.getByLabelText(/IBAN/), {
      target: { value: "de89 3704 0044 0532 0130 01" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(/Diese IBAN stimmt nicht/),
    ).toBeInTheDocument();
    expect(api.createSepaMandate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/IBAN/), {
      target: { value: "de89 3704 0044 0532 0130 00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(/Datum der Unterschrift wählen/),
    ).toBeInTheDocument();
  });

  it("shows the mask only and no way to a second mandate while one is in force", async () => {
    api.fetchSepaMandates.mockResolvedValue([MANDATE]);
    renderCard();
    expect(
      (await screen.findAllByText("DE•• •••• •••• •••• ••30 00")).length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("Gültig").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Folgelastschrift").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Mandat erfassen" }),
    ).not.toBeInTheDocument();
  });

  it("asks for a new mandate once the creditor identifier changed", async () => {
    api.fetchSepaMandates.mockResolvedValue([
      { ...MANDATE, creditorCurrent: false },
    ]);
    renderCard();
    expect(
      (await screen.findAllByText("Neues Mandat nötig")).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText(/Gilt nicht mehr/)).toBeInTheDocument();
    expect(screen.queryByText("Gültig")).not.toBeInTheDocument();
  });

  it("maps and shows the imported file and line in the mandate history", async () => {
    const original = await vi.importActual<
      typeof import("~/lib/traeger-sepa-api")
    >("~/lib/traeger-sepa-api");
    api.fetchSepaMandates.mockResolvedValue([{ ...MANDATE, source: "import" }]);
    api.fetchSepaMandateHistory.mockImplementation(
      original.fetchSepaMandateHistory,
    );
    vi.mocked(traegerRequest).mockResolvedValue([
      {
        id: 1,
        action: "create",
        source: "import",
        fields: [],
        account_id: 5,
        occurred_at: "2026-09-01T10:00:00+02:00",
        import_file: "ogs-connect.csv",
        import_line: 4,
      },
      {
        id: 2,
        action: "read",
        source: "office",
        fields: [],
        account_id: null,
        occurred_at: "2026-09-01T11:00:00+02:00",
      },
    ]);
    renderCard();
    fireEvent.click(
      await screen.findByRole("button", {
        name: `Aktionen für das Mandat ${MANDATE.reference}`,
      }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Verlauf ansehen" }));
    expect(
      await screen.findByText("Datei: ogs-connect.csv · Zeile 4"),
    ).toBeInTheDocument();
    expect(traegerRequest).toHaveBeenCalledWith(
      "/api/traeger/sepa-mandates/9/history",
    );
    expect(screen.getAllByText(/^Datei:/)).toHaveLength(1);
  });

  it("stays away for a role without the right „Abrechnung“", async () => {
    let refuse: (error: unknown) => void = () => undefined;
    api.fetchSepaMandates.mockReturnValue(
      new Promise((_, reject) => {
        refuse = reject;
      }),
    );
    renderCard();
    // Anker: while it loads, the card shows its title.
    expect(
      await screen.findByText("SEPA-Lastschriftmandat"),
    ).toBeInTheDocument();
    refuse(new ApiError("forbidden", 403));
    await waitFor(() =>
      expect(
        screen.queryByText("SEPA-Lastschriftmandat"),
      ).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
