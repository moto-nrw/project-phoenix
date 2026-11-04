import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render as renderPlain,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { OperatorInvitationsData } from "~/lib/operator/operator-invitation-api";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const {
  mockUseSWR,
  mockMutate,
  mockListInvitations,
  mockCreateInvitation,
  mockResendInvitation,
  mockRevokeInvitation,
  mockDeactivateOperator,
  mockReactivateOperator,
} = vi.hoisted(() => ({
  mockUseSWR: vi.fn(),
  mockMutate: vi.fn(),
  mockListInvitations: vi.fn(),
  mockCreateInvitation: vi.fn(),
  mockResendInvitation: vi.fn(),
  mockRevokeInvitation: vi.fn(),
  mockDeactivateOperator: vi.fn(),
  mockReactivateOperator: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mockUseSWR,
}));

vi.mock("~/lib/operator/operator-invitation-api", () => ({
  listOperatorInvitations: mockListInvitations,
  createOperatorInvitation: mockCreateInvitation,
  resendOperatorInvitation: mockResendInvitation,
  revokeOperatorInvitation: mockRevokeInvitation,
  deactivateOperator: mockDeactivateOperator,
  reactivateOperator: mockReactivateOperator,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
    children,
    isConfirmLoading = false,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    children: React.ReactNode;
    isConfirmLoading?: boolean;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        {children}
        <button type="button" onClick={onConfirm} disabled={isConfirmLoading}>
          Confirm
        </button>
        <button type="button" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

import OperatorOperatorsPage from "./page";

const sampleData: OperatorInvitationsData = {
  invitations: [
    {
      id: "1",
      email: "pending@example.com",
      displayName: "Pending Op",
      createdBy: "2",
      creatorName: "Admin",
      expiresAt: "2026-04-06T12:00:00Z",
      emailSentAt: "2026-04-04T12:00:00Z",
      emailRetryCount: 0,
      createdAt: "2026-04-04T12:00:00Z",
    },
  ],
  operators: [
    {
      id: "2",
      email: "admin@example.com",
      displayName: "Admin User",
      active: true,
      lastLogin: "2026-04-03T10:00:00Z",
      createdAt: "2026-01-01T00:00:00Z",
      isSelf: true,
    },
    {
      id: "3",
      email: "inactive@example.com",
      displayName: "Inactive User",
      active: false,
      deactivatedAt: "2026-03-01T10:00:00Z",
      deletionDueAt: "2027-12-31T23:00:00Z",
      createdAt: "2026-01-01T00:00:00Z",
      isSelf: false,
    },
    {
      id: "4",
      email: "colleague@example.com",
      displayName: "Colleague User",
      active: true,
      lastLogin: "2026-04-02T10:00:00Z",
      createdAt: "2026-01-01T00:00:00Z",
      isSelf: false,
    },
  ],
};

function setupSWR(
  data: OperatorInvitationsData | undefined,
  options?: { isLoading?: boolean; error?: Error },
) {
  mockUseSWR.mockReturnValue({
    data,
    error: options?.error ?? undefined,
    isLoading: options?.isLoading ?? false,
    isValidating: false,
    mutate: mockMutate,
  });
}

function openActions(displayName: string) {
  fireEvent.click(
    screen.getByRole("button", { name: `Aktionen für ${displayName}` }),
  );
}

describe("OperatorOperatorsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders page title and description", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("Operatoren")).toBeInTheDocument();
    expect(
      screen.getByText("Neue Operatoren einladen und bestehende anzeigen."),
    ).toBeInTheDocument();
  });

  it("shows loading skeleton when loading", () => {
    setupSWR(undefined, { isLoading: true });
    render(<OperatorOperatorsPage />);

    expect(
      screen.getByLabelText("Operatoren werden geladen"),
    ).toBeInTheDocument();
  });

  it("displays pending invitations and operators", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("pending@example.com")).toBeInTheDocument();
    expect(screen.getByText("Admin User")).toBeInTheDocument();
    expect(screen.getByText("admin@example.com")).toBeInTheDocument();
    expect(screen.getByText("Inactive User")).toBeInTheDocument();
  });

  it("shows active/inactive status badges", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getAllByText("Aktiv")).toHaveLength(2);
    expect(screen.getByText("Inaktiv")).toBeInTheDocument();
  });

  it("shows 'Noch nicht angemeldet' for operators without last login", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("Noch nicht angemeldet")).toBeInTheDocument();
  });

  it("shows empty state when no operators exist", () => {
    setupSWR({ invitations: [], operators: [] });
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("Keine Operatoren vorhanden.")).toBeInTheDocument();
  });

  it("hides pending invitations section when empty", () => {
    setupSWR({ invitations: [], operators: sampleData.operators });
    render(<OperatorOperatorsPage />);

    // Anker: die Operatorenliste ist geladen.
    expect(screen.getByText("Admin User")).toBeInTheDocument();
    expect(screen.queryByText("Offene Einladungen")).not.toBeInTheDocument();
  });

  it("shows pending invitations section when invitations exist", () => {
    setupSWR({
      invitations: sampleData.invitations,
      operators: sampleData.operators,
    });
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("Admin User")).toBeInTheDocument();
    expect(screen.getByText("Offene Einladungen")).toBeInTheDocument();
  });

  it("submits invite form and shows toast", async () => {
    setupSWR(sampleData);
    mockCreateInvitation.mockResolvedValue(undefined);

    render(<OperatorOperatorsPage />);

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "new@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Anzeigename (optional)"), {
      target: { value: "New Op" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Einladung senden" }));

    await waitFor(() => {
      expect(mockCreateInvitation).toHaveBeenCalledWith({
        email: "new@example.com",
        displayName: "New Op",
      });
    });
    expect(
      await screen.findByText("Einladung wurde gesendet."),
    ).toBeInTheDocument();
  });

  it("sends undefined displayName when field is empty", async () => {
    setupSWR(sampleData);
    mockCreateInvitation.mockResolvedValue(undefined);

    render(<OperatorOperatorsPage />);

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "new@example.com" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Einladung senden" }));

    await waitFor(() => {
      expect(mockCreateInvitation).toHaveBeenCalledWith({
        email: "new@example.com",
        displayName: undefined,
      });
    });
  });

  // #2519: catalog text by code in the form's alert, field marked; never
  // the backend sentence.
  it("shows a failed invite in the form and marks the field", async () => {
    setupSWR(sampleData);
    mockCreateInvitation.mockRejectedValue(
      new ApiError("email already exists", 409, {
        code: "general.business_rejection",
        errors: [{ field: "email", reason: "taken" }],
      }),
    );

    render(<OperatorOperatorsPage />);

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "fail@example.com" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Einladung senden" }));

    expect(
      await screen.findByText(
        catalogText("general.business_rejection", "die Einladung"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("E-Mail-Adresse")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.queryByText("email already exists")).toBeNull();
  });

  it("shows a failed resend as a toast with retry", async () => {
    setupSWR(sampleData);
    mockResendInvitation
      .mockRejectedValueOnce(new ApiError("down", 503))
      .mockResolvedValueOnce(undefined);

    render(<OperatorOperatorsPage />);
    fireEvent.click(screen.getByText("Erneut senden"));

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "das erneute Senden der Einladung"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockResendInvitation).toHaveBeenCalledTimes(2));
  });

  it("keeps a failed revoke inside the confirmation", async () => {
    setupSWR(sampleData);
    mockRevokeInvitation.mockRejectedValue(
      new ApiError("gone", 404, { code: "general.input" }),
    );

    render(<OperatorOperatorsPage />);
    fireEvent.click(screen.getByText("Widerrufen"));
    fireEvent.click(screen.getByText("Confirm"));

    const modal = screen.getByTestId("confirmation-modal");
    await waitFor(() =>
      expect(modal.textContent).toContain(
        catalogText("general.input", "das Widerrufen der Einladung"),
      ),
    );
  });

  it("shows resend and revoke buttons for pending invitations", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("Erneut senden")).toBeInTheDocument();
    expect(screen.getByText("Widerrufen")).toBeInTheDocument();
  });

  it("handles resend action", async () => {
    setupSWR(sampleData);
    mockResendInvitation.mockResolvedValue(undefined);

    render(<OperatorOperatorsPage />);

    fireEvent.click(screen.getByText("Erneut senden"));

    await waitFor(() => {
      expect(mockResendInvitation).toHaveBeenCalledWith("1");
    });
    expect(
      await screen.findByText("Einladung wurde erneut gesendet."),
    ).toBeInTheDocument();
  });

  it("opens confirmation modal for revoke", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    fireEvent.click(screen.getByText("Widerrufen"));

    const modal = screen.getByTestId("confirmation-modal");
    expect(modal).toBeInTheDocument();
    expect(modal.textContent).toContain("pending@example.com");
  });

  it("handles revoke confirmation", async () => {
    setupSWR(sampleData);
    mockRevokeInvitation.mockResolvedValue(undefined);

    render(<OperatorOperatorsPage />);

    fireEvent.click(screen.getByText("Widerrufen"));
    fireEvent.click(screen.getByText("Confirm"));

    await waitFor(() => {
      expect(mockRevokeInvitation).toHaveBeenCalledWith("1");
    });
    expect(
      await screen.findByText("Einladung wurde widerrufen."),
    ).toBeInTheDocument();
  });

  it("shows a failed load with retry", async () => {
    setupSWR(undefined, { error: new ApiError("down", 503) });
    render(<OperatorOperatorsPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Operatoren"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mockMutate).toHaveBeenCalled();
  });

  it("shows creator name and expiry for pending invitations", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getByText(/Eingeladen von Admin/)).toBeInTheDocument();
    expect(screen.getByText(/Läuft ab am/)).toBeInTheDocument();
  });

  it("shows last login date for operators", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(screen.getAllByText(/Letzter Login:/)).toHaveLength(2);
  });

  it("offers deactivation for colleagues but not for the own account", () => {
    setupSWR(sampleData);
    render(<OperatorOperatorsPage />);

    expect(
      screen.getByRole("button", { name: "Aktionen für Colleague User" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Aktionen für Admin User" }),
    ).not.toBeInTheDocument();
    openActions("Colleague User");
    expect(
      screen.getByRole("menuitem", { name: "Deaktivieren" }),
    ).toBeInTheDocument();
  });

  it("deactivates a colleague after the confirmation", async () => {
    setupSWR(sampleData);
    mockDeactivateOperator.mockResolvedValue(undefined);
    render(<OperatorOperatorsPage />);

    openActions("Colleague User");
    fireEvent.click(screen.getByRole("menuitem", { name: "Deaktivieren" }));
    const modal = screen.getByTestId("confirmation-modal");
    expect(modal.textContent).toContain("colleague@example.com");
    expect(modal.textContent).toContain("nicht mehr anmelden");
    expect(mockDeactivateOperator).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("Confirm"));

    await waitFor(() => {
      expect(mockDeactivateOperator).toHaveBeenCalledWith("4");
    });
    expect(
      await screen.findByText("Colleague User wurde deaktiviert."),
    ).toBeInTheDocument();
    expect(mockMutate).toHaveBeenCalled();
  });

  it("disables the deactivation confirmation while the request runs", async () => {
    setupSWR(sampleData);
    let complete!: () => void;
    mockDeactivateOperator.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          complete = resolve;
        }),
    );
    render(<OperatorOperatorsPage />);

    openActions("Colleague User");
    fireEvent.click(screen.getByRole("menuitem", { name: "Deaktivieren" }));
    fireEvent.click(screen.getByText("Confirm"));

    await waitFor(() =>
      expect(mockDeactivateOperator).toHaveBeenCalledWith("4"),
    );
    expect(screen.getByText("Confirm")).toBeDisabled();
    fireEvent.click(screen.getByText("Confirm"));
    expect(mockDeactivateOperator).toHaveBeenCalledOnce();

    complete();
  });

  it("keeps a failed deactivation inside the confirmation", async () => {
    setupSWR(sampleData);
    mockDeactivateOperator.mockRejectedValue(
      new ApiError("self", 409, {
        code: "identity.operator_self_deactivation",
      }),
    );
    render(<OperatorOperatorsPage />);

    openActions("Colleague User");
    fireEvent.click(screen.getByRole("menuitem", { name: "Deaktivieren" }));
    fireEvent.click(screen.getByText("Confirm"));

    const modal = screen.getByTestId("confirmation-modal");
    await waitFor(() =>
      expect(modal.textContent).toContain(
        catalogText(
          "identity.operator_self_deactivation",
          "das Deaktivieren des Kontos",
        ),
      ),
    );
  });

  it("shows when a deactivated account is deleted and reactivates it", async () => {
    setupSWR(sampleData);
    mockReactivateOperator.mockResolvedValue(undefined);
    render(<OperatorOperatorsPage />);

    expect(screen.getByText("Deaktiviert am 01.03.2026")).toBeInTheDocument();
    expect(screen.getByText("Wird ab 01.01.2028 gelöscht")).toBeInTheDocument();
    openActions("Inactive User");
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Wieder aktivieren" }),
    );

    await waitFor(() => {
      expect(mockReactivateOperator).toHaveBeenCalledWith("3");
    });
    expect(
      await screen.findByText("Inactive User kann sich wieder anmelden."),
    ).toBeInTheDocument();
    expect(mockMutate).toHaveBeenCalled();
  });
});
