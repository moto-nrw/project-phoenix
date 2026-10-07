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
} = vi.hoisted(() => ({
  mockUseSWR: vi.fn(),
  mockMutate: vi.fn(),
  mockListInvitations: vi.fn(),
  mockCreateInvitation: vi.fn(),
  mockResendInvitation: vi.fn(),
  mockRevokeInvitation: vi.fn(),
}));

vi.mock("swr", () => ({
  default: mockUseSWR,
}));

vi.mock("~/lib/operator/operator-invitation-api", () => ({
  listOperatorInvitations: mockListInvitations,
  createOperatorInvitation: mockCreateInvitation,
  resendOperatorInvitation: mockResendInvitation,
  revokeOperatorInvitation: mockRevokeInvitation,
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
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        {children}
        <button type="button" onClick={onConfirm}>
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
    },
    {
      id: "3",
      email: "inactive@example.com",
      displayName: "Inactive User",
      active: false,
      createdAt: "2026-01-01T00:00:00Z",
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

    expect(screen.getByText("Aktiv")).toBeInTheDocument();
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

    expect(screen.queryByText("Offene Einladungen")).not.toBeInTheDocument();
  });

  it("submits invite form and shows toast", async () => {
    setupSWR(sampleData);
    mockCreateInvitation.mockResolvedValue(undefined);

    render(<OperatorOperatorsPage />);

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse *"), {
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

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse *"), {
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

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse *"), {
      target: { value: "fail@example.com" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Einladung senden" }));

    expect(
      await screen.findByText(
        catalogText("general.business_rejection", "die Einladung"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("E-Mail-Adresse *")).toHaveAttribute(
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

    expect(screen.getByText(/Letzter Login:/)).toBeInTheDocument();
  });
});
