import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import type { GuardianWithRelationship } from "@/lib/guardian-helpers";
import { StudentPaymentCard } from "./student-payment-card";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const mockFetchPayment = vi.fn();
const mockRevealPayment = vi.fn();
const mockUpdatePayment = vi.fn();
const mockSetPayer = vi.fn();

vi.mock("~/lib/guardian-payment-api", () => ({
  fetchGuardianPayment: (id: string) => mockFetchPayment(id),
  revealGuardianPayment: (id: string) => mockRevealPayment(id),
  updateGuardianPayment: (id: string, input: unknown) =>
    mockUpdatePayment(id, input),
  setStudentPayer: (studentId: string, guardianId: string | null) =>
    mockSetPayer(studentId, guardianId),
}));

const { mockToastError, mockToastSuccess, mockShowActionError } = vi.hoisted(
  () => ({
    mockToastError: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockShowActionError: vi.fn(),
  }),
);
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useApiErrorDisplay: () => ({ show: mockShowActionError }),
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

const FULL_IBAN = "DE89370400440532013000";

function guardian(
  id: string,
  firstName: string,
  isPayer: boolean,
): GuardianWithRelationship {
  return {
    id,
    firstName,
    lastName: "Schneider",
    phoneNumbers: [],
    preferredContactMethod: "email",
    languagePreference: "de",
    hasAccount: false,
    relationshipId: `rel-${id}`,
    relationshipType: "parent",
    isPrimary: true,
    isEmergencyContact: false,
    canPickup: true,
    isPayer,
    emergencyPriority: 1,
  } as GuardianWithRelationship;
}

