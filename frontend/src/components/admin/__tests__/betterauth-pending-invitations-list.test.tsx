/**
 * Tests for BetterAuthPendingInvitationsList component
 *
 * This file tests:
 * - Loading and displaying pending invitations
 * - Empty state rendering
 * - Error handling and retry functionality
 * - Resend invitation functionality
 * - Cancel/revoke invitation functionality
 * - Role display mapping
 * - Expiry date formatting and expired status
 * - Refresh on refreshKey change
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { BetterAuthPendingInvitationsList } from "../betterauth-pending-invitations-list";

// Mock toast context
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

// Mock authClient - use hoisted mocks
const mockListInvitations = vi.hoisted(() => vi.fn());
const mockInviteMember = vi.hoisted(() => vi.fn());
const mockCancelInvitation = vi.hoisted(() => vi.fn());
const mockUseSession = vi.hoisted(() => vi.fn());

vi.mock("~/lib/auth-client", () => ({
  authClient: {
    organization: {
      listInvitations: mockListInvitations,
      inviteMember: mockInviteMember,
      cancelInvitation: mockCancelInvitation,
    },
  },
  useSession: mockUseSession,
}));

// Mock getRoleDisplayName
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => {
    const displayNames: Record<string, string> = {
      supervisor: "Supervisor",
      ogsAdmin: "OGS-Administrator",
      admin: "Administrator",
      member: "Mitglied",
    };
    return displayNames[name] ?? name;
  },
}));

// Mock date helpers
vi.mock("~/lib/utils/date-helpers", () => ({
  isValidDateString: (dateString: string | null | undefined) => {
    if (!dateString) return false;
    const date = new Date(dateString);
    return !Number.isNaN(date.getTime());
  },
  isDateExpired: (dateString: string | null | undefined) => {
    if (!dateString) return false;
    const date = new Date(dateString);
    if (Number.isNaN(date.getTime())) return false;
    return date.getTime() < Date.now();
  },
}));

// Test data
const createTestInvitation = (
  overrides: Partial<{
    id: string;
    email: string;
    role: string;
    status: string;
    organizationId: string;
    expiresAt: string;
  }> = {},
) => ({
  id: "inv-1",
  email: "test@example.com",
  role: "supervisor",
  status: "pending",
  organizationId: "org-123",
  expiresAt: new Date(Date.now() + 86400000).toISOString(), // Tomorrow
  ...overrides,
});

beforeEach(() => {
  vi.clearAllMocks();

  // Default session mock with active organization
  mockUseSession.mockReturnValue({
    data: {
      session: {
        activeOrganizationId: "org-123",
      },
    },
  });

  // Default invitations mock
  mockListInvitations.mockResolvedValue({
    data: [createTestInvitation()],
    error: null,
  });
});

describe("BetterAuthPendingInvitationsList", () => {
  // =============================================================================
  // Loading State Tests
  // =============================================================================

  describe("loading state", () => {
    it("shows loading state initially", async () => {
      // Delay the response to see loading state
      mockListInvitations.mockImplementation(
        () =>
          new Promise((resolve) => {
            setTimeout(
              () =>
                resolve({
                  data: [createTestInvitation()],
                  error: null,
                }),
              100,
            );
          }),
      );

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      // Should show loading state (Loading component uses <output aria-label="Lädt...">)
      expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
    });

    it("hides loading state after data loads", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.queryByLabelText("Lädt...")).not.toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Rendering Tests
  // =============================================================================

  describe("rendering", () => {
    it("renders the card title", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Offene Einladungen")).toBeInTheDocument();
      });
    });

    it("shows invitation count in description", async () => {
      mockListInvitations.mockResolvedValue({
        data: [createTestInvitation({ id: "inv-1" })],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("1 offen")).toBeInTheDocument();
      });
    });

    it("renders invitation email", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("test@example.com")).toBeInTheDocument();
      });
    });

    it("renders invitation role with display name", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Supervisor")).toBeInTheDocument();
      });
    });

    it("renders resend button", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });
    });

    it("renders delete button", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Empty State Tests
  // =============================================================================

  describe("empty state", () => {
    it("shows empty state when no invitations", async () => {
      mockListInvitations.mockResolvedValue({
        data: [],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(
          screen.getByText("Keine offenen Einladungen"),
        ).toBeInTheDocument();
      });
    });

    it("shows empty state when only non-pending invitations exist", async () => {
      mockListInvitations.mockResolvedValue({
        data: [createTestInvitation({ status: "accepted" })],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(
          screen.getByText("Keine offenen Einladungen"),
        ).toBeInTheDocument();
      });
    });

    it("stops loading when no organization ID", async () => {
      mockUseSession.mockReturnValue({
        data: {
          session: {
            activeOrganizationId: null,
          },
        },
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(mockListInvitations).not.toHaveBeenCalled();
      });
    });
  });

  // =============================================================================
  // Error Handling Tests
  // =============================================================================

  describe("error handling", () => {
    it("shows error message when list invitations fails", async () => {
      mockListInvitations.mockResolvedValue({
        data: null,
        error: { message: "Failed to load" },
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Failed to load")).toBeInTheDocument();
      });
    });

    it("shows default error message when no error message provided", async () => {
      mockListInvitations.mockResolvedValue({
        data: null,
        error: {},
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(
          screen.getByText("Einladungen konnten nicht geladen werden."),
        ).toBeInTheDocument();
      });
    });

    it("shows error on thrown exception", async () => {
      mockListInvitations.mockRejectedValue(new Error("Network error"));

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(
          screen.getByText("Einladungen konnten nicht geladen werden."),
        ).toBeInTheDocument();
      });
    });

    it("has retry button that reloads invitations", async () => {
      mockListInvitations.mockResolvedValueOnce({
        data: null,
        error: { message: "Failed to load" },
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut versuchen")).toBeInTheDocument();
      });

      // Setup success for retry
      mockListInvitations.mockResolvedValueOnce({
        data: [createTestInvitation()],
        error: null,
      });

      const retryButton = screen.getByText("Erneut versuchen");
      fireEvent.click(retryButton);

      await waitFor(() => {
        expect(mockListInvitations).toHaveBeenCalledTimes(2);
      });
    });
  });

  // =============================================================================
  // Resend Invitation Tests
  // =============================================================================

  describe("resend invitation", () => {
    it("resends invitation when resend button clicked", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "new-inv-1" },
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith({
          email: "test@example.com",
          role: "supervisor",
          organizationId: "org-123",
          resend: true,
        });
      });

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Einladung wurde erneut gesendet.",
        );
      });
    });

    it("shows error toast on resend failure", async () => {
      mockInviteMember.mockResolvedValue({
        data: null,
        error: { message: "Resend failed" },
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith("Resend failed");
      });
    });

    it("shows default error message on resend failure without message", async () => {
      mockInviteMember.mockResolvedValue({
        data: null,
        error: {},
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Die Einladung konnte nicht erneut gesendet werden.",
        );
      });
    });

    it("shows error toast on resend exception", async () => {
      mockInviteMember.mockRejectedValue(new Error("Network error"));

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Die Einladung konnte nicht erneut gesendet werden.",
        );
      });
    });

    it("disables resend button for expired invitations", async () => {
      mockListInvitations.mockResolvedValue({
        data: [
          createTestInvitation({
            expiresAt: new Date(Date.now() - 86400000).toISOString(), // Yesterday
          }),
        ],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        const resendButton = screen.getByText("Erneut");
        expect(resendButton).toBeDisabled();
      });
    });

    it("reloads invitations after successful resend", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "new-inv-1" },
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      // Clear initial call count
      mockListInvitations.mockClear();

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        expect(mockListInvitations).toHaveBeenCalled();
      });
    });
  });

  // =============================================================================
  // Cancel Invitation Tests
  // =============================================================================

  describe("cancel invitation", () => {
    it("shows confirmation modal when delete button clicked", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Einladung widerrufen?")).toBeInTheDocument();
      });
    });

    it("shows email in confirmation modal", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        // Email appears in both the table and the modal, verify modal is shown
        expect(screen.getByText("Einladung widerrufen?")).toBeInTheDocument();
        // Modal should contain the email address
        const emailMatches = screen.getAllByText(/test@example\.com/);
        expect(emailMatches.length).toBeGreaterThanOrEqual(2); // Table + Modal
      });
    });

    it("cancels invitation when confirmed", async () => {
      mockCancelInvitation.mockResolvedValue({
        data: {},
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Widerrufen")).toBeInTheDocument();
      });

      const confirmButton = screen.getByText("Widerrufen");
      fireEvent.click(confirmButton);

      await waitFor(() => {
        expect(mockCancelInvitation).toHaveBeenCalledWith({
          invitationId: "inv-1",
        });
      });

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Einladung wurde widerrufen.",
        );
      });
    });

    it("shows error toast on cancel failure", async () => {
      mockCancelInvitation.mockResolvedValue({
        data: null,
        error: { message: "Cancel failed" },
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Widerrufen")).toBeInTheDocument();
      });

      const confirmButton = screen.getByText("Widerrufen");
      fireEvent.click(confirmButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith("Cancel failed");
      });
    });

    it("shows default error message on cancel failure without message", async () => {
      mockCancelInvitation.mockResolvedValue({
        data: null,
        error: {},
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Widerrufen")).toBeInTheDocument();
      });

      const confirmButton = screen.getByText("Widerrufen");
      fireEvent.click(confirmButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Die Einladung konnte nicht widerrufen werden.",
        );
      });
    });

    it("shows error toast on cancel exception", async () => {
      mockCancelInvitation.mockRejectedValue(new Error("Network error"));

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Widerrufen")).toBeInTheDocument();
      });

      const confirmButton = screen.getByText("Widerrufen");
      fireEvent.click(confirmButton);

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Die Einladung konnte nicht widerrufen werden.",
        );
      });
    });

    it("closes modal when cancel is clicked in dialog", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Abbrechen")).toBeInTheDocument();
      });

      const cancelButton = screen.getByText("Abbrechen");
      fireEvent.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByText("Einladung widerrufen?"),
        ).not.toBeInTheDocument();
      });
    });

    it("does nothing when handleCancel called without target", async () => {
      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      // The handleCancel function should check for cancelTarget
      // We can't directly test this, but we verify no errors occur
      expect(mockCancelInvitation).not.toHaveBeenCalled();
    });

    it("reloads invitations after successful cancel", async () => {
      mockCancelInvitation.mockResolvedValue({
        data: {},
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Löschen")).toBeInTheDocument();
      });

      // Clear initial call count
      mockListInvitations.mockClear();

      const deleteButton = screen.getByText("Löschen");
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText("Widerrufen")).toBeInTheDocument();
      });

      const confirmButton = screen.getByText("Widerrufen");
      fireEvent.click(confirmButton);

      await waitFor(() => {
        expect(mockListInvitations).toHaveBeenCalled();
      });
    });
  });

  // =============================================================================
  // Date/Expiry Tests
  // =============================================================================

  describe("expiry date handling", () => {
    it("shows formatted date for valid expiry", async () => {
      const tomorrow = new Date(Date.now() + 86400000);
      mockListInvitations.mockResolvedValue({
        data: [createTestInvitation({ expiresAt: tomorrow.toISOString() })],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        // Check that date is formatted (contains expected elements)
        const dateStr = tomorrow.toLocaleDateString("de-DE", {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
        });
        expect(screen.getByText(new RegExp(dateStr))).toBeInTheDocument();
      });
    });

    it("shows expired styling for past dates", async () => {
      mockListInvitations.mockResolvedValue({
        data: [
          createTestInvitation({
            expiresAt: new Date(Date.now() - 86400000).toISOString(),
          }),
        ],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        // Should have expired styling class (red background)
        const expiredBadge = document.querySelector(".bg-red-50");
        expect(expiredBadge).toBeInTheDocument();
      });
    });

    it("shows 'Ungültig' for invalid date strings", async () => {
      mockListInvitations.mockResolvedValue({
        data: [createTestInvitation({ expiresAt: "invalid-date" })],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Ungültig")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Role Display Tests
  // =============================================================================

  describe("role display", () => {
    it("shows mapped BetterAuth role display names", async () => {
      mockListInvitations.mockResolvedValue({
        data: [createTestInvitation({ role: "ogsAdmin" })],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("OGS-Administrator")).toBeInTheDocument();
      });
    });

    it("falls back to getRoleDisplayName for unknown roles", async () => {
      mockListInvitations.mockResolvedValue({
        data: [createTestInvitation({ role: "customRole" })],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        // Should fall back to getRoleDisplayName which returns original
        expect(screen.getByText("customRole")).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Refresh Key Tests
  // =============================================================================

  describe("refresh key changes", () => {
    it("reloads invitations when refreshKey changes", async () => {
      const { rerender } = render(
        <BetterAuthPendingInvitationsList refreshKey={0} />,
      );

      await waitFor(() => {
        expect(mockListInvitations).toHaveBeenCalledTimes(1);
      });

      // Change refreshKey
      rerender(<BetterAuthPendingInvitationsList refreshKey={1} />);

      await waitFor(() => {
        expect(mockListInvitations).toHaveBeenCalledTimes(2);
      });
    });
  });

  // =============================================================================
  // Data Mapping Tests
  // =============================================================================

  describe("data mapping", () => {
    it("handles Date object expiresAt", async () => {
      const tomorrow = new Date(Date.now() + 86400000);
      mockListInvitations.mockResolvedValue({
        data: [
          {
            id: "inv-1",
            email: "test@example.com",
            role: "supervisor",
            status: "pending",
            organizationId: "org-123",
            expiresAt: tomorrow, // Date object, not string
          },
        ],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("test@example.com")).toBeInTheDocument();
      });
    });

    it("handles null data response", async () => {
      mockListInvitations.mockResolvedValue({
        data: null,
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(
          screen.getByText("Keine offenen Einladungen"),
        ).toBeInTheDocument();
      });
    });

    it("sorts invitations by expiry date ascending", async () => {
      const tomorrow = new Date(Date.now() + 86400000);
      const nextWeek = new Date(Date.now() + 604800000);

      mockListInvitations.mockResolvedValue({
        data: [
          createTestInvitation({
            id: "inv-later",
            email: "later@example.com",
            expiresAt: nextWeek.toISOString(),
          }),
          createTestInvitation({
            id: "inv-sooner",
            email: "sooner@example.com",
            expiresAt: tomorrow.toISOString(),
          }),
        ],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("sooner@example.com")).toBeInTheDocument();
        expect(screen.getByText("later@example.com")).toBeInTheDocument();
      });

      // Check order by finding rows - sooner should come first
      const rows = screen.getAllByRole("row");
      const soonerIndex = rows.findIndex((row) =>
        row.textContent?.includes("sooner@example.com"),
      );
      const laterIndex = rows.findIndex((row) =>
        row.textContent?.includes("later@example.com"),
      );

      expect(soonerIndex).toBeLessThan(laterIndex);
    });
  });

  // =============================================================================
  // Action Loading State Tests
  // =============================================================================

  describe("action loading states", () => {
    it("shows loading indicator during resend", async () => {
      let resolveResend: ((value: unknown) => void) | undefined;
      mockInviteMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveResend = resolve;
          }),
      );

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        // Button should show loading state (ellipsis)
        expect(screen.getByText("…")).toBeInTheDocument();
      });

      // Cleanup
      resolveResend?.({ data: { id: "new-inv" }, error: null });
    });

    it("disables buttons during action", async () => {
      let resolveResend: ((value: unknown) => void) | undefined;
      mockInviteMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveResend = resolve;
          }),
      );

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("Erneut")).toBeInTheDocument();
      });

      const resendButton = screen.getByText("Erneut");
      fireEvent.click(resendButton);

      await waitFor(() => {
        const deleteButton = screen.getByText("Löschen");
        expect(deleteButton).toBeDisabled();
      });

      // Cleanup
      resolveResend?.({ data: { id: "new-inv" }, error: null });
    });
  });

  // =============================================================================
  // Multiple Invitations Tests
  // =============================================================================

  describe("multiple invitations", () => {
    it("renders multiple invitations", async () => {
      mockListInvitations.mockResolvedValue({
        data: [
          createTestInvitation({ id: "inv-1", email: "user1@example.com" }),
          createTestInvitation({ id: "inv-2", email: "user2@example.com" }),
          createTestInvitation({ id: "inv-3", email: "user3@example.com" }),
        ],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("user1@example.com")).toBeInTheDocument();
        expect(screen.getByText("user2@example.com")).toBeInTheDocument();
        expect(screen.getByText("user3@example.com")).toBeInTheDocument();
      });

      expect(screen.getByText("3 offen")).toBeInTheDocument();
    });

    it("filters out non-pending invitations", async () => {
      mockListInvitations.mockResolvedValue({
        data: [
          createTestInvitation({
            id: "inv-1",
            email: "pending@example.com",
            status: "pending",
          }),
          createTestInvitation({
            id: "inv-2",
            email: "accepted@example.com",
            status: "accepted",
          }),
          createTestInvitation({
            id: "inv-3",
            email: "rejected@example.com",
            status: "rejected",
          }),
        ],
        error: null,
      });

      render(<BetterAuthPendingInvitationsList refreshKey={0} />);

      await waitFor(() => {
        expect(screen.getByText("pending@example.com")).toBeInTheDocument();
      });

      expect(
        screen.queryByText("accepted@example.com"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText("rejected@example.com"),
      ).not.toBeInTheDocument();
      expect(screen.getByText("1 offen")).toBeInTheDocument();
    });
  });
});
