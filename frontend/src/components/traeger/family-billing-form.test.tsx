import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import {
  type FamilyBilling,
  familyBillingBody,
} from "~/lib/traeger-billing-terms-api";
import {
  EMPTY_FAMILY_BILLING,
  FamilyBillingForm,
  type FamilyBillingTarget,
  familyBillingErrors,
  familyBillingInput,
} from "./family-billing-form";

const { saveFamilyBilling } = vi.hoisted(() => ({
  saveFamilyBilling: vi.fn(),
}));
vi.mock("~/lib/traeger-billing-terms-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  saveFamilyBilling,
}));

const target: FamilyBillingTarget = {
  contractId: "42",
  applicantName: "Anna Muster",
  childName: "Mia Muster",
  issued: false,
};

function renderModal(existing: FamilyBilling | null = null) {
  const onSaved = vi.fn();
  render(
    <ToastProvider>
      <FamilyBillingForm
        target={target}
        existing={existing}
        onCancel={vi.fn()}
        onSaved={onSaved}
      />
    </ToastProvider>,
  );
  return { onSaved };
}

describe("FamilyBillingForm", () => {
  beforeEach(() => {
    saveFamilyBilling.mockReset();
  });

  it("marks the payer's missing fields and sends nothing", async () => {
    renderModal();
    fireEvent.click(screen.getByRole("radio", { name: "Andere Person" }));
    fireEvent.change(screen.getByLabelText(/E-Mail-Adresse/), {
      target: { value: "kein-at" },
    });
    fireEvent.change(screen.getByLabelText(/Postleitzahl/), {
      target: { value: "123" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte eine Postleitzahl mit 5 Ziffern eingeben."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/E-Mail-Adresse/)).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(saveFamilyBilling).not.toHaveBeenCalled();
  });

  it("saves both parents with an own payment method and due day", async () => {
    saveFamilyBilling.mockResolvedValue(null);
    const { onSaved } = renderModal();
    fireEvent.click(screen.getByRole("radio", { name: "Beide Eltern" }));
    fireEvent.change(screen.getByLabelText(/Vorname des zweiten Elternteils/), {
      target: { value: " Ben " },
    });
    fireEvent.change(
      screen.getByLabelText(/Nachname des zweiten Elternteils/),
      {
        target: { value: "Muster" },
      },
    );
    fireEvent.click(screen.getByRole("radio", { name: "Lastschrift" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(saveFamilyBilling).toHaveBeenCalledWith({
        contractId: "42",
        recipientKind: "both",
        secondFirstName: " Ben ",
        secondLastName: "Muster",
        payerFirstName: "",
        payerLastName: "",
        payerEmail: "",
        payerAddress: null,
        paymentMethod: "direct_debit",
        dueDay: null,
        debtorNumber: "",
      }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(null));
  });

  it("saves a debtor number of the family alone (#4693)", async () => {
    saveFamilyBilling.mockResolvedValue(null);
    renderModal();
    fireEvent.change(screen.getByLabelText(/Debitorennummer/), {
      target: { value: "10 4711" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(saveFamilyBilling).toHaveBeenCalledWith(
        expect.objectContaining({
          recipientKind: "applicant",
          debtorNumber: "10 4711",
        }),
      ),
    );
  });

  it("starts from the stored deviation", () => {
    renderModal({
      id: "5",
      familyEmail: "anna@example.org",
      contractId: "42",
      recipientKind: "payer",
      secondFirstName: "",
      secondLastName: "",
      payerFirstName: "Carla",
      payerLastName: "Amt",
      payerEmail: "amt@example.org",
      payerAddress: { street: "Weg 1", postalCode: "12345", city: "Ort" },
      paymentMethod: "",
      dueDay: 15,
      debtorNumber: "10 4711",
      updatedAt: "2026-09-01T10:00:00Z",
    });
    expect(screen.getByLabelText(/Vorname/)).toHaveValue("Carla");
    expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("10 4711");
    expect(screen.getByLabelText(/Postleitzahl/)).toHaveValue("12345");
  });
});

describe("FamilyBillingForm and the family's debtor number", () => {
  beforeEach(() => {
    saveFamilyBilling.mockReset();
  });

  const STORED: FamilyBilling = {
    id: "5",
    familyEmail: "anna@example.org",
    contractId: "42",
    recipientKind: "applicant",
    secondFirstName: "",
    secondLastName: "",
    payerFirstName: "",
    payerLastName: "",
    payerEmail: "",
    payerAddress: null,
    paymentMethod: "direct_debit",
    dueDay: null,
    debtorNumber: "40300",
    updatedAt: "2026-09-01T10:00:00Z",
  };

  // #6692: the number belongs to the family, not to the contract. Going back
  // to the contract must not drop a number the holiday care remembered.
  it("keeps the number on „Wieder wie im Vertrag“", async () => {
    saveFamilyBilling.mockResolvedValue(STORED);
    renderModal(STORED);

    fireEvent.click(
      screen.getByRole("button", { name: "Felder wie im Vertrag ausfüllen" }),
    );
    expect(screen.getByLabelText(/Debitorennummer/)).toHaveValue("40300");
    expect(
      screen.queryByRole("button", { name: "Felder wie im Vertrag ausfüllen" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(saveFamilyBilling).toHaveBeenCalledWith(
        expect.objectContaining({
          recipientKind: "applicant",
          paymentMethod: "",
          debtorNumber: "40300",
        }),
      ),
    );
  });

  it("saves unrelated edits with an unchanged legacy number after the format tightens", async () => {
    saveFamilyBilling.mockResolvedValue(STORED);
    render(
      <ToastProvider>
        <FamilyBillingForm
          target={target}
          existing={{ ...STORED, debtorNumber: "LEGACY 1" }}
          format={{ digitsOnly: true, minDigits: 5, maxDigits: 5 }}
          onCancel={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("radio", { name: "Überweisung" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(saveFamilyBilling).toHaveBeenCalledWith(
        expect.objectContaining({
          paymentMethod: "transfer",
          debtorNumber: "LEGACY 1",
        }),
      ),
    );
    saveFamilyBilling.mockClear();
    fireEvent.change(screen.getByLabelText(/Debitorennummer/), {
      target: { value: "LEGACY 2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(
        "Die Debitorennummer hat 5 Ziffern, zum Beispiel 20815.",
      ),
    ).toBeInTheDocument();
    expect(saveFamilyBilling).not.toHaveBeenCalled();
  });

  it("refuses a number in another format of the Träger (#6675)", async () => {
    render(
      <ToastProvider>
        <FamilyBillingForm
          target={target}
          existing={null}
          format={{ digitsOnly: true, minDigits: 5, maxDigits: 5 }}
          onCancel={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    const field = screen.getByLabelText(/Debitorennummer/);
    expect(field).toHaveAttribute("inputmode", "numeric");
    fireEvent.change(field, { target: { value: "40 300" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Debitorennummer hat 5 Ziffern, zum Beispiel 20815.",
      ),
    ).toBeInTheDocument();
    expect(saveFamilyBilling).not.toHaveBeenCalled();
  });
});

describe("family billing request", () => {
  it("accepts the contract party without further fields", () => {
    expect(familyBillingErrors(EMPTY_FAMILY_BILLING)).toEqual({});
  });

  it("sends only the fields of the chosen recipient, trimmed", () => {
    const draft = {
      ...EMPTY_FAMILY_BILLING,
      recipientKind: "payer" as const,
      secondFirstName: "Stale",
      payerFirstName: " Carla ",
      payerLastName: "Amt",
      payerEmail: " amt@example.org ",
      street: " Weg 1 ",
      postalCode: "12345",
      city: "Ort",
      dueDay: "15",
      debtorNumber: " 10 4711 ",
    };
    expect(familyBillingErrors(draft)).toEqual({});
    expect(familyBillingBody(familyBillingInput("42", draft))).toEqual({
      contract_id: 42,
      recipient_kind: "payer",
      second_first_name: "",
      second_last_name: "",
      payer_first_name: "Carla",
      payer_last_name: "Amt",
      payer_email: "amt@example.org",
      payer_address: { street: "Weg 1", postal_code: "12345", city: "Ort" },
      payment_method: "",
      due_day: 15,
      debtor_number: "10 4711",
    });
  });
});
