import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  SepaMandateText,
  SepaMandateView,
} from "~/lib/parent-sepa-mandate-api";
import { ParentSepaMandateEntry } from "./parent-sepa-mandate-entry";
import { ParentSepaMandateView } from "./parent-sepa-mandate-view";

const mocks = vi.hoisted(() => ({
  fetch: vi.fn(),
  preview: vi.fn(),
  grant: vi.fn(),
  revoke: vi.fn(),
}));
vi.mock("~/lib/parent-sepa-mandate-api", () => ({
  fetchSepaMandate: mocks.fetch,
  previewSepaMandate: mocks.preview,
  grantSepaMandate: mocks.grant,
  revokeSepaMandate: mocks.revoke,
}));

const IBAN = "DE89370400440532013000";

const open: SepaMandateView = {
  enabled: true,
  canManage: true,
  creditorMissing: false,
  creditor: {
    name: "Jugendring Muster",
    address: "Hauptstraße 1, 75175 Pforzheim",
    creditorId: "DE98ZZZ09999999999",
  },
  mandate: null,
  prefill: {
    accountHolder: "Eva Muster",
    street: "",
    postalCode: "",
    city: "",
  },
};

const text: SepaMandateText = {
  version: "sepa-core-de-1",
  reference: "MOTO-75-20260909-1A2B",
  text: "SEPA-Lastschriftmandat\n\nMandatsreferenz: MOTO-75-20260909-1A2B",
  textSha256: "a".repeat(64),
  accountToken: "sealed-preview-account",
};

const granted: SepaMandateView = {
  ...open,
  prefill: null,
  mandate: {
    collectionEligible: true,
    id: "9",
    reference: text.reference,
    signedOn: "2026-09-09",
    accountHolder: "Eva Muster",
    ibanMasked: "DE•• •••• •••• •••• ••30 00",
    bic: "",
    source: "parent",
    creditorId: "DE98ZZZ09999999999",
    revokedOn: null,
    proof: {
      channel: "parent_account",
      version: text.version,
      textSha256: text.textSha256,
      text: text.text,
      confirmedAt: "2026-09-09T10:00:00Z",
    },
  },
};

function renderView() {
  return render(<ParentSepaMandateView organizationId="4" contractId="75" />, {
    wrapper: ToastProvider,
  });
}

async function fillAccount() {
  fireEvent.change(await screen.findByLabelText(/^IBAN/), {
    target: { value: IBAN },
  });
  fireEvent.change(screen.getByLabelText(/Straße und Hausnummer/), {
    target: { value: "Gartenweg 2" },
  });
  fireEvent.change(screen.getByLabelText(/^PLZ/), {
    target: { value: "75175" },
  });
  fireEvent.change(screen.getByLabelText(/^Ort/), {
    target: { value: "Pforzheim" },
  });
}

