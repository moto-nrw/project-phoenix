/**
 * Tests for PendingInvitationsList Component
 * Tests the rendering and management of pending invitations
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PendingInvitationsList } from "./pending-invitations-list";
import type { PendingInvitation } from "~/lib/invitation-helpers";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// Mock dependencies. Load and dialog errors run through the real hooks; the
// resend error is spied at the action path (#2517).
const { mockShowActionError, mockToastError } = vi.hoisted(() => ({
  mockShowActionError: vi.fn(),
  mockToastError: vi.fn(),
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: mockToastError,
  })),
  useApiErrorDisplay: () => ({ show: mockShowActionError }),
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
        <button type="button" onClick={onConfirm} data-testid="confirm-button">
          Widerrufen
        </button>
        <button type="button" onClick={onClose} data-testid="cancel-button">
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
    roleId: "1",
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
    roleId: "1",
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
      expect(screen.getByText("2")).toBeInTheDocument();
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

  // Zeilenaktionen liegen im Kebab der Zeile (Bauart 1 Regel 4): Menü der
  // n-ten Zeile öffnen und den Eintrag zurückgeben. Escape schließt es.
  async function openRowMenu(index: number) {
    const triggers = await screen.findAllByRole("button", {
      name: /^Aktionen für/,
    });
    fireEvent.click(triggers[index]!);
    return triggers[index]!;
  }
  function closeRowMenu() {
    fireEvent.keyDown(document, { key: "Escape" });
  }

  it("renders resend only for active invitations", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(
        screen.getAllByRole("button", { name: /^Aktionen für/ }),
      ).toHaveLength(2);
    });
    // ID 2 is expired and sorted first, so only the second menu offers resend.
    await openRowMenu(0);
    expect(
      screen.queryByRole("menuitem", { name: "Erneut senden" }),
    ).not.toBeInTheDocument();
    closeRowMenu();

    await openRowMenu(1);
    expect(
      screen.getByRole("menuitem", { name: "Erneut senden" }),
    ).toBeInTheDocument();
  });

  it("renders delete buttons", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await waitFor(() => {
      expect(
        screen.getAllByRole("button", { name: /^Aktionen für/ }),
      ).toHaveLength(2);
    });
    for (const index of [0, 1]) {
      await openRowMenu(index);
      expect(
        screen.getByRole("menuitem", { name: "Löschen" }),
      ).toBeInTheDocument();
      closeRowMenu();
    }
  });

  it("calls resendInvitation when resend button clicked", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    // Invitations are sorted by expiration date (earliest first)
    // ID 2 (expired) comes first, so open the second row's menu for ID 1 (not expired)
    await openRowMenu(1);
    fireEvent.click(screen.getByRole("menuitem", { name: "Erneut senden" }));

    await waitFor(() => {
      expect(mockResendInvitation).toHaveBeenCalledWith(1);
    });
  });

  it("opens confirmation modal when delete button clicked", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    await openRowMenu(0);
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
      expect(screen.getByText("Einladung widerrufen?")).toBeInTheDocument();
    });
  });

  it("calls revokeInvitation when confirm button clicked", async () => {
    render(<PendingInvitationsList refreshKey={0} />);

    // Open the second row's menu (ID 1, non-expired)
    await openRowMenu(1);
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

    const confirmButton = await screen.findByTestId("confirm-button");
    fireEvent.click(confirmButton);

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

  // #2517: catalog text in the card instead of the raw message; retry
  // reloads; a failed load is no empty list.
  it("shows error state when loading fails", async () => {
    mockListPendingInvitations
      .mockRejectedValueOnce(
        new ApiError("Failed to load", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([]);

    render(<PendingInvitationsList refreshKey={0} />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der offenen Einladungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Failed to load/)).toBeNull();
    expect(screen.queryByText("Keine offenen Einladungen")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText("Keine offenen Einladungen"),
    ).toBeInTheDocument();
  });

  it("keeps a failed revoke in the confirmation dialog", async () => {
    mockRevokeInvitation.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );

    render(<PendingInvitationsList refreshKey={0} />);
    await openRowMenu(1);
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(await screen.findByTestId("confirm-button"));

    expect(
      await screen.findByText(
        catalogText("general.server", "das Widerrufen der Einladung"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();
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
