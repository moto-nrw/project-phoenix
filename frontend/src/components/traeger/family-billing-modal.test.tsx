import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import {
  type FamilyBilling,
  familyBillingBody,
} from "~/lib/traeger-billing-terms-api";
import {
  EMPTY_FAMILY_BILLING,
  FamilyBillingModal,
  type FamilyBillingTarget,
  familyBillingErrors,
  familyBillingInput,
} from "./family-billing-modal";

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
      <FamilyBillingModal
        target={target}
        existing={existing}
        onClose={vi.fn()}
        onSaved={onSaved}
      />
    </ToastProvider>,
  );
  return { onSaved };
}

describe("FamilyBillingModal", () => {
  beforeEach(() => {
    saveFamilyBilling.mockReset();
  });

  it("marks the payer's missing fields and sends nothing", async () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Andere Person" }));
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
    fireEvent.click(screen.getByRole("button", { name: "Beide Eltern" }));
    fireEvent.change(screen.getByLabelText(/Vorname des zweiten Elternteils/), {
      target: { value: " Ben " },
    });
    fireEvent.change(
      screen.getByLabelText(/Nachname des zweiten Elternteils/),
      {
        target: { value: "Muster" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Lastschrift" }));
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
      }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(null));
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
      updatedAt: "2026-09-01T10:00:00Z",
    });
    expect(screen.getByLabelText(/Vorname/)).toHaveValue("Carla");
    expect(screen.getByLabelText(/Postleitzahl/)).toHaveValue("12345");
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
    });
  });
});