describe("StudentPaymentCard", () => {
  beforeEach(() => {
    mockFetchPayment.mockReset();
    mockRevealPayment.mockReset();
    mockUpdatePayment.mockReset();
    mockSetPayer.mockReset();
    mockToastError.mockReset();
    mockToastSuccess.mockReset();
    mockShowActionError.mockReset();
    mockFetchPayment.mockResolvedValue({
      guardianId: "10",
      ibanMasked: "•••• 3000",
      accountHolder: null,
    });
  });

  it("shows only the masked IBAN until Anzeigen is pressed", async () => {
    mockRevealPayment.mockResolvedValue({
      guardianId: "10",
      iban: FULL_IBAN,
      accountHolder: null,
    });

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", true)]}
      />,
    );

    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalledWith("10"));
    expect(screen.getByText("•••• 3000")).toBeInTheDocument();
    expect(screen.queryByText(FULL_IBAN)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Anzeigen/ }));

    await waitFor(() =>
      expect(screen.getByText(FULL_IBAN)).toBeInTheDocument(),
    );
    expect(mockRevealPayment).toHaveBeenCalledWith("10");
  });

  // Guards the failure mode of masked edit forms: prefilling from the display
  // value would save the dots back over the stored IBAN.
  it("prefills the edit form from the real value, never from the mask", async () => {
    mockRevealPayment.mockResolvedValue({
      guardianId: "10",
      iban: FULL_IBAN,
      accountHolder: null,
    });
    mockUpdatePayment.mockResolvedValue(undefined);

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", true)]}
      />,
    );
    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    const input = await screen.findByLabelText("IBAN");
    expect(input).toHaveValue(FULL_IBAN);

    // Unverändert gibt es nichts zu speichern.
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Anderer Kontoinhaber"), {
      target: { value: "Sabine Schneider-Kern" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockUpdatePayment).toHaveBeenCalledWith("10", {
        iban: FULL_IBAN,
        accountHolder: "Sabine Schneider-Kern",
      }),
    );
    expect(mockSetPayer).not.toHaveBeenCalled();
  });

  // Bauart 2 Regel 5 (#3113): der Fehler steht als Alert oben in der Karte,
  // das Formular bleibt mit der Eingabe offen; kein Toast.
  it("keeps the edit form open and shows a failed save as an alert", async () => {
    mockRevealPayment.mockResolvedValue({
      guardianId: "10",
      iban: FULL_IBAN,
      accountHolder: null,
    });
    mockUpdatePayment.mockRejectedValue(
      new ApiError("malformed IBAN", 400, {
        code: "general.input",
        errors: [{ field: "iban", reason: "malformed" }],
      }),
    );

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", true)]}
      />,
    );
    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await screen.findByLabelText("IBAN");
    fireEvent.change(screen.getByLabelText("Anderer Kontoinhaber"), {
      target: { value: "Sabine Schneider-Kern" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    // #2517: catalog text in the edit area, the named field marked.
    expect(
      await screen.findByText(
        catalogText("general.input", "das Speichern der Bankverbindung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/malformed/)).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText("IBAN")).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
    expect(screen.getByLabelText("IBAN")).toHaveValue(FULL_IBAN);
    expect(screen.getByLabelText("Anderer Kontoinhaber")).toHaveValue(
      "Sabine Schneider-Kern",
    );
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("assigns the payer and lets the parent refetch", async () => {
    mockSetPayer.mockResolvedValue(undefined);
    const onChanged = vi.fn();

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[
          guardian("10", "Sabine", false),
          guardian("11", "Klaus", false),
        ]}
        onChanged={onChanged}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(
      await screen.findByRole("option", { name: "Klaus Schneider" }),
    );

    // Die Auswahl allein schreibt nichts (Bauart 2, Regel 4).
    expect(mockSetPayer).not.toHaveBeenCalled();
    expect(onChanged).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockSetPayer).toHaveBeenCalledWith("7", "11"));
    expect(onChanged).toHaveBeenCalled();
  });

  it("leaves edit mode when the payer changes outside the card", async () => {
    mockRevealPayment.mockResolvedValue({
      guardianId: "10",
      iban: FULL_IBAN,
      accountHolder: null,
    });

    const { rerender } = render(
      <StudentPaymentCard
        studentId="7"
        guardians={[
          guardian("10", "Sabine", true),
          guardian("11", "Klaus", false),
        ]}
      />,
    );
    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalledWith("10"));

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(await screen.findByLabelText("IBAN")).toBeInTheDocument();

    rerender(
      <StudentPaymentCard
        studentId="7"
        guardians={[
          guardian("10", "Sabine", false),
          guardian("11", "Klaus", true),
        ]}
      />,
    );

    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalledWith("11"));
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bearbeiten" })).toBeEnabled();
    expect(mockSetPayer).not.toHaveBeenCalled();
  });

  it("hides the IBAN fields while the payer is being changed and names the next step", async () => {
    mockRevealPayment.mockResolvedValue({
      guardianId: "10",
      iban: FULL_IBAN,
      accountHolder: null,
    });

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[
          guardian("10", "Sabine", true),
          guardian("11", "Klaus", false),
        ]}
      />,
    );
    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(await screen.findByLabelText("IBAN")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(
      await screen.findByRole("option", { name: "Klaus Schneider" }),
    );

    expect(screen.queryByLabelText("IBAN")).not.toBeInTheDocument();
    expect(
      screen.getByText(/Die Bankverbindung von Klaus Schneider/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(mockSetPayer).not.toHaveBeenCalled();
    expect(mockUpdatePayment).not.toHaveBeenCalled();
    expect(screen.getAllByText("Sabine Schneider").length).toBeGreaterThan(0);
  });

  it("shows a save failure as an Alert inside the edit area", async () => {
    mockSetPayer.mockRejectedValue(
      new ApiError("payer exploded", 500, { code: "general.server" }),
    );

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("11", "Klaus", false)]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(
      await screen.findByRole("option", { name: "Klaus Schneider" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("general.server", "das Speichern des Zahlungskontos"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/payer exploded/)).not.toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();
    // Der Entwurf bleibt offen.
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("names the consequence when no payer is assigned", () => {
    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", false)]}
      />,
    );

    expect(
      screen.getByText(/noch niemand als Zahlungskonto eingetragen/),
    ).toBeInTheDocument();
    expect(mockFetchPayment).not.toHaveBeenCalled();
  });

  it("states the precondition when the child has no guardians yet", () => {
    render(<StudentPaymentCard studentId="7" guardians={[]} />);

    expect(
      screen.getByText(/noch keine erziehungsberechtigte Person/),
    ).toBeInTheDocument();
  });

  it("hides the edit action in read-only mode but keeps Anzeigen", async () => {
    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", true)]}
        readOnly
      />,
    );
    await waitFor(() => expect(mockFetchPayment).toHaveBeenCalled());

    expect(
      screen.getByRole("button", { name: /Anzeigen/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  // #2517: a failed load says so in the card, never "Noch keine IBAN".
  it("shows a failed load in the card and reloads on retry", async () => {
    mockFetchPayment.mockReset();
    mockFetchPayment
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({
        guardianId: "10",
        ibanMasked: "•••• 3000",
        accountHolder: null,
      });

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", true)]}
      />,
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Bankverbindung"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine IBAN gespeichert."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("•••• 3000")).toBeInTheDocument();
    expect(
      screen.queryByText(
        catalogText("general.unavailable", "die Bankverbindung"),
      ),
    ).not.toBeInTheDocument();
  });

  it("reports a failed Anzeigen on the action path", async () => {
    const failure = new ApiError("nope", 403, { code: "general.permission" });
    mockRevealPayment.mockRejectedValue(failure);

    render(
      <StudentPaymentCard
        studentId="7"
        guardians={[guardian("10", "Sabine", true)]}
      />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Anzeigen" }));

    await waitFor(() =>
      expect(mockShowActionError).toHaveBeenCalledWith(failure, {
        object: "das Anzeigen der IBAN",
        retry: expect.any(Function),
      }),
    );
  });
});