describe("ParentSepaMandateView", () => {
  beforeEach(() => vi.clearAllMocks());

  it("only points to paper while the Träger has the switch off", async () => {
    mocks.fetch.mockResolvedValue({ ...open, enabled: false });
    renderView();
    expect(
      await screen.findByText(/Fragen Sie den Träger nach einem Formular/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weiter" }),
    ).not.toBeInTheDocument();
    expect(mocks.fetch).toHaveBeenCalledWith("4", "75");
  });

  it("checks the account before it asks for the text", async () => {
    mocks.fetch.mockResolvedValue(open);
    renderView();
    fireEvent.change(await screen.findByLabelText(/^IBAN/), {
      target: { value: "DE00123" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByText(
        "Diese IBAN stimmt nicht. Bitte prüfen Sie jede Ziffer.",
      ),
    ).toBeInTheDocument();
    expect(mocks.preview).not.toHaveBeenCalled();
  });

  it("gives exactly the confirmed text, and only as account holder", async () => {
    mocks.fetch.mockResolvedValue(open);
    mocks.preview.mockResolvedValue(text);
    mocks.grant.mockResolvedValue(granted);
    renderView();
    await fillAccount();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    const grant = await screen.findByRole("button", {
      name: "Mandat erteilen",
    });
    expect(screen.getByText(/Mandatsreferenz: MOTO-75/)).toBeInTheDocument();

    fireEvent.click(grant);
    const form = grant.closest("form") as HTMLFormElement;
    expect(await within(form).findByRole("alert")).toBeInTheDocument();
    expect(mocks.grant).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ich bin Inhaberin oder Inhaber dieses Kontos/,
      }),
    );
    fireEvent.click(grant);
    expect(await screen.findByText("Ihr Mandat")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Sie erlauben Ihrem Träger, Beiträge von diesem Konto abzubuchen. Wie Sie bezahlen, steht auf Ihrer Rechnung.",
      ),
    ).toBeInTheDocument();
    expect(mocks.grant).toHaveBeenCalledWith(
      "4",
      "75",
      expect.objectContaining({
        accountHolder: "Eva Muster",
        city: "Pforzheim",
      }),
      expect.objectContaining({
        reference: text.reference,
        textSha256: text.textSha256,
        accountToken: text.accountToken,
      }),
    );
    expect(screen.queryByText(IBAN)).not.toBeInTheDocument();
    expect(screen.getByText("DE•• •••• •••• •••• ••30 00")).toBeInTheDocument();
  });

  it("shows the new text when the old one no longer holds", async () => {
    mocks.fetch.mockResolvedValue(open);
    mocks.preview.mockResolvedValueOnce(text).mockResolvedValueOnce({
      ...text,
      text: "Neuer Text",
      textSha256: "b".repeat(64),
    });
    mocks.grant.mockRejectedValue(
      Object.assign(new ApiError("changed", 409), {
        code: "parent_billing.sepa_mandate_text_changed",
      }),
    );
    renderView();
    await fillAccount();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    fireEvent.click(
      await screen.findByRole("checkbox", {
        name: /Ich bin Inhaberin oder Inhaber dieses Kontos/,
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Mandat erteilen" }));
    expect(await screen.findByText("Neuer Text")).toBeInTheDocument();
    expect(screen.getByText(/Der Text hat sich geändert/)).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", {
        name: /Ich bin Inhaberin oder Inhaber dieses Kontos/,
      }),
    ).not.toBeChecked();
  });

  it("revokes the mandate after a confirmation", async () => {
    mocks.fetch.mockResolvedValueOnce(granted).mockResolvedValueOnce(open);
    mocks.revoke.mockResolvedValue(granted);
    renderView();
    fireEvent.click(
      await screen.findByRole("button", { name: "Mandat widerrufen" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Ab der nächsten Rechnung/),
    ).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Widerrufen" }));
    expect(
      await screen.findByRole("button", { name: "Weiter" }),
    ).toBeInTheDocument();
    expect(mocks.revoke).toHaveBeenCalledWith("4", "75");
  });
});

describe("Mandate collection status", () => {
  beforeEach(() => vi.clearAllMocks());

  it("keeps proof and revocation available when a replacement is needed", async () => {
    mocks.fetch.mockResolvedValue({
      ...granted,
      mandate: { ...granted.mandate, collectionEligible: false },
    });
    renderView();
    expect(await screen.findByText("Neues Mandat nötig")).toBeInTheDocument();
    expect(screen.queryByText(/Gültig seit/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Ihr Träger bucht die Beiträge von diesem Konto ab."),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Mandat widerrufen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Als Datei speichern" }),
    ).toBeInTheDocument();
  });

  it("uses only the inbox status for links and eligible badges", async () => {
    mocks.fetch.mockResolvedValue(granted);
    const { rerender } = render(
      <ParentSepaMandateEntry
        organizationId="4"
        contractId="75"
        status={{ enabled: true, collection_eligible: true }}
      />,
    );
    expect(
      await screen.findByRole("link", { name: "Lastschrift" }),
    ).toHaveAttribute("href", expect.stringContaining("/4/75?lastschrift=1"));
    expect(screen.getByText("Lastschrift erteilt")).toBeInTheDocument();
    expect(mocks.fetch).not.toHaveBeenCalled();
    rerender(
      <ParentSepaMandateEntry
        organizationId="4"
        contractId="75"
        status={{ enabled: true, collection_eligible: false }}
      />,
    );
    expect(screen.queryByText("Lastschrift erteilt")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Lastschrift" }),
    ).toBeInTheDocument();
    rerender(
      <ParentSepaMandateEntry
        organizationId="4"
        contractId="75"
        status={{ enabled: false, collection_eligible: false }}
      />,
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
