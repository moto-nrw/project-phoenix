import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import type { GuardianWithRelationship } from "@/lib/guardian-helpers";
import { StudentPaymentCard } from "./student-payment-card";

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

const mockToastError = vi.fn();
const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
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

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockUpdatePayment).toHaveBeenCalledWith("10", {
        iban: FULL_IBAN,
        accountHolder: null,
      }),
    );
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

    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(
      await screen.findByRole("option", { name: "Klaus Schneider" }),
    );

    await waitFor(() => expect(mockSetPayer).toHaveBeenCalledWith("7", "11"));
    expect(onChanged).toHaveBeenCalled();
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
});
