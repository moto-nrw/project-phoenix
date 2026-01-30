/**
 * Tests for PendingInvitationsList Component
 * Tests the rendering and management of pending invitations
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PendingInvitationsList } from "./pending-invitations-list";
import type { PendingInvitation } from "~/lib/invitation-helpers";

// Mock dependencies
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
  })),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onConfirm: () => void;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h3>{title}</h3>
        {children}
        <button onClick={onConfirm} data-testid="confirm-button">
          Widerrufen
        </button>
        <button onClick={onClose} data-testid="cancel-button">
          Abbrechen
        </button>
      </div>
    ) : null,
}));

const mockListPendingInvitations = vi.fn();
const mockResendInvitation = vi.fn();
const mockRevokeInvitation = vi.fn();

vi.mock("~/lib/invitation-api", () => ({
  listPendingInvitations: (): unknown => mockListPendingInvitations(),
  resendInvitation: (id: number): unknown => mockResendInvitation(id),
  revokeInvitation: (id: number): unknown => mockRevokeInvitation(id),
}));

vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (role: string) =>
    role === "teacher" ? "Lehrkraft" : role,
}));

vi.mock("~/lib/utils/date-helpers", () => ({
  isValidDateString: (date: string) => !isNaN(Date.parse(date)),
  isDateExpired: (date: string) => new Date(date) < new Date(),
}));

const mockInvitations: PendingInvitation[] = [
  {
    id: 1,
    email: "test1@example.com",
    roleId: 1,
    roleName: "teacher",
    createdBy: 1,
    creatorEmail: "admin@example.com",
    firstName: "John",
    lastName: "Doe",
    expiresAt: new Date(Date.now() + 86400000).toISOString(),
    token: "token1",
  },
  {
    id: 2,
    email: "test2@example.com",
    roleId: 1,
    roleName: "teacher",
    createdBy: 1,
    creatorEmail: "admin@example.com",
    firstName: "Jane",
    lastName: "Smith",
    expiresAt: new Date(Date.now() - 86400000).toISOString(),
    token: "token2",
  },
];

describe("PendingInvitationsList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListPendingInvitations.mockResolvedValue(mockInvitations);
    mockResendInvitation.mockResolvedValue(undefined);
    mockRevokeInvitation.mockResolvedValue(undefined);
  });

  it("shows loading state initially", () => {
    render(<PendingInvitationsList refreshKey={0} />);

    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();
  });

  it("renders invitation list after loading", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getByText("test1@example.com")).toBeInTheDocument();
      expect(screen.getByText("test2@example.com")).toBeInTheDocument();
    });
  });

  it("displays correct invitation count", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getByText("2 offen")).toBeInTheDocument();
    });
  });

  it("displays role names", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getAllByText("Lehrkraft")).toHaveLength(2);
    });
  });

  it("displays creator email", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getAllByText("admin@example.com")).toHaveLength(2);
    });
  });

  it("renders resend buttons", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getAllByText("Erneut")).toHaveLength(2);
    });
  });

  it("renders delete buttons", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getAllByText("Löschen")).toHaveLength(2);
    });
  });

  it("calls resendInvitation when resend button clicked", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      const resendButtons = screen.getAllByText("Erneut");
      // Invitations are sorted by expiration date (earliest first)
      // ID 2 (expired) comes first, so click the second button for ID 1 (not expired)
      fireEvent.click(resendButtons[1]!);
    });

    await waitFor(() => {
      expect(mockResendInvitation).toHaveBeenCalledWith(1);
    });
  });

  it("opens confirmation modal when delete button clicked", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      const deleteButtons = screen.getAllByText("Löschen");
      fireEvent.click(deleteButtons[0]!);
    });

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      expect(screen.getByText("Einladung widerrufen?")).toBeInTheDocument();
    });
  });

  it("calls revokeInvitation when confirm button clicked", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      const deleteButtons = screen.getAllByText("Löschen");
      // Click the second delete button (ID 1, non-expired)
      fireEvent.click(deleteButtons[1]!);
    });

    await waitFor(() => {
      const confirmButton = screen.getByTestId("confirm-button");
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(mockRevokeInvitation).toHaveBeenCalledWith(1);
    });
  });

  it("shows empty state when no invitations", async () => {
    mockListPendingInvitations.mockResolvedValue([]);

    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(screen.getByText("Keine offenen Einladungen")).toBeInTheDocument();
    });
  });

  it("shows error state when loading fails", async () => {
    mockListPendingInvitations.mockRejectedValue(new Error("Failed to load"));

    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      // Error object's message is shown directly
      expect(screen.getByText(/Failed to load/)).toBeInTheDocument();
    });
  });

  it("reloads when refreshKey changes", async () => {
    const { rerender } = render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(mockListPendingInvitations).toHaveBeenCalledTimes(1);
    });

    rerender(<PendingInvitationsList refreshKey={1} />);

    await waitFor(() => {
      expect(mockListPendingInvitations).toHaveBeenCalledTimes(2);
    });
  });

  it("disables resend for expired invitations", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      const resendButtons = screen.getAllByText("Erneut");
      // ID 2 (expired) is sorted first, so it's at index 0
      expect(resendButtons[0]).toBeDisabled();
    });
  });

  it("sorts invitations by expiration date", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      const emails = screen
        .getAllByRole("row")
        .slice(1)
        .map((row) => row.textContent);

      expect(emails[0]).toContain("test2@example.com");
      expect(emails[1]).toContain("test1@example.com");
    });
  });
});
